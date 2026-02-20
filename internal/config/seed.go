package config

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/revittco/mcplexer/internal/oauth"
	"github.com/revittco/mcplexer/internal/store"
)

// SeedDefaultOAuthProviders creates or updates OAuth provider records from
// built-in templates. On first startup it creates all providers; on subsequent
// runs it updates template-sourced fields (URLs, scopes, PKCE) on existing
// seeded providers while preserving user-configured fields (client ID/secret).
// If scopes changed, invalidates existing tokens so users re-auth with correct permissions.
func SeedDefaultOAuthProviders(ctx context.Context, s store.Store) error {
	existing, err := s.ListOAuthProviders(ctx)
	if err != nil {
		return err
	}

	// Index existing providers by template_id for fast lookup.
	byTemplate := make(map[string]*store.OAuthProvider, len(existing))
	for i := range existing {
		if existing[i].TemplateID != "" {
			byTemplate[existing[i].TemplateID] = &existing[i]
		}
	}

	templates := oauth.ListTemplates()
	for _, t := range templates {
		scopes, _ := json.Marshal(t.Scopes)
		now := time.Now().UTC()

		if ep, ok := byTemplate[t.ID]; ok {
			scopesChanged := string(ep.Scopes) != string(scopes)

			// Update template-sourced fields; preserve client credentials.
			ep.AuthorizeURL = t.AuthorizeURL
			ep.TokenURL = t.TokenURL
			ep.Scopes = scopes
			ep.UsePKCE = t.UsePKCE
			ep.UpdatedAt = now
			if err := s.UpdateOAuthProvider(ctx, ep); err != nil {
				return err
			}

			// If scopes changed, invalidate tokens on linked auth scopes
			// so users re-auth with the correct permissions.
			if scopesChanged {
				invalidateProviderTokens(ctx, s, ep.ID)
				slog.Info("updated seeded OAuth provider (scopes changed, tokens invalidated)",
					"id", ep.ID, "name", ep.Name)
			} else {
				slog.Info("updated seeded OAuth provider",
					"id", ep.ID, "name", ep.Name)
			}
			continue
		}

		p := &store.OAuthProvider{
			ID:           t.ID,
			Name:         t.Name,
			TemplateID:   t.ID,
			AuthorizeURL: t.AuthorizeURL,
			TokenURL:     t.TokenURL,
			Scopes:       scopes,
			UsePKCE:      t.UsePKCE,
			Source:       "default",
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := s.CreateOAuthProvider(ctx, p); err != nil {
			return err
		}
		slog.Info("seeded OAuth provider", "id", t.ID, "name", t.Name)
	}
	return nil
}

// defaultDownstreamServers defines the built-in MCP servers seeded on first run.
// Servers marked Disabled require user configuration (API keys, paths) before use.
var defaultDownstreamServers = []store.DownstreamServer{
	// ── HTTP (Streamable HTTP) servers ──
	{
		ID:             "github",
		Name:           "GitHub",
		Transport:      "http",
		URL:            strPtr("https://api.githubcopilot.com/mcp/"),
		ToolNamespace:  "github",
		Discovery:      "dynamic",
		IdleTimeoutSec: 300,
		MaxInstances:   1,
		RestartPolicy:  "on-failure",
		Source:         "default",
	},
	{
		ID:             "linear",
		Name:           "Linear",
		Transport:      "http",
		URL:            strPtr("https://mcp.linear.app/mcp"),
		ToolNamespace:  "linear",
		Discovery:      "dynamic",
		IdleTimeoutSec: 300,
		MaxInstances:   1,
		RestartPolicy:  "on-failure",
		Source:         "default",
	},
	{
		ID:             "clickup",
		Name:           "ClickUp",
		Transport:      "http",
		URL:            strPtr("https://mcp.clickup.com/mcp"),
		ToolNamespace:  "clickup",
		Discovery:      "dynamic",
		IdleTimeoutSec: 300,
		MaxInstances:   1,
		RestartPolicy:  "on-failure",
		Source:         "default",
	},

	// ── Stdio servers — no auth required ──
	{
		ID:             "filesystem",
		Name:           "Filesystem",
		Transport:      "stdio",
		Command:        "npx",
		Args:           json.RawMessage(`["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]`),
		ToolNamespace:  "fs",
		Discovery:      "dynamic",
		IdleTimeoutSec: 300,
		MaxInstances:   1,
		RestartPolicy:  "on-failure",
		Disabled:       true,
		Source:         "default",
	},
	{
		ID:             "memory",
		Name:           "Memory",
		Transport:      "stdio",
		Command:        "npx",
		Args:           json.RawMessage(`["-y", "@modelcontextprotocol/server-memory"]`),
		ToolNamespace:  "memory",
		Discovery:      "dynamic",
		IdleTimeoutSec: 300,
		MaxInstances:   1,
		RestartPolicy:  "on-failure",
		Disabled:       true,
		Source:         "default",
	},
	{
		ID:             "sequential-thinking",
		Name:           "Sequential Thinking",
		Transport:      "stdio",
		Command:        "npx",
		Args:           json.RawMessage(`["-y", "@modelcontextprotocol/server-sequential-thinking"]`),
		ToolNamespace:  "thinking",
		Discovery:      "dynamic",
		IdleTimeoutSec: 300,
		MaxInstances:   1,
		RestartPolicy:  "on-failure",
		Disabled:       true,
		Source:         "default",
	},
	{
		ID:             "fetch",
		Name:           "Fetch",
		Transport:      "stdio",
		Command:        "uvx",
		Args:           json.RawMessage(`["mcp-server-fetch"]`),
		ToolNamespace:  "fetch",
		Discovery:      "dynamic",
		IdleTimeoutSec: 300,
		MaxInstances:   1,
		RestartPolicy:  "on-failure",
		Disabled:       true,
		Source:         "default",
	},
	{
		ID:             "git",
		Name:           "Git",
		Transport:      "stdio",
		Command:        "uvx",
		Args:           json.RawMessage(`["mcp-server-git"]`),
		ToolNamespace:  "git",
		Discovery:      "dynamic",
		IdleTimeoutSec: 300,
		MaxInstances:   1,
		RestartPolicy:  "on-failure",
		Disabled:       true,
		Source:         "default",
	},
	{
		ID:             "playwright",
		Name:           "Playwright",
		Transport:      "stdio",
		Command:        "npx",
		Args:           json.RawMessage(`["-y", "@playwright/mcp@latest"]`),
		ToolNamespace:  "playwright",
		Discovery:      "dynamic",
		IdleTimeoutSec: 300,
		MaxInstances:   1,
		RestartPolicy:  "on-failure",
		Disabled:       true,
		Source:         "default",
	},

	// ── Stdio servers — require auth / config ──
	{
		ID:             "brave-search",
		Name:           "Brave Search",
		Transport:      "stdio",
		Command:        "npx",
		Args:           json.RawMessage(`["-y", "@modelcontextprotocol/server-brave-search"]`),
		ToolNamespace:  "brave",
		Discovery:      "dynamic",
		IdleTimeoutSec: 300,
		MaxInstances:   1,
		RestartPolicy:  "on-failure",
		Disabled:       true,
		Source:         "default",
	},
	{
		ID:             "slack",
		Name:           "Slack",
		Transport:      "stdio",
		Command:        "npx",
		Args:           json.RawMessage(`["-y", "@modelcontextprotocol/server-slack"]`),
		ToolNamespace:  "slack",
		Discovery:      "dynamic",
		IdleTimeoutSec: 300,
		MaxInstances:   1,
		RestartPolicy:  "on-failure",
		Disabled:       true,
		Source:         "default",
	},
	{
		ID:             "notion",
		Name:           "Notion",
		Transport:      "stdio",
		Command:        "npx",
		Args:           json.RawMessage(`["-y", "@notionhq/notion-mcp-server"]`),
		ToolNamespace:  "notion",
		Discovery:      "dynamic",
		IdleTimeoutSec: 300,
		MaxInstances:   1,
		RestartPolicy:  "on-failure",
		Disabled:       true,
		Source:         "default",
	},
	{
		ID:             "stripe",
		Name:           "Stripe",
		Transport:      "stdio",
		Command:        "npx",
		Args:           json.RawMessage(`["-y", "@stripe/mcp", "--tools=all"]`),
		ToolNamespace:  "stripe",
		Discovery:      "dynamic",
		IdleTimeoutSec: 300,
		MaxInstances:   1,
		RestartPolicy:  "on-failure",
		Disabled:       true,
		Source:         "default",
	},
	{
		ID:             "sentry",
		Name:           "Sentry",
		Transport:      "stdio",
		Command:        "uvx",
		Args:           json.RawMessage(`["mcp-server-sentry"]`),
		ToolNamespace:  "sentry",
		Discovery:      "dynamic",
		IdleTimeoutSec: 300,
		MaxInstances:   1,
		RestartPolicy:  "on-failure",
		Disabled:       true,
		Source:         "default",
	},
	{
		ID:             "grafana",
		Name:           "Grafana",
		Transport:      "stdio",
		Command:        "uvx",
		Args:           json.RawMessage(`["mcp-grafana"]`),
		ToolNamespace:  "grafana",
		Discovery:      "dynamic",
		IdleTimeoutSec: 300,
		MaxInstances:   1,
		RestartPolicy:  "on-failure",
		Disabled:       true,
		Source:         "default",
	},

	// ── Database servers ──
	{
		ID:             "postgres",
		Name:           "PostgreSQL",
		Transport:      "stdio",
		Command:        "npx",
		Args:           json.RawMessage(`["-y", "@modelcontextprotocol/server-postgres", "postgresql://localhost:5432/mydb"]`),
		ToolNamespace:  "postgres",
		Discovery:      "dynamic",
		IdleTimeoutSec: 300,
		MaxInstances:   1,
		RestartPolicy:  "on-failure",
		Disabled:       true,
		Source:         "default",
	},
	{
		ID:             "sqlite",
		Name:           "SQLite",
		Transport:      "stdio",
		Command:        "uvx",
		Args:           json.RawMessage(`["mcp-server-sqlite", "--db-path", "./data/mydb.db"]`),
		ToolNamespace:  "sqlite",
		Discovery:      "dynamic",
		IdleTimeoutSec: 300,
		MaxInstances:   1,
		RestartPolicy:  "on-failure",
		Disabled:       true,
		Source:         "default",
	},
	{
		ID:             "redis",
		Name:           "Redis",
		Transport:      "stdio",
		Command:        "npx",
		Args:           json.RawMessage(`["-y", "@modelcontextprotocol/server-redis", "redis://localhost:6379"]`),
		ToolNamespace:  "redis",
		Discovery:      "dynamic",
		IdleTimeoutSec: 300,
		MaxInstances:   1,
		RestartPolicy:  "on-failure",
		Disabled:       true,
		Source:         "default",
	},
	{
		ID:             "supabase",
		Name:           "Supabase",
		Transport:      "stdio",
		Command:        "npx",
		Args:           json.RawMessage(`["-y", "@supabase/mcp-server-supabase@latest", "--read-only"]`),
		ToolNamespace:  "supabase",
		Discovery:      "dynamic",
		IdleTimeoutSec: 300,
		MaxInstances:   1,
		RestartPolicy:  "on-failure",
		Disabled:       true,
		Source:         "default",
	},
	{
		ID:             "cloudflare",
		Name:           "Cloudflare",
		Transport:      "stdio",
		Command:        "npx",
		Args:           json.RawMessage(`["-y", "@cloudflare/mcp-server-cloudflare"]`),
		ToolNamespace:  "cloudflare",
		Discovery:      "dynamic",
		IdleTimeoutSec: 300,
		MaxInstances:   1,
		RestartPolicy:  "on-failure",
		Disabled:       true,
		Source:         "default",
	},

	// ── Internal ──
	{
		ID:             "mcplexer",
		Name:           "MCPlexer Control",
		Transport:      "stdio",
		Command:        "mcplexer",
		Args:           json.RawMessage(`["control-server"]`),
		ToolNamespace:  "mcplexer",
		Discovery:      "dynamic",
		IdleTimeoutSec: 0,
		MaxInstances:   1,
		RestartPolicy:  "on-failure",
		Source:         "default",
	},
	{
		ID:            "mcpx-builtin",
		Name:          "MCPlexer Built-in Tools",
		Transport:     "internal",
		ToolNamespace: "mcpx",
		Discovery:     "static",
		Source:        "default",
	},
}

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

func strPtr(s string) *string { return &s }

// invalidateProviderTokens clears OAuth token data on all auth scopes
// linked to the given provider, forcing users to re-authenticate.
func invalidateProviderTokens(ctx context.Context, s store.Store, providerID string) {
	scopes, err := s.ListAuthScopes(ctx)
	if err != nil {
		slog.Warn("failed to list auth scopes for token invalidation", "error", err)
		return
	}
	for _, scope := range scopes {
		if scope.OAuthProviderID != providerID {
			continue
		}
		if err := s.UpdateAuthScopeTokenData(ctx, scope.ID, nil); err != nil {
			slog.Warn("failed to invalidate token",
				"scope_id", scope.ID, "error", err)
			continue
		}
		slog.Info("invalidated token for scope (provider scopes changed)",
			"scope_id", scope.ID, "scope_name", scope.Name)
	}
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

// SeedDefaultRouteRules creates route rules if none exist.
// Seeds a builtin-allow at high priority and global deny-all at lowest priority.
// For existing databases, ensures the builtin-allow rule exists.
func SeedDefaultRouteRules(ctx context.Context, s store.Store) error {
	existing, err := s.ListRouteRules(ctx, "")
	if err != nil {
		return err
	}

	if len(existing) > 0 {
		// Existing DB: ensure builtin-allow route exists.
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
			return nil // already present
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

// SeedDefaultDownstreamServers creates downstream server records if none exist.
// For existing databases, ensures the mcpx-builtin virtual server exists.
func SeedDefaultDownstreamServers(ctx context.Context, s store.Store) error {
	existing, err := s.ListDownstreamServers(ctx)
	if err != nil {
		return err
	}

	if len(existing) > 0 {
		// Existing DB: ensure mcpx-builtin virtual server is present.
		return ensureBuiltinServer(ctx, s, existing)
	}

	slog.Info("seeding default downstream servers",
		"count", len(defaultDownstreamServers))

	now := time.Now().UTC()
	for _, d := range defaultDownstreamServers {
		d.CreatedAt = now
		d.UpdatedAt = now
		if err := s.CreateDownstreamServer(ctx, &d); err != nil {
			return err
		}
		slog.Info("seeded downstream server",
			"id", d.ID, "name", d.Name, "transport", d.Transport)
	}
	return nil
}

// ensureBuiltinServer creates the mcpx-builtin virtual server if missing.
func ensureBuiltinServer(ctx context.Context, s store.Store, existing []store.DownstreamServer) error {
	for _, srv := range existing {
		if srv.ID == "mcpx-builtin" {
			return nil // already present
		}
	}

	now := time.Now().UTC()
	d := store.DownstreamServer{
		ID:            "mcpx-builtin",
		Name:          "MCPlexer Built-in Tools",
		Transport:     "internal",
		ToolNamespace: "mcpx",
		Discovery:     "static",
		Source:        "default",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.CreateDownstreamServer(ctx, &d); err != nil {
		return err
	}
	slog.Info("migrated: seeded mcpx-builtin virtual server")
	return nil
}
