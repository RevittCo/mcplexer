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
		ID:                 aikidoMutateRouteID,
		Name:               "Require approval for Aikido mutating tools",
		Priority:           60,
		WorkspaceID:        "global",
		PathGlob:           "**",
		ToolMatch:          json.RawMessage(`["aikido__mutate_*"]`),
		DownstreamServerID: aikidoServerID,
		AuthScopeID:        aikidoAuthScopeID,
		Policy:             "allow",
		ApprovalMode:       "all",
		ApprovalTimeout:    300,
		Source:             "default",
	},
	{
		ID:                 aikidoReadRouteID,
		Name:               "Allow Aikido read tools",
		Priority:           50,
		WorkspaceID:        "global",
		PathGlob:           "**",
		ToolMatch:          json.RawMessage(`["aikido__read_*"]`),
		DownstreamServerID: aikidoServerID,
		AuthScopeID:        aikidoAuthScopeID,
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
// For existing databases, ensures required default rules exist.
func SeedDefaultRouteRules(ctx context.Context, s store.Store) error {
	existing, err := s.ListRouteRules(ctx, "")
	if err != nil {
		return err
	}

	if len(existing) > 0 {
		return ensureRequiredDefaultRouteRules(ctx, s, existing)
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

// ensureRequiredDefaultRouteRules creates critical default route rules if missing.
func ensureRequiredDefaultRouteRules(ctx context.Context, s store.Store, existing []store.RouteRule) error {
	requiredIDs := []string{
		"builtin-allow",
		aikidoMutateRouteID,
		aikidoReadRouteID,
	}

	existingByID := make(map[string]struct{}, len(existing))
	for _, r := range existing {
		existingByID[r.ID] = struct{}{}
	}

	now := time.Now().UTC()
	for _, id := range requiredIDs {
		if _, ok := existingByID[id]; ok {
			continue
		}

		seed, ok := defaultRouteRuleByID(id)
		if !ok {
			continue
		}

		// Skip seeding if dependencies are missing. They may be added later.
		if seed.WorkspaceID != "" {
			if _, err := s.GetWorkspace(ctx, seed.WorkspaceID); err != nil {
				slog.Warn("skipping default route seed (missing workspace)", "id", seed.ID, "workspace_id", seed.WorkspaceID)
				continue
			}
		}
		if seed.DownstreamServerID != "" {
			if _, err := s.GetDownstreamServer(ctx, seed.DownstreamServerID); err != nil {
				slog.Warn("skipping default route seed (missing downstream server)", "id", seed.ID, "downstream_server_id", seed.DownstreamServerID)
				continue
			}
		}
		if seed.AuthScopeID != "" {
			if _, err := s.GetAuthScope(ctx, seed.AuthScopeID); err != nil {
				slog.Warn("skipping default route seed (missing auth scope)", "id", seed.ID, "auth_scope_id", seed.AuthScopeID)
				continue
			}
		}

		seed.CreatedAt = now
		seed.UpdatedAt = now
		if err := s.CreateRouteRule(ctx, &seed); err != nil {
			return err
		}
		slog.Info("migrated: seeded default route rule", "id", seed.ID, "name", seed.Name)
	}
	return nil
}

func defaultRouteRuleByID(id string) (store.RouteRule, bool) {
	for _, r := range defaultRouteRules {
		if r.ID == id {
			return r, true
		}
	}
	return store.RouteRule{}, false
}

// SeedDefaultWorkspaces creates workspace records if none exist.
func SeedDefaultWorkspaces(ctx context.Context, s store.Store) error {
	existing, err := s.ListWorkspaces(ctx)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return ensureGlobalWorkspace(ctx, s, existing)
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

// ensureGlobalWorkspace creates the global workspace if missing.
func ensureGlobalWorkspace(ctx context.Context, s store.Store, existing []store.Workspace) error {
	for _, w := range existing {
		if w.ID == "global" {
			return nil
		}
	}

	now := time.Now().UTC()
	w := store.Workspace{
		ID:            "global",
		Name:          "Global",
		RootPath:      "/",
		DefaultPolicy: "deny",
		Source:        "default",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.CreateWorkspace(ctx, &w); err != nil {
		return err
	}
	slog.Info("migrated: seeded global workspace")
	return nil
}
