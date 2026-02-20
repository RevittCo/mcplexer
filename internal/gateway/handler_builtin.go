package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/revittco/mcplexer/internal/approval"
	"github.com/revittco/mcplexer/internal/routing"
	"github.com/revittco/mcplexer/internal/store"
)

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
