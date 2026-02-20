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
// For existing databases, ensures the mcpx-builtin virtual server exists.
func SeedDefaultDownstreamServers(ctx context.Context, s store.Store) error {
	existing, err := s.ListDownstreamServers(ctx)
	if err != nil {
		return err
	}

	if len(existing) > 0 {
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
			return nil
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
