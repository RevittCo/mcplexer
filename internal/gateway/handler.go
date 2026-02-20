package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/revittco/mcplexer/internal/approval"
	"github.com/revittco/mcplexer/internal/audit"
	"github.com/revittco/mcplexer/internal/cache"
	"github.com/revittco/mcplexer/internal/routing"
	"github.com/revittco/mcplexer/internal/store"
)

// ToolLister abstracts downstream tool discovery and invocation.
type ToolLister interface {
	ListAllTools(ctx context.Context) (map[string]json.RawMessage, error)
	ListToolsForServers(ctx context.Context, serverIDs []string) (map[string]json.RawMessage, error)
	Call(ctx context.Context, serverID, authScopeID, toolName string, args json.RawMessage) (json.RawMessage, error)
}

// CachingCaller extends ToolLister with cache-aware calling.
type CachingCaller interface {
	ToolLister
	CallWithMeta(ctx context.Context, serverID, authScopeID, toolName string, args json.RawMessage, cacheBust bool) (cache.CallResult, error)
	ToolCache() *cache.ToolCache
}

// handler contains the logic for each MCP method.
type handler struct {
	store          store.Store
	engine         *routing.Engine
	manager        ToolLister
	sessions       *sessionManager
	auditor        *audit.Logger
	approvals      *approval.Manager // nil = approval system disabled
	toolsListCache *cache.Cache[string, json.RawMessage]
	notifier       Notifier // set at runtime for sending notifications
}

// setNotifier sets the notifier for sending client notifications.
func (h *handler) setNotifier(n Notifier) {
	h.notifier = n
}

func newHandler(
	s store.Store,
	e *routing.Engine,
	m ToolLister,
	a *audit.Logger,
	t TransportMode,
	approvals *approval.Manager,
) *handler {
	return &handler{
		store:          s,
		engine:         e,
		manager:        m,
		sessions:       newSessionManager(s, t),
		auditor:        a,
		approvals:      approvals,
		toolsListCache: cache.New[string, json.RawMessage](10, 15*time.Second),
	}
}

func (h *handler) handleInitialize(
	ctx context.Context, params json.RawMessage,
) (json.RawMessage, *RPCError) {
	var p InitializeParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &RPCError{Code: CodeInvalidParams, Message: err.Error()}
	}

	if err := h.sessions.create(ctx, p.ClientInfo, p.Roots); err != nil {
		slog.Error("create session", "error", err)
	}

	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: ServerCapability{
			Tools: &ToolCapability{ListChanged: true},
		},
		ServerInfo: ServerInfo{Name: "mcplexer", Version: "0.1.0"},
	}

	data, err := json.Marshal(result)
	if err != nil {
		return nil, &RPCError{Code: CodeInternalError, Message: err.Error()}
	}
	return data, nil
}

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
		// Session has explicitly loaded tools — use those.
		tools = append(tools, activeTools...)
	} else {
		// No tools loaded yet — fall back to capabilities cache.
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
	// Build a cache key from sorted server IDs.
	key := strings.Join(serverIDs, ",")

	// We cache the aggregate JSON map, keyed by the server ID list.
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
		if errors.Is(err, routing.ErrNoRoute) || errors.Is(err, routing.ErrDenied) {
			h.recordAuditBlocked(ctx, req.Name, req.Arguments, nil, nil, rpcErr, start)
		} else {
			h.recordAudit(ctx, req.Name, req.Arguments, nil, nil, rpcErr, start)
		}
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

// handleApprovalGate implements two-phase approval interception.
// Phase 1: no _justification → return error asking for it.
// Phase 2: _justification present → block until approved/denied/timeout.
// Returns (nil, nil) when approved (caller should proceed to dispatch).
func (h *handler) handleApprovalGate(
	ctx context.Context,
	req CallToolRequest,
	route *routing.RouteResult,
	originalTool string,
	start time.Time,
) (json.RawMessage, *RPCError) {
	// Parse arguments to check for _justification.
	var args map[string]json.RawMessage
	if len(req.Arguments) > 0 {
		_ = json.Unmarshal(req.Arguments, &args)
	}

	justRaw, hasJust := args["_justification"]
	var justification string
	if hasJust {
		_ = json.Unmarshal(justRaw, &justification)
	}
	justification = strings.TrimSpace(justification)

	// Phase 1: no justification provided.
	if justification == "" {
		result := marshalErrorResult(
			"This tool requires approval before execution. " +
				"Retry your call with an additional `_justification` field " +
				"explaining why you need to use this tool.",
		)
		h.recordAuditBlocked(ctx, req.Name, req.Arguments, route, result, nil, start)
		return result, nil
	}

	// Phase 2: justification present — strip it from args and block.
	delete(args, "_justification")
	cleanArgs, _ := json.Marshal(args)
	req.Arguments = cleanArgs

	timeout := route.ApprovalTimeout
	if timeout <= 0 {
		timeout = 300
	}

	rec := &store.ToolApproval{
		RequestSessionID:   h.sessions.sessionID(),
		RequestClientType:  h.sessions.clientType(),
		RequestModel:       h.sessions.modelHint(),
		WorkspaceID:        h.sessions.workspaceID(),
		WorkspaceName:      h.sessions.workspaceName(),
		ToolName:           req.Name,
		Arguments:          string(cleanArgs),
		Justification:      justification,
		RouteRuleID:        route.MatchedRuleID,
		DownstreamServerID: route.DownstreamServerID,
		AuthScopeID:        route.AuthScopeID,
		TimeoutSec:         timeout,
	}

	approved, err := h.approvals.RequestApproval(ctx, rec)
	if err != nil {
		rpcErr := &RPCError{
			Code:    CodeInternalError,
			Message: fmt.Sprintf("approval request failed: %v", err),
		}
		h.recordAudit(ctx, req.Name, req.Arguments, route, nil, rpcErr, start)
		return nil, rpcErr
	}

	if !approved {
		result := marshalErrorResult(
			fmt.Sprintf("Tool call denied. Reason: %s", rec.Resolution),
		)
		h.recordAuditBlocked(ctx, req.Name, req.Arguments, route, result, nil, start)
		return result, nil
	}

	// Approved — return nil to signal caller to proceed with dispatch.
	return nil, nil
}

// recordAudit creates and persists an audit record for a tool call.
func (h *handler) recordAudit(
	ctx context.Context,
	toolName string,
	params json.RawMessage,
	route *routing.RouteResult,
	result json.RawMessage,
	rpcErr *RPCError,
	start time.Time,
) {
	h.recordAuditWithCache(ctx, toolName, params, route, result, rpcErr, start, false)
}

// recordAuditWithCache creates and persists an audit record with cache hit info.
func (h *handler) recordAuditWithCache(
	ctx context.Context,
	toolName string,
	params json.RawMessage,
	route *routing.RouteResult,
	result json.RawMessage,
	rpcErr *RPCError,
	start time.Time,
	cacheHit bool,
) {
	if h.auditor == nil {
		return
	}

	// Use the workspace and subpath from the route match when available,
	// falling back to the primary workspace for built-in/unrouted calls.
	wsID := h.sessions.workspaceID()
	wsName := h.sessions.workspaceName()
	var subpath string
	if route != nil && route.MatchedWorkspaceID != "" {
		wsID = route.MatchedWorkspaceID
		wsName = route.MatchedWorkspaceName
		subpath = route.Subpath
	} else if ancestors := h.sessions.workspaceAncestors(); len(ancestors) > 0 {
		subpath = routing.ComputeSubpath(h.sessions.clientRoot(), ancestors[0].RootPath)
	}

	rec := &store.AuditRecord{
		ID:             uuid.NewString(),
		Timestamp:      start,
		SessionID:      h.sessions.sessionID(),
		ClientType:     h.sessions.clientType(),
		Model:          h.sessions.modelHint(),
		WorkspaceID:    wsID,
		WorkspaceName:  wsName,
		Subpath:        subpath,
		ToolName:       toolName,
		ParamsRedacted: params,
		Status:         "success",
		LatencyMs:      int(time.Since(start).Milliseconds()),
		ResponseSize:   len(result),
		CacheHit:       cacheHit,
	}

	if route != nil {
		rec.RouteRuleID = route.MatchedRuleID
		rec.DownstreamServerID = route.DownstreamServerID
		rec.AuthScopeID = route.AuthScopeID
	}

	if rpcErr != nil {
		rec.Status = "error"
		rec.ErrorCode = fmt.Sprintf("%d", rpcErr.Code)
		rec.ErrorMessage = rpcErr.Message
	} else if isToolError(result) {
		rec.Status = "error"
		rec.ErrorCode = "tool_error"
		rec.ErrorMessage = extractToolErrorText(result)
	}

	if err := h.auditor.Record(ctx, rec); err != nil {
		slog.Error("audit record failed", "error", err)
	}
}

// recordAuditBlocked creates an audit record with status "blocked" for route
// denials, approval gates, and other policy-level rejections.
func (h *handler) recordAuditBlocked(
	ctx context.Context,
	toolName string,
	params json.RawMessage,
	route *routing.RouteResult,
	result json.RawMessage,
	rpcErr *RPCError,
	start time.Time,
) {
	if h.auditor == nil {
		return
	}

	wsID := h.sessions.workspaceID()
	wsName := h.sessions.workspaceName()
	var subpath string
	if route != nil && route.MatchedWorkspaceID != "" {
		wsID = route.MatchedWorkspaceID
		wsName = route.MatchedWorkspaceName
		subpath = route.Subpath
	} else if ancestors := h.sessions.workspaceAncestors(); len(ancestors) > 0 {
		subpath = routing.ComputeSubpath(h.sessions.clientRoot(), ancestors[0].RootPath)
	}

	rec := &store.AuditRecord{
		ID:             uuid.NewString(),
		Timestamp:      start,
		SessionID:      h.sessions.sessionID(),
		ClientType:     h.sessions.clientType(),
		Model:          h.sessions.modelHint(),
		WorkspaceID:    wsID,
		WorkspaceName:  wsName,
		Subpath:        subpath,
		ToolName:       toolName,
		ParamsRedacted: params,
		Status:         "blocked",
		LatencyMs:      int(time.Since(start).Milliseconds()),
		ResponseSize:   len(result),
	}

	if route != nil {
		rec.RouteRuleID = route.MatchedRuleID
		rec.DownstreamServerID = route.DownstreamServerID
		rec.AuthScopeID = route.AuthScopeID
	}

	if rpcErr != nil {
		rec.ErrorCode = fmt.Sprintf("%d", rpcErr.Code)
		rec.ErrorMessage = rpcErr.Message
	} else if isToolError(result) {
		rec.ErrorCode = "blocked"
		rec.ErrorMessage = extractToolErrorText(result)
	}

	if err := h.auditor.Record(ctx, rec); err != nil {
		slog.Error("audit record failed", "error", err)
	}
}

func (h *handler) handleBuiltinCall(
	ctx context.Context, req CallToolRequest,
) (json.RawMessage, *RPCError) {
	switch req.Name {
	case "mcpx__search_tools":
		var args struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(req.Arguments, &args); err != nil {
			return nil, &RPCError{Code: CodeInvalidParams, Message: err.Error()}
		}
		return h.handleSearchTools(ctx, args.Query)

	case "mcpx__load_tools":
		var args struct {
			Tools []string `json:"tools"`
		}
		if err := json.Unmarshal(req.Arguments, &args); err != nil {
			return nil, &RPCError{Code: CodeInvalidParams, Message: err.Error()}
		}
		return h.handleLoadTools(ctx, args.Tools)

	case "mcpx__unload_tools":
		var args struct {
			Tools []string `json:"tools"`
		}
		if err := json.Unmarshal(req.Arguments, &args); err != nil {
			return nil, &RPCError{Code: CodeInvalidParams, Message: err.Error()}
		}
		return h.handleUnloadTools(args.Tools)

	case "mcpx__list_pending_approvals":
		return h.handleListPendingApprovals()

	case "mcpx__approve_tool_call":
		var args struct {
			ApprovalID string `json:"approval_id"`
			Reason     string `json:"reason"`
		}
		if err := json.Unmarshal(req.Arguments, &args); err != nil {
			return nil, &RPCError{Code: CodeInvalidParams, Message: err.Error()}
		}
		return h.handleResolveApproval(args.ApprovalID, args.Reason, true)

	case "mcpx__deny_tool_call":
		var args struct {
			ApprovalID string `json:"approval_id"`
			Reason     string `json:"reason"`
		}
		if err := json.Unmarshal(req.Arguments, &args); err != nil {
			return nil, &RPCError{Code: CodeInvalidParams, Message: err.Error()}
		}
		if args.Reason == "" {
			return nil, &RPCError{Code: CodeInvalidParams, Message: "reason is required for denial"}
		}
		return h.handleResolveApproval(args.ApprovalID, args.Reason, false)

	case "mcpx__flush_cache":
		var args struct {
			ServerID string `json:"server_id"`
		}
		if len(req.Arguments) > 0 {
			_ = json.Unmarshal(req.Arguments, &args)
		}
		return h.handleFlushCache(args.ServerID)

	default:
		return nil, &RPCError{
			Code:    CodeMethodNotFound,
			Message: fmt.Sprintf("unknown built-in: %s", req.Name),
		}
	}
}

func (h *handler) handleFlushCache(serverID string) (json.RawMessage, *RPCError) {
	cc, ok := h.manager.(CachingCaller)
	if !ok {
		return marshalErrorResult("Cache system is not enabled."), nil
	}
	tc := cc.ToolCache()
	if serverID != "" {
		tc.InvalidateServer(serverID)
		return marshalToolResult(fmt.Sprintf("Flushed cache for server %q.", serverID)), nil
	}
	tc.Flush()
	return marshalToolResult("Flushed all tool call cache entries."), nil
}

func (h *handler) handleListPendingApprovals() (json.RawMessage, *RPCError) {
	if h.approvals == nil {
		return marshalToolResult("Approval system is not enabled."), nil
	}

	pending := h.approvals.ListPending(h.sessions.sessionID())
	if len(pending) == 0 {
		return marshalToolResult("No pending approvals."), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d pending approval(s):\n", len(pending))
	for _, a := range pending {
		fmt.Fprintf(&b, "\n## %s\n", a.ID)
		fmt.Fprintf(&b, "Tool: %s\n", a.ToolName)
		fmt.Fprintf(&b, "Justification: %s\n", a.Justification)
		fmt.Fprintf(&b, "Requested by: %s (%s)\n", a.RequestClientType, a.RequestModel)
		fmt.Fprintf(&b, "Arguments: %s\n", a.Arguments)
		fmt.Fprintf(&b, "Created: %s\n", a.CreatedAt.Format(time.RFC3339))
	}
	return marshalToolResult(b.String()), nil
}

func (h *handler) handleResolveApproval(
	approvalID, reason string, approved bool,
) (json.RawMessage, *RPCError) {
	if h.approvals == nil {
		return marshalErrorResult("Approval system is not enabled."), nil
	}
	if approvalID == "" {
		return nil, &RPCError{Code: CodeInvalidParams, Message: "approval_id is required"}
	}

	err := h.approvals.Resolve(
		approvalID,
		h.sessions.sessionID(),
		"mcp_agent",
		reason,
		approved,
	)
	if err != nil {
		if errors.Is(err, approval.ErrSelfApproval) {
			return marshalErrorResult("You cannot approve your own tool call request."), nil
		}
		if errors.Is(err, approval.ErrAlreadyResolved) {
			return marshalErrorResult("This approval has already been resolved."), nil
		}
		return nil, &RPCError{Code: CodeInternalError, Message: err.Error()}
	}

	action := "denied"
	if approved {
		action = "approved"
	}
	return marshalToolResult(fmt.Sprintf("Tool call %s successfully %s.", approvalID, action)), nil
}

// extractOriginalToolName strips the namespace prefix.
func extractOriginalToolName(namespacedTool string) string {
	if _, after, ok := strings.Cut(namespacedTool, "__"); ok {
		return after
	}
	return namespacedTool
}

// isToolError checks whether a tools/call result has isError set.
func isToolError(result json.RawMessage) bool {
	if len(result) == 0 {
		return false
	}
	var peek struct {
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(result, &peek); err != nil {
		return false
	}
	return peek.IsError
}

// extractToolErrorText pulls the first text content from an isError result.
func extractToolErrorText(result json.RawMessage) string {
	var r CallToolResult
	if err := json.Unmarshal(result, &r); err != nil {
		return "tool returned error"
	}
	for _, c := range r.Content {
		if c.Text != "" {
			if len(c.Text) > 200 {
				return c.Text[:200]
			}
			return c.Text
		}
	}
	return "tool returned error"
}

func mapRouteError(err error) *RPCError {
	switch {
	case errors.Is(err, routing.ErrNoRoute):
		return &RPCError{Code: CodeRouteNotFound, Message: "no matching route"}
	case errors.Is(err, routing.ErrDenied):
		return &RPCError{Code: CodeRouteNotFound, Message: "route denied by policy"}
	default:
		return &RPCError{Code: CodeInternalError, Message: err.Error()}
	}
}

// extractAndRemoveCacheBust checks for a _cache_bust boolean in the
// tool call arguments and removes it before forwarding downstream.
func extractAndRemoveCacheBust(args *json.RawMessage) bool {
	if args == nil || len(*args) == 0 {
		return false
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(*args, &m); err != nil {
		return false
	}
	raw, ok := m["_cache_bust"]
	if !ok {
		return false
	}
	var bust bool
	if err := json.Unmarshal(raw, &bust); err != nil || !bust {
		return false
	}
	delete(m, "_cache_bust")
	cleaned, err := json.Marshal(m)
	if err != nil {
		return false
	}
	*args = cleaned
	return true
}

// injectCacheMeta adds a _cache field to the MCP tool result _meta object
// so the AI can see whether the response was served from cache and how old it is.
func injectCacheMeta(result json.RawMessage, cacheHit bool, cacheAge time.Duration) json.RawMessage {
	if len(result) == 0 {
		return result
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(result, &envelope); err != nil {
		return result
	}

	cacheMeta := map[string]any{
		"cached": cacheHit,
	}
	if cacheHit {
		cacheMeta["age_seconds"] = int(cacheAge.Seconds())
	}

	// Merge into existing _meta or create it.
	meta := make(map[string]json.RawMessage)
	if raw, ok := envelope["_meta"]; ok {
		_ = json.Unmarshal(raw, &meta)
	}
	cacheJSON, _ := json.Marshal(cacheMeta)
	meta["cache"] = cacheJSON

	metaJSON, _ := json.Marshal(meta)
	envelope["_meta"] = metaJSON

	out, err := json.Marshal(envelope)
	if err != nil {
		return result
	}
	return out
}
