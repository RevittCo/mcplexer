package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/revittco/mcplexer/internal/store"
	_ "modernc.org/sqlite"
)

// Compile-time check that DB satisfies store.Store.
var _ store.Store = (*DB)(nil)

// queryable abstracts *sql.DB and *sql.Tx for shared query code.
type queryable interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// DB is the SQLite-backed store implementation.
type DB struct {
	db *sql.DB
	q  queryable // points to db or active tx
}

// New opens a SQLite database at the given path and runs migrations.
func New(ctx context.Context, path string) (*DB, error) {
	dsn := path + "?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	// Enable foreign keys.
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &DB{db: db, q: db}, nil
}

// Tx executes fn within a database transaction.
func (d *DB) Tx(ctx context.Context, fn func(store.Store) error) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	txDB := &DB{db: d.db, q: tx}
	if err := fn(txDB); err != nil {
		return err
	}

	return tx.Commit()
}

// Ping checks database connectivity.
func (d *DB) Ping(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

// withTx runs fn inside a transaction. If d.q is already a *sql.Tx (e.g.
// when called from config.Apply's transaction), it reuses that tx to avoid
// deadlocking on MaxOpenConns(1). Otherwise it starts a new transaction.
func (d *DB) withTx(ctx context.Context, fn func(q queryable) error) error {
	if tx, ok := d.q.(*sql.Tx); ok {
		return fn(tx)
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.db.Close()
}
