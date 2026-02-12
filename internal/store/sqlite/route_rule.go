package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/revitteth/mcplexer/internal/store"
)

func (d *DB) CreateRouteRule(ctx context.Context, r *store.RouteRule) error {
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	r.CreatedAt = now
	r.UpdatedAt = now

	toolMatch := normalizeJSON(r.ToolMatch, `["*"]`)
	if r.Source == "" {
		r.Source = "api"
	}

	_, err := d.q.ExecContext(ctx, `
		INSERT INTO route_rules
			(id, priority, workspace_id, path_glob, tool_match,
			 downstream_server_id, auth_scope_id, policy, log_level,
			 source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.Priority, r.WorkspaceID, r.PathGlob, toolMatch,
		r.DownstreamServerID, r.AuthScopeID, r.Policy, r.LogLevel,
		r.Source, formatTime(r.CreatedAt), formatTime(r.UpdatedAt),
	)
	if err != nil {
		return mapConstraintError(err)
	}
	return nil
}

func (d *DB) GetRouteRule(ctx context.Context, id string) (*store.RouteRule, error) {
	row := d.q.QueryRowContext(ctx, `
		SELECT id, priority, workspace_id, path_glob, tool_match,
		       downstream_server_id, auth_scope_id, policy, log_level,
		       source, created_at, updated_at
		FROM route_rules WHERE id = ?`, id)
	return scanRouteRule(row)
}

func (d *DB) ListRouteRules(ctx context.Context, workspaceID string) ([]store.RouteRule, error) {
	var rows *sql.Rows
	var err error
	if workspaceID != "" {
		rows, err = d.q.QueryContext(ctx, `
			SELECT id, priority, workspace_id, path_glob, tool_match,
			       downstream_server_id, auth_scope_id, policy, log_level,
			       source, created_at, updated_at
			FROM route_rules
			WHERE workspace_id = ?
			ORDER BY priority DESC, id ASC`, workspaceID)
	} else {
		rows, err = d.q.QueryContext(ctx, `
			SELECT id, priority, workspace_id, path_glob, tool_match,
			       downstream_server_id, auth_scope_id, policy, log_level,
			       source, created_at, updated_at
			FROM route_rules
			ORDER BY priority DESC, id ASC`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []store.RouteRule
	for rows.Next() {
		r, err := scanRouteRuleRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

func (d *DB) UpdateRouteRule(ctx context.Context, r *store.RouteRule) error {
	r.UpdatedAt = time.Now().UTC()
	toolMatch := normalizeJSON(r.ToolMatch, `["*"]`)
	if r.Source == "" {
		r.Source = "api"
	}

	res, err := d.q.ExecContext(ctx, `
		UPDATE route_rules
		SET priority = ?, workspace_id = ?, path_glob = ?, tool_match = ?,
		    downstream_server_id = ?, auth_scope_id = ?, policy = ?,
		    log_level = ?, source = ?, updated_at = ?
		WHERE id = ?`,
		r.Priority, r.WorkspaceID, r.PathGlob, toolMatch,
		r.DownstreamServerID, r.AuthScopeID, r.Policy,
		r.LogLevel, r.Source, formatTime(r.UpdatedAt), r.ID,
	)
	if err != nil {
		return mapConstraintError(err)
	}
	return checkRowsAffected(res)
}

func (d *DB) DeleteRouteRule(ctx context.Context, id string) error {
	res, err := d.q.ExecContext(ctx, `DELETE FROM route_rules WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return checkRowsAffected(res)
}

func scanRouteRule(row *sql.Row) (*store.RouteRule, error) {
	var r store.RouteRule
	var createdAt, updatedAt, toolMatch string
	err := row.Scan(
		&r.ID, &r.Priority, &r.WorkspaceID, &r.PathGlob, &toolMatch,
		&r.DownstreamServerID, &r.AuthScopeID, &r.Policy, &r.LogLevel,
		&r.Source, &createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	r.ToolMatch = json.RawMessage(toolMatch)
	r.CreatedAt = parseTime(createdAt)
	r.UpdatedAt = parseTime(updatedAt)
	return &r, nil
}

func scanRouteRuleRow(row rowScanner) (*store.RouteRule, error) {
	var r store.RouteRule
	var createdAt, updatedAt, toolMatch string
	err := row.Scan(
		&r.ID, &r.Priority, &r.WorkspaceID, &r.PathGlob, &toolMatch,
		&r.DownstreamServerID, &r.AuthScopeID, &r.Policy, &r.LogLevel,
		&r.Source, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	r.ToolMatch = json.RawMessage(toolMatch)
	r.CreatedAt = parseTime(createdAt)
	r.UpdatedAt = parseTime(updatedAt)
	return &r, nil
}
