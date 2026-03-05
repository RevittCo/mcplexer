package config

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/revittco/mcplexer/internal/store"
)

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
	{
		ID:             aikidoServerID,
		Name:           "Aikido",
		Transport:      "stdio",
		Command:        "aikido-mcp",
		ToolNamespace:  "aikido",
		Discovery:      "dynamic",
		IdleTimeoutSec: 300,
		MaxInstances:   1,
		RestartPolicy:  "on-failure",
		Disabled:       true,
		Source:         "default",
	},
	{
		ID:             "portainer",
		Name:           "Portainer",
		Transport:      "stdio",
		Command:        "portainer-mcp",
		Args:           json.RawMessage(`["-server", "localhost:9443", "-token", "YOUR_TOKEN"]`),
		ToolNamespace:  "portainer",
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

// SeedDefaultDownstreamServers creates downstream server records if none exist.
// For existing databases, ensures required default servers exist.
func SeedDefaultDownstreamServers(ctx context.Context, s store.Store) error {
	existing, err := s.ListDownstreamServers(ctx)
	if err != nil {
		return err
	}

	if len(existing) > 0 {
		return ensureRequiredDefaultServers(ctx, s, existing)
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

// ensureRequiredDefaultServers creates critical default servers if missing.
func ensureRequiredDefaultServers(ctx context.Context, s store.Store, existing []store.DownstreamServer) error {
	requiredIDs := []string{
		"mcpx-builtin",
		aikidoServerID,
	}

	existingByID := make(map[string]struct{}, len(existing))
	for _, srv := range existing {
		existingByID[srv.ID] = struct{}{}
	}

	now := time.Now().UTC()
	for _, id := range requiredIDs {
		if _, ok := existingByID[id]; ok {
			continue
		}

		seed, ok := defaultDownstreamServerByID(id)
		if !ok {
			continue
		}
		seed.CreatedAt = now
		seed.UpdatedAt = now

		if err := s.CreateDownstreamServer(ctx, &seed); err != nil {
			return err
		}
		slog.Info("migrated: seeded default downstream server", "id", seed.ID, "name", seed.Name)
	}
	return nil
}

func defaultDownstreamServerByID(id string) (store.DownstreamServer, bool) {
	for _, srv := range defaultDownstreamServers {
		if srv.ID == id {
			return srv, true
		}
	}
	return store.DownstreamServer{}, false
}
