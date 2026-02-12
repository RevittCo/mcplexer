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

func (d *DB) CreateDownstreamServer(ctx context.Context, ds *store.DownstreamServer) error {
	if ds.ID == "" {
		ds.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	ds.CreatedAt = now
	ds.UpdatedAt = now

	args := normalizeJSON(ds.Args, "[]")
	caps := normalizeJSON(ds.CapabilitiesCache, "{}")

	if ds.Discovery == "" {
		ds.Discovery = "static"
	}
	if ds.Source == "" {
		ds.Source = "api"
	}

	_, err := d.q.ExecContext(ctx, `
		INSERT INTO downstream_servers
			(id, name, transport, command, args, url, tool_namespace, discovery,
			 capabilities_cache, idle_timeout_sec, max_instances, restart_policy,
			 disabled, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ds.ID, ds.Name, ds.Transport, ds.Command, args, ds.URL,
		ds.ToolNamespace, ds.Discovery, caps, ds.IdleTimeoutSec, ds.MaxInstances,
		ds.RestartPolicy, ds.Disabled, ds.Source, formatTime(ds.CreatedAt), formatTime(ds.UpdatedAt),
	)
	if err != nil {
		return mapConstraintError(err)
	}
	return nil
}

func (d *DB) GetDownstreamServer(ctx context.Context, id string) (*store.DownstreamServer, error) {
	row := d.q.QueryRowContext(ctx, `
		SELECT id, name, transport, command, args, url, tool_namespace, discovery,
		       capabilities_cache, idle_timeout_sec, max_instances, restart_policy,
		       disabled, source, created_at, updated_at
		FROM downstream_servers WHERE id = ?`, id)
	return scanDownstreamServer(row)
}

func (d *DB) GetDownstreamServerByName(ctx context.Context, name string) (*store.DownstreamServer, error) {
	row := d.q.QueryRowContext(ctx, `
		SELECT id, name, transport, command, args, url, tool_namespace, discovery,
		       capabilities_cache, idle_timeout_sec, max_instances, restart_policy,
		       disabled, source, created_at, updated_at
		FROM downstream_servers WHERE name = ?`, name)
	return scanDownstreamServer(row)
}

func (d *DB) ListDownstreamServers(ctx context.Context) ([]store.DownstreamServer, error) {
	rows, err := d.q.QueryContext(ctx, `
		SELECT id, name, transport, command, args, url, tool_namespace, discovery,
		       capabilities_cache, idle_timeout_sec, max_instances, restart_policy,
		       disabled, source, created_at, updated_at
		FROM downstream_servers ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []store.DownstreamServer
	for rows.Next() {
		ds, err := scanDownstreamServerRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *ds)
	}
	return out, rows.Err()
}

func (d *DB) UpdateDownstreamServer(ctx context.Context, ds *store.DownstreamServer) error {
	ds.UpdatedAt = time.Now().UTC()
	args := normalizeJSON(ds.Args, "[]")
	caps := normalizeJSON(ds.CapabilitiesCache, "{}")
	if ds.Source == "" {
		ds.Source = "api"
	}

	res, err := d.q.ExecContext(ctx, `
		UPDATE downstream_servers
		SET name = ?, transport = ?, command = ?, args = ?, url = ?,
		    tool_namespace = ?, discovery = ?, capabilities_cache = ?,
		    idle_timeout_sec = ?, max_instances = ?, restart_policy = ?,
		    disabled = ?, source = ?, updated_at = ?
		WHERE id = ?`,
		ds.Name, ds.Transport, ds.Command, args, ds.URL,
		ds.ToolNamespace, ds.Discovery, caps,
		ds.IdleTimeoutSec, ds.MaxInstances, ds.RestartPolicy,
		ds.Disabled, ds.Source, formatTime(ds.UpdatedAt), ds.ID,
	)
	if err != nil {
		return mapConstraintError(err)
	}
	return checkRowsAffected(res)
}

func (d *DB) DeleteDownstreamServer(ctx context.Context, id string) error {
	res, err := d.q.ExecContext(ctx, `DELETE FROM downstream_servers WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return checkRowsAffected(res)
}

func (d *DB) UpdateCapabilitiesCache(
	ctx context.Context, id string, cache json.RawMessage,
) error {
	res, err := d.q.ExecContext(ctx, `
		UPDATE downstream_servers
		SET capabilities_cache = ?, updated_at = ?
		WHERE id = ?`,
		string(cache), formatTime(time.Now().UTC()), id,
	)
	if err != nil {
		return err
	}
	return checkRowsAffected(res)
}

func scanDownstreamServer(row *sql.Row) (*store.DownstreamServer, error) {
	var ds store.DownstreamServer
	var createdAt, updatedAt, args, caps string
	err := row.Scan(
		&ds.ID, &ds.Name, &ds.Transport, &ds.Command, &args,
		&ds.URL, &ds.ToolNamespace, &ds.Discovery, &caps,
		&ds.IdleTimeoutSec, &ds.MaxInstances, &ds.RestartPolicy,
		&ds.Disabled, &ds.Source, &createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	ds.Args = json.RawMessage(args)
	ds.CapabilitiesCache = json.RawMessage(caps)
	ds.CreatedAt = parseTime(createdAt)
	ds.UpdatedAt = parseTime(updatedAt)
	return &ds, nil
}

func scanDownstreamServerRow(row rowScanner) (*store.DownstreamServer, error) {
	var ds store.DownstreamServer
	var createdAt, updatedAt, args, caps string
	err := row.Scan(
		&ds.ID, &ds.Name, &ds.Transport, &ds.Command, &args,
		&ds.URL, &ds.ToolNamespace, &ds.Discovery, &caps,
		&ds.IdleTimeoutSec, &ds.MaxInstances, &ds.RestartPolicy,
		&ds.Disabled, &ds.Source, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	ds.Args = json.RawMessage(args)
	ds.CapabilitiesCache = json.RawMessage(caps)
	ds.CreatedAt = parseTime(createdAt)
	ds.UpdatedAt = parseTime(updatedAt)
	return &ds, nil
}
