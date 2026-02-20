package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/revittco/mcplexer/internal/cache"
	"github.com/revittco/mcplexer/internal/routing"
	"github.com/revittco/mcplexer/internal/store"
)

func (h *handler) handleToolsList(
	ctx context.Context,
) (json.RawMessage, *RPCError) {
	servers, err := h.store.ListDownstreamServers(ctx)
	if err != nil {
		return nil, &RPCError{
			Code:    CodeInternalError,
			Message: fmt.Sprintf("list servers: %v", err),
		}
	}

	// Split servers by discovery mode.
	var staticIDs []string
	var dynamicServers []store.DownstreamServer
	namespaces := make(map[string]string, len(servers))
	for _, srv := range servers {
		namespaces[srv.ID] = srv.ToolNamespace
		if srv.Discovery == "dynamic" {
			dynamicServers = append(dynamicServers, srv)
		} else {
			staticIDs = append(staticIDs, srv.ID)
		}
	}

	// Query static servers live for the advertised tool list.
	// Use the tools/list cache to avoid hammering downstream servers.
	liveTools, err := h.cachedListToolsForServers(ctx, staticIDs)
	if err != nil {
		return nil, &RPCError{
			Code:    CodeInternalError,
			Message: fmt.Sprintf("list tools: %v", err),
		}
	}

	tools := make([]Tool, 0)
	for serverID, rawResult := range liveTools {
		ns := namespaces[serverID]
		t, err := extractNamespacedTools(ns, rawResult)
		if err != nil {
			slog.Warn("failed to extract tools",
				"server", serverID, "error", err)
			continue
		}
		tools = append(tools, t...)

		if err := h.store.UpdateCapabilitiesCache(ctx, serverID, rawResult); err != nil {
			slog.Warn("failed to update capabilities cache",
				"server", serverID, "error", err)
		}
	}

	// For dynamic servers, only include tools that have been explicitly loaded
	// into the session via load_tools (or from capabilities cache if no session
	// tools exist yet, for backward compat during transition).
	activeTools := h.sessions.getActiveTools()
	if len(activeTools) > 0 {
		tools = append(tools, activeTools...)
	} else {
		for _, srv := range dynamicServers {
			if len(srv.CapabilitiesCache) == 0 || string(srv.CapabilitiesCache) == "{}" {
				continue
			}
			t, err := extractNamespacedTools(srv.ToolNamespace, srv.CapabilitiesCache)
			if err != nil {
				slog.Warn("failed to extract cached tools",
					"server", srv.ID, "error", err)
				continue
			}
			tools = append(tools, t...)
		}
	}

	// Include built-in tools based on server configuration.
	if len(dynamicServers) > 0 {
		tools = append(tools, searchToolDefinition())
		tools = append(tools, loadToolDefinition())
		tools = append(tools, unloadToolDefinition())
	}

	if h.approvals != nil {
		tools = append(tools, approvalToolDefinitions()...)
	}

	if _, ok := h.manager.(CachingCaller); ok {
		tools = append(tools, flushCacheToolDefinition())
	}

	// Only advertise tools the current session can actually route to.
	tools = h.filterByWorkspaceRoutes(ctx, tools)

	// Minify schemas to reduce context window consumption.
	if slimToolsEnabled() {
		tools = minifyToolSchemas(tools)
	}

	result := map[string]any{"tools": tools}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, &RPCError{Code: CodeInternalError, Message: err.Error()}
	}
	return data, nil
}

// extractNamespacedTools parses a tools/list result and prefixes tool names.
func extractNamespacedTools(namespace string, toolsResult json.RawMessage) ([]Tool, error) {
	if len(toolsResult) == 0 || string(toolsResult) == "{}" {
		return nil, nil
	}

	var result struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(toolsResult, &result); err != nil {
		return nil, err
	}

	out := make([]Tool, 0, len(result.Tools))
	for _, t := range result.Tools {
		t.Name = namespace + "__" + t.Name
		out = append(out, t)
	}
	return out, nil
}

// cachedListToolsForServers uses the tools/list cache to avoid hammering
// downstream servers on rapid tools/list calls. The cache has a 15s TTL.
func (h *handler) cachedListToolsForServers(ctx context.Context, serverIDs []string) (map[string]json.RawMessage, error) {
	key := strings.Join(serverIDs, ",")

	cached, err := h.toolsListCache.GetOrLoad(key, func() (json.RawMessage, error) {
		result, err := h.manager.ListToolsForServers(ctx, serverIDs)
		if err != nil {
			return nil, err
		}
		data, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}
		return data, nil
	})
	if err != nil {
		return nil, err
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(cached, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ToolsListStats returns cache statistics for the tools/list cache.
func (h *handler) ToolsListStats() cache.Stats {
	return h.toolsListCache.Stats()
}

func (h *handler) handleToolsCall(
	ctx context.Context, params json.RawMessage,
) (json.RawMessage, *RPCError) {
	start := time.Now()

	var req CallToolRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &RPCError{Code: CodeInvalidParams, Message: err.Error()}
	}

	// Normalize legacy mcplexer__ prefix to mcpx__ for backward compat.
	req.Name = normalizeBuiltinName(req.Name)

	// Extract namespace from tool name (namespace__toolname).
	originalTool := extractOriginalToolName(req.Name)

	// Route ALL tools through the engine (including built-ins).
	routeResult, err := h.engine.RouteWithFallback(ctx, routing.RouteContext{
		ToolName: req.Name,
	}, h.sessions.clientRoot(), h.sessions.workspaceAncestors())
	if err != nil {
		rpcErr := mapRouteError(err)
		h.recordAuditBlocked(ctx, req.Name, req.Arguments, nil, nil, rpcErr, start)
		return nil, rpcErr
	}

	// Dispatch based on whether it's a built-in or downstream tool.
	if routeResult.DownstreamServerID == "mcpx-builtin" {
		result, rpcErr := h.handleBuiltinCall(ctx, req)
		h.recordAudit(ctx, req.Name, req.Arguments, routeResult, result, rpcErr, start)
		return result, rpcErr
	}

	// Optional GitHub scope enforcement from route allowlists.
	if strings.HasPrefix(req.Name, "github__") {
		policy, err := newGitHubScopePolicy(routeResult.AllowedOrgs, routeResult.AllowedRepos)
		if err != nil {
			rpcErr := &RPCError{
				Code:    CodeInvalidParams,
				Message: fmt.Sprintf("invalid route allowlist configuration: %v", err),
			}
			h.recordAudit(ctx, req.Name, req.Arguments, routeResult, nil, rpcErr, start)
			return nil, rpcErr
		}
		if err := policy.Enforce(req.Arguments); err != nil {
			rpcErr := &RPCError{
				Code:    CodeInvalidParams,
				Message: err.Error(),
			}
			h.recordAudit(ctx, req.Name, req.Arguments, routeResult, nil, rpcErr, start)
			return nil, rpcErr
		}
	}

	// Two-phase approval interception.
	if routeResult.RequiresApproval && h.approvals != nil {
		result, rpcErr := h.handleApprovalGate(ctx, req, routeResult, originalTool, start)
		if result != nil || rpcErr != nil {
			return result, rpcErr
		}
		// Approval granted — fall through to dispatch.
	}

	// Extract _cache_bust from arguments if present.
	cacheBust := extractAndRemoveCacheBust(&req.Arguments)

	// Dispatch to downstream, with cache hit detection.
	var result json.RawMessage
	var cacheHit bool
	var cacheAge time.Duration

	if cc, ok := h.manager.(CachingCaller); ok {
		cr, callErr := cc.CallWithMeta(
			ctx,
			routeResult.DownstreamServerID,
			routeResult.AuthScopeID,
			originalTool,
			req.Arguments,
			cacheBust,
		)
		if callErr != nil {
			rpcErr := &RPCError{
				Code:    CodeProcessError,
				Message: fmt.Sprintf("downstream call: %v", callErr),
			}
			h.recordAudit(ctx, req.Name, req.Arguments, routeResult, nil, rpcErr, start)
			return nil, rpcErr
		}
		result = cr.Data
		cacheHit = cr.CacheHit
		cacheAge = cr.CacheAge
	} else {
		var callErr error
		result, callErr = h.manager.Call(
			ctx,
			routeResult.DownstreamServerID,
			routeResult.AuthScopeID,
			originalTool,
			req.Arguments,
		)
		if callErr != nil {
			rpcErr := &RPCError{
				Code:    CodeProcessError,
				Message: fmt.Sprintf("downstream call: %v", callErr),
			}
			h.recordAudit(ctx, req.Name, req.Arguments, routeResult, nil, rpcErr, start)
			return nil, rpcErr
		}
	}

	// Inject cache metadata into the tool result.
	result = injectCacheMeta(result, cacheHit, cacheAge)

	h.recordAuditWithCache(ctx, req.Name, req.Arguments, routeResult, result, nil, start, cacheHit)
	return result, nil
}

// extractOriginalToolName strips the namespace prefix.
func extractOriginalToolName(namespacedTool string) string {
	if _, after, ok := strings.Cut(namespacedTool, "__"); ok {
		return after
	}
	return namespacedTool
}

