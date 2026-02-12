package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/revitteth/mcplexer/internal/audit"
	"github.com/revitteth/mcplexer/internal/routing"
	"github.com/revitteth/mcplexer/internal/store"
)

// ToolLister abstracts downstream tool discovery and invocation.
type ToolLister interface {
	ListAllTools(ctx context.Context) (map[string]json.RawMessage, error)
	ListToolsForServers(ctx context.Context, serverIDs []string) (map[string]json.RawMessage, error)
	Call(ctx context.Context, serverID, authScopeID, toolName string, args json.RawMessage) (json.RawMessage, error)
}

// handler contains the logic for each MCP method.
type handler struct {
	store    store.Store
	engine   *routing.Engine
	manager  ToolLister
	sessions *sessionManager
	auditor  *audit.Logger
}

func newHandler(
	s store.Store,
	e *routing.Engine,
	m ToolLister,
	a *audit.Logger,
	t TransportMode,
) *handler {
	return &handler{
		store:    s,
		engine:   e,
		manager:  m,
		sessions: newSessionManager(s, t),
		auditor:  a,
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
	liveTools, err := h.manager.ListToolsForServers(ctx, staticIDs)
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

	// For dynamic servers, serve tools from the capabilities cache so all
	// clients see them in tools/list without needing to call search_tools.
	// The cache is populated by: discover API, search_tools, or previous listing.
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

	// Include the built-in search tool when dynamic servers exist so smart
	// models can still force a live re-discovery.
	if len(dynamicServers) > 0 {
		tools = append(tools, searchToolDefinition())
	}

	// Only advertise tools the current session can actually route to.
	tools = h.filterByWorkspaceRoutes(ctx, tools)

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

func (h *handler) handleToolsCall(
	ctx context.Context, params json.RawMessage,
) (json.RawMessage, *RPCError) {
	start := time.Now()

	var req CallToolRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &RPCError{Code: CodeInvalidParams, Message: err.Error()}
	}

	// Handle built-in mcplexer tools before routing.
	if strings.HasPrefix(req.Name, "mcplexer__") {
		result, rpcErr := h.handleBuiltinCall(ctx, req)
		h.recordAudit(ctx, req.Name, req.Arguments, nil, result, rpcErr, start)
		return result, rpcErr
	}

	// Extract namespace from tool name (namespace__toolname).
	originalTool := extractOriginalToolName(req.Name)

	// Route the call, falling back through ancestor workspaces.
	routeResult, err := h.engine.RouteWithFallback(ctx, routing.RouteContext{
		ToolName: req.Name,
	}, h.sessions.clientRoot(), h.sessions.workspaceAncestors())
	if err != nil {
		rpcErr := mapRouteError(err)
		h.recordAudit(ctx, req.Name, req.Arguments, nil, nil, rpcErr, start)
		return nil, rpcErr
	}

	// Dispatch to downstream.
	result, err := h.manager.Call(
		ctx,
		routeResult.DownstreamServerID,
		routeResult.AuthScopeID,
		originalTool,
		req.Arguments,
	)
	if err != nil {
		rpcErr := &RPCError{
			Code:    CodeProcessError,
			Message: fmt.Sprintf("downstream call: %v", err),
		}
		h.recordAudit(ctx, req.Name, req.Arguments, routeResult, nil, rpcErr, start)
		return nil, rpcErr
	}

	h.recordAudit(ctx, req.Name, req.Arguments, routeResult, result, nil, start)
	return result, nil
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
	if h.auditor == nil {
		return
	}

	// Compute subpath relative to the primary workspace root for audit.
	var subpath string
	if ancestors := h.sessions.workspaceAncestors(); len(ancestors) > 0 {
		subpath = routing.ComputeSubpath(h.sessions.clientRoot(), ancestors[0].RootPath)
	}

	rec := &store.AuditRecord{
		ID:             uuid.NewString(),
		Timestamp:      start,
		SessionID:      h.sessions.sessionID(),
		ClientType:     h.sessions.clientType(),
		Model:          h.sessions.modelHint(),
		WorkspaceID:    h.sessions.workspaceID(),
		Subpath:        subpath,
		ToolName:       toolName,
		ParamsRedacted: params,
		Status:         "success",
		LatencyMs:      int(time.Since(start).Milliseconds()),
		ResponseSize:   len(result),
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

func (h *handler) handleBuiltinCall(
	ctx context.Context, req CallToolRequest,
) (json.RawMessage, *RPCError) {
	switch req.Name {
	case "mcplexer__search_tools":
		var args struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(req.Arguments, &args); err != nil {
			return nil, &RPCError{Code: CodeInvalidParams, Message: err.Error()}
		}
		return h.handleSearchTools(ctx, args.Query)
	default:
		return nil, &RPCError{
			Code:    CodeMethodNotFound,
			Message: fmt.Sprintf("unknown built-in: %s", req.Name),
		}
	}
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
	switch err {
	case routing.ErrNoRoute:
		return &RPCError{Code: CodeRouteNotFound, Message: "no matching route"}
	case routing.ErrDenied:
		return &RPCError{Code: CodeRouteNotFound, Message: "route denied by policy"}
	default:
		return &RPCError{Code: CodeInternalError, Message: err.Error()}
	}
}
