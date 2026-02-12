package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/revitteth/mcplexer/internal/store"
)

func (d *DB) InsertAuditRecord(ctx context.Context, r *store.AuditRecord) error {
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	if r.Timestamp.IsZero() {
		r.Timestamp = time.Now().UTC()
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}

	params := normalizeJSON(r.ParamsRedacted, "{}")

	_, err := d.q.ExecContext(ctx, `
		INSERT INTO audit_records
			(id, timestamp, session_id, client_type, model, workspace_id,
			 subpath, tool_name, params_redacted, route_rule_id,
			 downstream_server_id, downstream_instance_id, auth_scope_id,
			 status, error_code, error_message, latency_ms, response_size,
			 created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, formatTime(r.Timestamp), r.SessionID, r.ClientType, r.Model,
		r.WorkspaceID, r.Subpath, r.ToolName, params, r.RouteRuleID,
		r.DownstreamServerID, r.DownstreamInstanceID, r.AuthScopeID,
		r.Status, r.ErrorCode, r.ErrorMessage, r.LatencyMs, r.ResponseSize,
		formatTime(r.CreatedAt),
	)
	return err
}

func (d *DB) QueryAuditRecords(
	ctx context.Context, f store.AuditFilter,
) ([]store.AuditRecord, int, error) {
	where, args := buildAuditWhere(f)

	// Count total.
	var total int
	countQ := "SELECT COUNT(*) FROM audit_records" + where
	if err := d.q.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Fetch page.
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	dataQ := `SELECT id, timestamp, session_id, client_type, model, workspace_id,
		subpath, tool_name, params_redacted, route_rule_id,
		downstream_server_id, downstream_instance_id, auth_scope_id,
		status, error_code, error_message, latency_ms, response_size, created_at
		FROM audit_records` + where +
		` ORDER BY timestamp DESC LIMIT ? OFFSET ?`
	dataArgs := append(args, limit, f.Offset)

	rows, err := d.q.QueryContext(ctx, dataQ, dataArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []store.AuditRecord
	for rows.Next() {
		r, err := scanAuditRow(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *r)
	}
	return out, total, rows.Err()
}

func (d *DB) GetAuditStats(
	ctx context.Context, workspaceID string, after, before time.Time,
) (*store.AuditStats, error) {
	var s store.AuditStats

	var whereClause string
	var args []any
	if workspaceID != "" {
		whereClause = "WHERE workspace_id = ? AND timestamp >= ? AND timestamp <= ?"
		args = []any{workspaceID, formatTime(after), formatTime(before)}
	} else {
		whereClause = "WHERE timestamp >= ? AND timestamp <= ?"
		args = []any{formatTime(after), formatTime(before)}
	}

	err := d.q.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE status = 'success'),
			COUNT(*) FILTER (WHERE status = 'error'),
			COALESCE(AVG(latency_ms), 0)
		FROM audit_records
		`+whereClause,
		args...,
	).Scan(&s.TotalRequests, &s.SuccessCount, &s.ErrorCount, &s.AvgLatencyMs)
	if err != nil {
		return nil, err
	}

	// P95 latency approximation.
	err = d.q.QueryRowContext(ctx, `
		SELECT COALESCE(latency_ms, 0) FROM audit_records
		`+whereClause+`
		ORDER BY latency_ms ASC
		LIMIT 1 OFFSET (
			SELECT CAST(COUNT(*) * 0.95 AS INTEGER) FROM audit_records
			`+whereClause+`
		)`,
		append(args, args...)...,
	).Scan(&s.P95LatencyMs)
	if err != nil {
		// No rows is fine — P95 stays 0.
		s.P95LatencyMs = 0
	}
	return &s, nil
}

func (d *DB) GetDashboardTimeSeries(
	ctx context.Context, after, before time.Time,
) ([]store.TimeSeriesPoint, error) {
	rows, err := d.q.QueryContext(ctx, `
		SELECT
			strftime('%Y-%m-%dT%H:%M:00Z', timestamp) AS bucket,
			COUNT(DISTINCT session_id) AS sessions,
			COUNT(DISTINCT downstream_server_id) AS servers,
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE status = 'error') AS errors
		FROM audit_records
		WHERE timestamp >= ? AND timestamp <= ?
		GROUP BY bucket
		ORDER BY bucket ASC`,
		formatTime(after), formatTime(before),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []store.TimeSeriesPoint
	for rows.Next() {
		var p store.TimeSeriesPoint
		var bucket string
		if err := rows.Scan(&bucket, &p.Sessions, &p.Servers, &p.Total, &p.Errors); err != nil {
			return nil, fmt.Errorf("scan time series row: %w", err)
		}
		p.Bucket = parseTime(bucket)
		out = append(out, p)
	}
	return out, rows.Err()
}

func buildAuditWhere(f store.AuditFilter) (string, []any) {
	var conds []string
	var args []any
	if f.SessionID != nil {
		conds = append(conds, "session_id = ?")
		args = append(args, *f.SessionID)
	}
	if f.WorkspaceID != nil {
		conds = append(conds, "workspace_id = ?")
		args = append(args, *f.WorkspaceID)
	}
	if f.ToolName != nil {
		conds = append(conds, "tool_name = ?")
		args = append(args, *f.ToolName)
	}
	if f.Status != nil {
		conds = append(conds, "status = ?")
		args = append(args, *f.Status)
	}
	if f.After != nil {
		conds = append(conds, "timestamp >= ?")
		args = append(args, formatTime(*f.After))
	}
	if f.Before != nil {
		conds = append(conds, "timestamp <= ?")
		args = append(args, formatTime(*f.Before))
	}
	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func scanAuditRow(row rowScanner) (*store.AuditRecord, error) {
	var r store.AuditRecord
	var ts, createdAt, params string
	err := row.Scan(
		&r.ID, &ts, &r.SessionID, &r.ClientType, &r.Model,
		&r.WorkspaceID, &r.Subpath, &r.ToolName, &params,
		&r.RouteRuleID, &r.DownstreamServerID, &r.DownstreamInstanceID,
		&r.AuthScopeID, &r.Status, &r.ErrorCode, &r.ErrorMessage,
		&r.LatencyMs, &r.ResponseSize, &createdAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan audit row: %w", err)
	}
	r.ParamsRedacted = json.RawMessage(params)
	r.Timestamp = parseTime(ts)
	r.CreatedAt = parseTime(createdAt)
	return &r, nil
}
