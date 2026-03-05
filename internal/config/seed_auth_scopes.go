package config

import (
	"context"
	"log/slog"
	"time"

	"github.com/revittco/mcplexer/internal/store"
)

// defaultAuthScopes defines built-in auth scopes seeded on first run.
var defaultAuthScopes = []store.AuthScope{
	{
		ID:     aikidoAuthScopeID,
		Name:   "Aikido Client Credentials",
		Type:   "env",
		Source: "default",
	},
}

// SeedDefaultAuthScopes creates auth scope records if none exist.
// For existing databases, ensures required default auth scopes exist.
func SeedDefaultAuthScopes(ctx context.Context, s store.Store) error {
	existing, err := s.ListAuthScopes(ctx)
	if err != nil {
		return err
	}

	if len(existing) > 0 {
		return ensureRequiredDefaultAuthScopes(ctx, s, existing)
	}

	slog.Info("seeding default auth scopes", "count", len(defaultAuthScopes))

	now := time.Now().UTC()
	for _, a := range defaultAuthScopes {
		a.CreatedAt = now
		a.UpdatedAt = now
		if err := s.CreateAuthScope(ctx, &a); err != nil {
			return err
		}
		slog.Info("seeded auth scope", "id", a.ID, "name", a.Name, "type", a.Type)
	}
	return nil
}

func ensureRequiredDefaultAuthScopes(ctx context.Context, s store.Store, existing []store.AuthScope) error {
	requiredIDs := []string{
		aikidoAuthScopeID,
	}

	existingByID := make(map[string]struct{}, len(existing))
	for _, scope := range existing {
		existingByID[scope.ID] = struct{}{}
	}

	now := time.Now().UTC()
	for _, id := range requiredIDs {
		if _, ok := existingByID[id]; ok {
			continue
		}

		seed, ok := defaultAuthScopeByID(id)
		if !ok {
			continue
		}
		seed.CreatedAt = now
		seed.UpdatedAt = now
		if err := s.CreateAuthScope(ctx, &seed); err != nil {
			return err
		}
		slog.Info("migrated: seeded default auth scope", "id", seed.ID, "name", seed.Name)
	}
	return nil
}

func defaultAuthScopeByID(id string) (store.AuthScope, bool) {
	for _, scope := range defaultAuthScopes {
		if scope.ID == id {
			return scope, true
		}
	}
	return store.AuthScope{}, false
}
