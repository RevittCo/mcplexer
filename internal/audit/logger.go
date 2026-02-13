package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/revitteth/mcplexer/internal/store"
)

// Logger writes audit records with parameter redaction.
type Logger struct {
	store store.AuditStore
	scope store.AuthScopeStore
	bus   *Bus
}

// NewLogger creates an audit Logger. The bus parameter is optional (nil-safe).
func NewLogger(auditStore store.AuditStore, scopeStore store.AuthScopeStore, bus *Bus) *Logger {
	return &Logger{store: auditStore, scope: scopeStore, bus: bus}
}

// Record redacts sensitive parameters and inserts the audit record.
func (l *Logger) Record(ctx context.Context, rec *store.AuditRecord) error {
	hints, err := l.loadRedactionHints(ctx, rec.AuthScopeID)
	if err != nil {
		return fmt.Errorf("load redaction hints: %w", err)
	}

	if len(rec.ParamsRedacted) > 0 {
		rec.ParamsRedacted = Redact(rec.ParamsRedacted, hints)
	}

	if err := l.store.InsertAuditRecord(ctx, rec); err != nil {
		return fmt.Errorf("insert audit record: %w", err)
	}
	if l.bus != nil {
		l.bus.Publish(rec)
	}
	return nil
}

// loadRedactionHints fetches per-scope redaction hints from the auth scope.
func (l *Logger) loadRedactionHints(ctx context.Context, authScopeID string) ([]string, error) {
	if authScopeID == "" {
		return nil, nil
	}

	scope, err := l.scope.GetAuthScope(ctx, authScopeID)
	if err != nil {
		return nil, nil // scope not found is non-fatal for audit
	}

	if len(scope.RedactionHints) == 0 {
		return nil, nil
	}

	var hints []string
	if err := json.Unmarshal(scope.RedactionHints, &hints); err != nil {
		return nil, nil // malformed hints is non-fatal
	}
	return hints, nil
}
