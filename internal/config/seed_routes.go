package config

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/revittco/mcplexer/internal/store"
)

// defaultRouteRules defines the built-in route rules seeded on first run.
// The builtin-allow rule at high priority ensures MCPlexer built-in tools are
// accessible by default. A global deny-all at low priority ensures deny-first routing.
var defaultRouteRules = []store.RouteRule{
	{
		ID:                 "builtin-allow",
		Name:               "Allow MCPlexer built-in tools",
		Priority:           100,
		WorkspaceID:        "global",
		PathGlob:           "**",
		ToolMatch:          json.RawMessage(`["mcpx__*"]`),
		DownstreamServerID: "mcpx-builtin",
		Policy:             "allow",
		Source:             "default",
	},
	{
		ID:        "global-deny",
		Priority:  0,
		PathGlob:  "**",
		ToolMatch: json.RawMessage(`["*"]`),
		Policy:    "deny",
		LogLevel:  "info",
		Source:    "default",
	},
}

// defaultWorkspaces defines the built-in workspaces seeded on first run.
var defaultWorkspaces = []store.Workspace{
	{
		ID:            "global",
		Name:          "Global",
		RootPath:      "/",
		DefaultPolicy: "deny",
		Source:        "default",
	},
}

// SeedDefaultRouteRules creates route rules if none exist.
// Seeds a builtin-allow at high priority and global deny-all at lowest priority.
// For existing databases, ensures the builtin-allow rule exists.
func SeedDefaultRouteRules(ctx context.Context, s store.Store) error {
	existing, err := s.ListRouteRules(ctx, "")
	if err != nil {
		return err
	}

	if len(existing) > 0 {
		return ensureBuiltinAllowRoute(ctx, s, existing)
	}

	slog.Info("seeding default route rules", "count", len(defaultRouteRules))

	now := time.Now().UTC()
	for _, r := range defaultRouteRules {
		r.CreatedAt = now
		r.UpdatedAt = now
		if err := s.CreateRouteRule(ctx, &r); err != nil {
			return err
		}
		slog.Info("seeded route rule",
			"id", r.ID, "priority", r.Priority, "policy", r.Policy,
			"path_glob", r.PathGlob)
	}
	return nil
}

// ensureBuiltinAllowRoute creates the builtin-allow route rule if missing.
func ensureBuiltinAllowRoute(ctx context.Context, s store.Store, existing []store.RouteRule) error {
	for _, r := range existing {
		if r.ID == "builtin-allow" {
			return nil
		}
	}

	now := time.Now().UTC()
	r := store.RouteRule{
		ID:                 "builtin-allow",
		Name:               "Allow MCPlexer built-in tools",
		Priority:           100,
		WorkspaceID:        "global",
		PathGlob:           "**",
		ToolMatch:          json.RawMessage(`["mcpx__*"]`),
		DownstreamServerID: "mcpx-builtin",
		Policy:             "allow",
		Source:             "default",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := s.CreateRouteRule(ctx, &r); err != nil {
		return err
	}
	slog.Info("migrated: seeded builtin-allow route rule")
	return nil
}

// SeedDefaultWorkspaces creates workspace records if none exist.
func SeedDefaultWorkspaces(ctx context.Context, s store.Store) error {
	existing, err := s.ListWorkspaces(ctx)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}

	slog.Info("seeding default workspaces", "count", len(defaultWorkspaces))

	now := time.Now().UTC()
	for _, w := range defaultWorkspaces {
		w.CreatedAt = now
		w.UpdatedAt = now
		if err := s.CreateWorkspace(ctx, &w); err != nil {
			return err
		}
		slog.Info("seeded workspace", "id", w.ID, "name", w.Name)
	}
	return nil
}
