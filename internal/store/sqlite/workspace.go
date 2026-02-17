package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/revittco/mcplexer/internal/store"
)

func (d *DB) CreateWorkspace(ctx context.Context, w *store.Workspace) error {
	if w.ID == "" {
		w.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	w.CreatedAt = now
	w.UpdatedAt = now

	tags := normalizeJSON(w.Tags, "[]")
	if w.Source == "" {
		w.Source = "api"
	}

	_, err := d.q.ExecContext(ctx, `
		INSERT INTO workspaces (id, name, root_path, tags, default_policy, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		w.ID, w.Name, w.RootPath, tags, w.DefaultPolicy, w.Source,
		formatTime(w.CreatedAt), formatTime(w.UpdatedAt),
	)
	if err != nil {
		return mapConstraintError(err)
	}
	return nil
}

func (d *DB) GetWorkspace(ctx context.Context, id string) (*store.Workspace, error) {
	row := d.q.QueryRowContext(ctx, `
		SELECT id, name, root_path, tags, default_policy, source, created_at, updated_at
		FROM workspaces WHERE id = ?`, id)
	return scanWorkspace(row)
}

func (d *DB) GetWorkspaceByName(ctx context.Context, name string) (*store.Workspace, error) {
	row := d.q.QueryRowContext(ctx, `
		SELECT id, name, root_path, tags, default_policy, source, created_at, updated_at
		FROM workspaces WHERE name = ?`, name)
	return scanWorkspace(row)
}

func (d *DB) ListWorkspaces(ctx context.Context) ([]store.Workspace, error) {
	rows, err := d.q.QueryContext(ctx, `
		SELECT id, name, root_path, tags, default_policy, source, created_at, updated_at
		FROM workspaces ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []store.Workspace
	for rows.Next() {
		w, err := scanWorkspaceRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *w)
	}
	return out, rows.Err()
}

func (d *DB) UpdateWorkspace(ctx context.Context, w *store.Workspace) error {
	w.UpdatedAt = time.Now().UTC()
	tags := normalizeJSON(w.Tags, "[]")
	if w.Source == "" {
		w.Source = "api"
	}

	res, err := d.q.ExecContext(ctx, `
		UPDATE workspaces
		SET name = ?, root_path = ?, tags = ?, default_policy = ?, source = ?, updated_at = ?
		WHERE id = ?`,
		w.Name, w.RootPath, tags, w.DefaultPolicy, w.Source,
		formatTime(w.UpdatedAt), w.ID,
	)
	if err != nil {
		return mapConstraintError(err)
	}
	return checkRowsAffected(res)
}

func (d *DB) DeleteWorkspace(ctx context.Context, id string) error {
	res, err := d.q.ExecContext(ctx, `DELETE FROM workspaces WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return checkRowsAffected(res)
}

func scanWorkspace(row *sql.Row) (*store.Workspace, error) {
	var w store.Workspace
	var createdAt, updatedAt, tags string
	err := row.Scan(&w.ID, &w.Name, &w.RootPath, &tags,
		&w.DefaultPolicy, &w.Source, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	w.Tags = json.RawMessage(tags)
	w.CreatedAt = parseTime(createdAt)
	w.UpdatedAt = parseTime(updatedAt)
	return &w, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanWorkspaceRow(row rowScanner) (*store.Workspace, error) {
	var w store.Workspace
	var createdAt, updatedAt, tags string
	err := row.Scan(&w.ID, &w.Name, &w.RootPath, &tags,
		&w.DefaultPolicy, &w.Source, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	w.Tags = json.RawMessage(tags)
	w.CreatedAt = parseTime(createdAt)
	w.UpdatedAt = parseTime(updatedAt)
	return &w, nil
}
