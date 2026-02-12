package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func migrate(ctx context.Context, db *sql.DB) error {
	if err := ensureSchemaTable(ctx, db); err != nil {
		return fmt.Errorf("ensure schema table: %w", err)
	}

	current, err := currentVersion(ctx, db)
	if err != nil {
		return fmt.Errorf("get current version: %w", err)
	}

	files, err := listMigrations()
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}

	for _, f := range files {
		ver := f.version
		if ver <= current {
			continue
		}
		if err := applyMigration(ctx, db, f); err != nil {
			return fmt.Errorf("apply migration %d: %w", ver, err)
		}
	}
	return nil
}

func ensureSchemaTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)`)
	return err
}

func currentVersion(ctx context.Context, db *sql.DB) (int, error) {
	var v int
	err := db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version), 0) FROM schema_version`,
	).Scan(&v)
	return v, err
}

type migrationFile struct {
	version  int
	filename string
}

func listMigrations() ([]migrationFile, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, err
	}

	var files []migrationFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		var ver int
		if _, err := fmt.Sscanf(e.Name(), "%03d_", &ver); err != nil {
			continue
		}
		files = append(files, migrationFile{version: ver, filename: e.Name()})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].version < files[j].version
	})
	return files, nil
}

func applyMigration(ctx context.Context, db *sql.DB, f migrationFile) error {
	data, err := migrationsFS.ReadFile("migrations/" + f.filename)
	if err != nil {
		return fmt.Errorf("read %s: %w", f.filename, err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx, string(data)); err != nil {
		return fmt.Errorf("exec %s: %w", f.filename, err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO schema_version (version, applied_at) VALUES (?, datetime('now'))`,
		f.version,
	)
	if err != nil {
		return fmt.Errorf("record version: %w", err)
	}

	return tx.Commit()
}
