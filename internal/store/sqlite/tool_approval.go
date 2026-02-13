package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/revitteth/mcplexer/internal/store"
)

func (d *DB) CreateToolApproval(ctx context.Context, a *store.ToolApproval) error {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	if a.Status == "" {
		a.Status = "pending"
	}

	_, err := d.q.ExecContext(ctx, `
		INSERT INTO tool_approvals
			(id, status, request_session_id, request_client_type, request_model,
			 workspace_id, tool_name, arguments, justification,
			 route_rule_id, downstream_server_id, auth_scope_id,
			 approver_session_id, approver_type, resolution,
			 timeout_sec, created_at, resolved_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.Status, a.RequestSessionID, a.RequestClientType, a.RequestModel,
		a.WorkspaceID, a.ToolName, a.Arguments, a.Justification,
		a.RouteRuleID, a.DownstreamServerID, a.AuthScopeID,
		a.ApproverSessionID, a.ApproverType, a.Resolution,
		a.TimeoutSec, formatTime(a.CreatedAt), formatTimePtr(a.ResolvedAt),
	)
	return err
}

func (d *DB) GetToolApproval(ctx context.Context, id string) (*store.ToolApproval, error) {
	row := d.q.QueryRowContext(ctx, `
		SELECT id, status, request_session_id, request_client_type, request_model,
		       workspace_id, tool_name, arguments, justification,
		       route_rule_id, downstream_server_id, auth_scope_id,
		       approver_session_id, approver_type, resolution,
		       timeout_sec, created_at, resolved_at
		FROM tool_approvals WHERE id = ?`, id)

	return scanToolApproval(row)
}

func (d *DB) ListPendingApprovals(ctx context.Context) ([]store.ToolApproval, error) {
	rows, err := d.q.QueryContext(ctx, `
		SELECT id, status, request_session_id, request_client_type, request_model,
		       workspace_id, tool_name, arguments, justification,
		       route_rule_id, downstream_server_id, auth_scope_id,
		       approver_session_id, approver_type, resolution,
		       timeout_sec, created_at, resolved_at
		FROM tool_approvals
		WHERE status = 'pending'
		ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []store.ToolApproval
	for rows.Next() {
		a, err := scanToolApprovalRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

func (d *DB) ResolveToolApproval(
	ctx context.Context,
	id, status, approverSessionID, approverType, resolution string,
) error {
	now := formatTime(time.Now().UTC())
	res, err := d.q.ExecContext(ctx, `
		UPDATE tool_approvals
		SET status = ?, approver_session_id = ?, approver_type = ?,
		    resolution = ?, resolved_at = ?
		WHERE id = ? AND status = 'pending'`,
		status, approverSessionID, approverType, resolution, now, id,
	)
	if err != nil {
		return err
	}
	return checkRowsAffected(res)
}

func (d *DB) ExpirePendingApprovals(ctx context.Context, before time.Time) (int, error) {
	res, err := d.q.ExecContext(ctx, `
		UPDATE tool_approvals
		SET status = 'timeout', resolved_at = ?
		WHERE status = 'pending' AND created_at < ?`,
		formatTime(time.Now().UTC()), formatTime(before),
	)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

func scanToolApproval(row *sql.Row) (*store.ToolApproval, error) {
	var a store.ToolApproval
	var createdAt string
	var resolvedAt *string
	err := row.Scan(
		&a.ID, &a.Status, &a.RequestSessionID, &a.RequestClientType, &a.RequestModel,
		&a.WorkspaceID, &a.ToolName, &a.Arguments, &a.Justification,
		&a.RouteRuleID, &a.DownstreamServerID, &a.AuthScopeID,
		&a.ApproverSessionID, &a.ApproverType, &a.Resolution,
		&a.TimeoutSec, &createdAt, &resolvedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	a.CreatedAt = parseTime(createdAt)
	a.ResolvedAt = parseTimePtr(resolvedAt)
	return &a, nil
}

func scanToolApprovalRow(row rowScanner) (*store.ToolApproval, error) {
	var a store.ToolApproval
	var createdAt string
	var resolvedAt *string
	err := row.Scan(
		&a.ID, &a.Status, &a.RequestSessionID, &a.RequestClientType, &a.RequestModel,
		&a.WorkspaceID, &a.ToolName, &a.Arguments, &a.Justification,
		&a.RouteRuleID, &a.DownstreamServerID, &a.AuthScopeID,
		&a.ApproverSessionID, &a.ApproverType, &a.Resolution,
		&a.TimeoutSec, &createdAt, &resolvedAt,
	)
	if err != nil {
		return nil, err
	}
	a.CreatedAt = parseTime(createdAt)
	a.ResolvedAt = parseTimePtr(resolvedAt)
	return &a, nil
}
