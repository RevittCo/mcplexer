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

	codeModeOn := h.codeModeEnabled(ctx)

	tools := make([]Tool, 0)

	// In code mode, downstream tools are suppressed from tools/list — they're
	// accessible through execute_code instead. We still gather them to build
	// the embedded TypeScript API.
	if !codeModeOn {
		// Query static servers live for the advertised tool list.
		// Use the tools/list cache to avoid hammering downstream servers.
		liveTools, err := h.cachedListToolsForServers(ctx, staticIDs)
		if err != nil {
			return nil, &RPCError{
				Code:    CodeInternalError,
				Message: fmt.Sprintf("list tools: %v", err),
			}
		}

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

		// Codex compatibility mode: include dynamic-discovery tools directly in
		// tools/list to avoid relying on load_tools/list_changed orchestration.
		if h.includeDynamicToolsForCodex(ctx) && len(dynamicServers) > 0 {
			dynamicIDs := make([]string, 0, len(dynamicServers))
			for _, srv := range dynamicServers {
				dynamicIDs = append(dynamicIDs, srv.ID)
			}

			dynamicLiveTools, err := h.cachedListToolsForServers(ctx, dynamicIDs)
			if err != nil {
				return nil, &RPCError{
					Code:    CodeInternalError,
					Message: fmt.Sprintf("list dynamic tools: %v", err),
				}
			}

			for _, srv := range dynamicServers {
				rawResult, ok := dynamicLiveTools[srv.ID]
				if !ok {
					if len(srv.CapabilitiesCache) > 0 && string(srv.CapabilitiesCache) != "{}" {
						rawResult = srv.CapabilitiesCache
					} else {
						continue
					}
				} else if err := h.store.UpdateCapabilitiesCache(ctx, srv.ID, rawResult); err != nil {
					slog.Warn("failed to update capabilities cache",
						"server", srv.ID, "error", err)
				}

				ns := namespaces[srv.ID]
				t, err := extractNamespacedTools(ns, rawResult)
				if err != nil {
					slog.Warn("failed to extract tools",
						"server", srv.ID, "error", err)
					continue
				}
				tools = append(tools, t...)
			}
		}

		// Inject addon tools (from YAML definitions that bridge MCP server gaps).
		if h.addonRegistry != nil {
			tools = append(tools, addonToolDefinitions(h.addonRegistry)...)
		}

		// For dynamic servers, only include tools explicitly loaded via load_tools.
		activeTools := h.sessions.getActiveTools()
		tools = append(tools, activeTools...)
	}

	// Include built-in tools based on server configuration.
	if !codeModeOn && len(dynamicServers) > 0 {
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

	// Code mode: replace individual tools with execute_code (with embedded API).
	// Only include get_code_api when the API is too large to embed.
	if codeModeOn {
		execTool, apiEmbedded := h.buildCodeExecuteTool(ctx)
		tools = append(tools, execTool)
		if !apiEmbedded {
			tools = append(tools, codeAPIToolDefinition())
			// search_tools helps discover namespaces when API isn't embedded.
			tools = append(tools, searchToolDefinition())
		}
	}

	tools = dedupeToolsByName(tools)

	// Only advertise tools the current session can actually route to.
	tools = h.filterByWorkspaceRoutes(ctx, tools)

	// Apply description overrides from settings.
	tools = h.applyDescriptionOverrides(ctx, tools)

	// Minify schemas to reduce context window consumption.
	if h.slimToolsEnabled(ctx) {
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
	}, h.sessions.clientRoot(), h.sessions.workspaceAncestors(ctx))
	if err != nil {
		rpcErr := mapRouteError(err)
		h.recordAuditBlocked(ctx, req.Name, req.Arguments, nil, nil, rpcErr, start)
		return nil, rpcErr
	}

	// Coerce stringified JSON arguments (LLMs often pass objects as strings).
	req.Arguments = coerceStringifiedArgs(req.Arguments)

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

	// Look up server name for clearer error messages.
	serverName := routeResult.DownstreamServerID
	if srv, err := h.store.GetDownstreamServer(ctx, routeResult.DownstreamServerID); err == nil {
		serverName = srv.Name
	}

	// Two-phase approval interception.
	if routeResult.ApprovalMode != "" && routeResult.ApprovalMode != "none" && h.approvals != nil {
		needsApproval := true
		if routeResult.ApprovalMode == "write" {
			needsApproval = !h.isReadOnlyTool(ctx, routeResult.DownstreamServerID, originalTool)
		}
		if needsApproval {
			result, rpcErr := h.handleApprovalGate(ctx, req, routeResult, originalTool, start)
			if result != nil || rpcErr != nil {
				return result, rpcErr
			}
			// Approval granted — fall through to dispatch.
		}
	}

	// Intercept addon tool calls — execute as direct REST API calls
	// instead of forwarding to the downstream MCP server.
	if h.addonRegistry != nil && h.addonExecutor != nil {
		if addonTool := h.addonRegistry.GetTool(req.Name); addonTool != nil {
			// Use the addon's own auth scope if configured, otherwise fall back to route's.
			addonAuthScope := routeResult.AuthScopeID
			if addonTool.AuthScopeID != "" {
				addonAuthScope = addonTool.AuthScopeID
			}
			result, callErr := h.addonExecutor.Execute(
				ctx, addonTool, addonAuthScope, req.Arguments,
			)
			if callErr != nil {
				rpcErr := &RPCError{
					Code:    CodeProcessError,
					Message: fmt.Sprintf("addon %s: %v", req.Name, callErr),
				}
				h.recordAudit(ctx, req.Name, req.Arguments, routeResult, nil, rpcErr, start)
				return nil, rpcErr
			}
			h.recordAudit(ctx, req.Name, req.Arguments, routeResult, result, nil, start)
			return result, nil
		}
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
				Message: formatDownstreamError(serverName, callErr),
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
				Message: formatDownstreamError(serverName, callErr),
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

// slimToolsEnabled checks settings (then env var fallback) to decide
// whether to minify tool schemas.
func (h *handler) slimToolsEnabled(ctx context.Context) bool {
	if h.settingsSvc != nil {
		return h.settingsSvc.Load(ctx).SlimTools
	}
	return slimToolsEnabled()
}

func (h *handler) includeDynamicToolsForCodex(ctx context.Context) bool {
	if h.settingsSvc == nil {
		return false
	}
	settings := h.settingsSvc.Load(ctx)
	if !settings.CodexDynamicToolCompat {
		return false
	}
	return strings.HasPrefix(strings.ToLower(h.sessions.clientType()), "codex")
}

func dedupeToolsByName(tools []Tool) []Tool {
	seen := make(map[string]struct{}, len(tools))
	out := make([]Tool, 0, len(tools))
	for _, t := range tools {
		if _, ok := seen[t.Name]; ok {
			continue
		}
		seen[t.Name] = struct{}{}
		out = append(out, t)
	}
	return out
}

// applyDescriptionOverrides replaces builtin tool descriptions with
// user-configured overrides from settings.
func (h *handler) applyDescriptionOverrides(ctx context.Context, tools []Tool) []Tool {
	if h.settingsSvc == nil {
		return tools
	}
	overrides := h.settingsSvc.Load(ctx).ToolDescriptionOverrides
	if len(overrides) == 0 {
		return tools
	}
	for i, t := range tools {
		if desc, ok := overrides[t.Name]; ok && desc != "" {
			tools[i].Description = desc
		}
	}
	return tools
}

// extractOriginalToolName strips the namespace prefix.
func extractOriginalToolName(namespacedTool string) string {
	if _, after, ok := strings.Cut(namespacedTool, "__"); ok {
		return after
	}
	return namespacedTool
}

// formatDownstreamError produces a human-readable error message for downstream
// failures, including the server name and actionable hints where possible.
func formatDownstreamError(serverName string, err error) string {
	msg := err.Error()

	// Extract the root cause from wrapped error chains.
	root := msg
	for _, prefix := range []string{
		"get or start instance: ",
		"start instance: ",
		"start process: ",
		"initialize: ",
	} {
		if idx := strings.LastIndex(root, prefix); idx >= 0 {
			root = root[idx+len(prefix):]
		}
	}

	// Provide actionable hints for common failures.
	switch {
	case strings.Contains(root, "exec:"):
		// e.g. exec: "npx": executable file not found in $PATH
		return fmt.Sprintf("%s server failed to start: %s — ensure the required command is installed and in PATH", serverName, root)
	case strings.Contains(root, "no initialize response"):
		return fmt.Sprintf("%s server started but did not respond (process may have crashed). Check that any required services (e.g. database, Docker) are running.", serverName)
	case strings.Contains(root, "timed out"):
		return fmt.Sprintf("%s server did not respond within the timeout period. The server may be slow to start or unable to connect to its backend.", serverName)
	case strings.Contains(root, "connection refused"):
		return fmt.Sprintf("%s server could not connect to its backend service. Ensure the service is running and accessible.", serverName)
	default:
		return fmt.Sprintf("%s server error: %s", serverName, root)
	}
}

// isReadOnlyTool checks the tool's annotations for readOnlyHint.
// Returns true if the tool is explicitly marked as read-only.
func (h *handler) isReadOnlyTool(ctx context.Context, serverID, toolName string) bool {
	srv, err := h.store.GetDownstreamServer(ctx, serverID)
	if err != nil || len(srv.CapabilitiesCache) == 0 {
		return false
	}

	var result struct {
		Tools []struct {
			Name        string                     `json:"name"`
			Annotations map[string]json.RawMessage `json:"annotations"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(srv.CapabilitiesCache, &result); err != nil {
		return false
	}

	for _, t := range result.Tools {
		if t.Name != toolName {
			continue
		}
		if t.Annotations == nil {
			return false
		}
		raw, ok := t.Annotations["readOnlyHint"]
		if !ok {
			return false
		}
		var readOnly bool
		if err := json.Unmarshal(raw, &readOnly); err != nil {
			return false
		}
		return readOnly
	}
	return false
}
