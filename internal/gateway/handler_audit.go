package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/revittco/mcplexer/internal/routing"
	"github.com/revittco/mcplexer/internal/store"
)

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
