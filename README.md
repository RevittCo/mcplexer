# mcplexer

Directory-scoped MCP routing and tool control. Like [direnv](https://direnv.net) for MCP.

Route, scope, and secure every AI tool call based on your working directory. Local-first. Auditable. Open source.

**[Website](https://revittco.github.io/mcplexer/)** &middot; **[Issues](https://github.com/revittco/mcplexer/issues)**

## What is mcplexer?

mcplexer is an MCP gateway that sits between your AI client (Claude Desktop, Claude Code, etc.) and your downstream MCP servers. It multiplexes tool calls across servers with workspace-based routing, human-in-the-loop approvals, OAuth credential injection, and full audit logging.

Your working directory determines which policies apply — tamper-proof in stdio mode, since mcplexer reads CWD directly from the OS.

## Features

- **Directory-scoped routing** — workspaces bind to directory trees, CWD determines policies
- **Tool approvals** — per-route approval requirements with SSE streaming to the dashboard
- **OAuth 2.0 + PKCE** — built-in flows with provider templates (GitHub, Linear, Google, ClickUp), automatic token refresh
- **Audit trail** — every tool call logged with workspace, route, auth scope, latency, and parameter redaction
- **Self-configurable** — 19 MCP tools via `mcplexer control-server` for AI-native configuration
- **Desktop app** — native app with tray icon, one-click Claude Desktop setup
- **Web dashboard** — real-time metrics, approval queue, audit stream, config editor
- **age encryption** — secrets encrypted at rest with [filippo.io/age](https://filippo.io/age), auto-generated keys
- **Dry run** — test routing decisions without execution via CLI or API
- **Pure Go** — single binary, zero CGO, runs anywhere Go compiles to

## Quick Start

### One-command setup (Claude Desktop)

```bash
mcplexer setup
```

This starts the daemon, configures Claude Desktop, and opens the web dashboard. Restart Claude Desktop to connect.

### Manual setup

```bash
# Initialize database and config
mcplexer init

# Run as MCP server (stdio mode for Claude Code)
mcplexer serve --mode=stdio

# Run with web UI
mcplexer serve --mode=http --addr=:8080

# Run as background daemon with Unix socket
mcplexer daemon start --addr=:3333 --socket=/tmp/mcplexer.sock
```

### Install

```bash
go install github.com/revittco/mcplexer@latest
```

Or build from source:

```bash
git clone https://github.com/revittco/mcplexer.git
cd mcplexer
make build
# Binary at ./bin/mcplexer
```

## Configuration

mcplexer supports four configuration methods:

| Method | Use case |
|--------|----------|
| **Desktop app** | One-click setup, tray icon, auto-starts daemon |
| **YAML config** | Version-controlled, seeds database on startup |
| **Web UI + REST API** | Visual management, real-time dashboard |
| **MCP control server** | AI-native configuration from Claude or any MCP client |

### YAML config

Default location: `~/.mcplexer/mcplexer.yaml`

```yaml
workspaces:
  - name: frontend
    root_path: ~/projects/app
    default_policy: deny

servers:
  - name: github
    namespace: github
    command: npx
    args: ["-y", "@modelcontextprotocol/server-github"]

rules:
  - name: allow-github
    workspace: frontend
    tool_pattern: "github__*"
    policy: allow
    priority: 10
```

YAML-sourced items are auto-pruned when removed from the config file. Items created via API or UI persist independently.

### Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MCPLEXER_MODE` | `stdio` | Transport mode: `stdio` or `http` |
| `MCPLEXER_HTTP_ADDR` | `:8080` | HTTP listen address |
| `MCPLEXER_DB_DSN` | `~/.mcplexer/mcplexer.db` | Database path |
| `MCPLEXER_CONFIG` | `~/.mcplexer/mcplexer.yaml` | Config file path |
| `MCPLEXER_AGE_KEY` | auto-generated | Path to age identity file |
| `MCPLEXER_SOCKET_PATH` | — | Unix socket path for multi-client mode |
| `MCPLEXER_EXTERNAL_URL` | — | External URL for OAuth callbacks |
| `MCPLEXER_LOG_LEVEL` | `info` | Log level: debug, info, warn, error |

## CLI Commands

```
mcplexer serve          Run MCP server (default: stdio mode)
mcplexer connect        Bridge stdio to daemon's Unix socket
mcplexer setup          One-command Claude Desktop integration
mcplexer init           Initialize database and default config
mcplexer status         Show workspaces, servers, auth scopes, sessions
mcplexer dry-run        Test routing rules without execution
mcplexer secret         Manage encrypted secrets (put/get/list/delete)
mcplexer daemon         Background process management (start/stop/status/logs)
mcplexer control-server Run MCP control protocol server (19 tools)
```

## How Routing Works

1. **CWD resolution** — in stdio mode, mcplexer reads `os.Getwd()` to determine the client's working directory
2. **Workspace matching** — the most specific matching workspace wins (longest path prefix)
3. **Rule evaluation** — rules are sorted by path glob specificity, then tool specificity, then priority
4. **Deny-first** — deny rules stop the chain immediately
5. **Approval** — if the matching rule requires approval, the request is held until resolved via the dashboard
6. **Dispatch** — tool call is forwarded to the downstream server with injected credentials

## Project Structure

```
cmd/mcplexer/       Entry point, CLI subcommands, config loading
internal/
  store/            Store interface + domain models (DB-agnostic)
  store/sqlite/     SQLite implementation (pure Go, no CGO)
  gateway/          MCP server, JSON-RPC protocol, tool aggregation
  routing/          Route matching engine
  downstream/       Process lifecycle manager
  auth/             Credential injection
  secrets/          age encryption + secret storage
  audit/            Audit logging with redaction
  approval/         Tool call approval system
  config/           YAML config loader, validation, seeding
  api/              REST API handlers (/api/v1/)
  oauth/            OAuth 2.0 flow management
  control/          MCP control protocol server
  web/              go:embed for SPA static files
web/                React SPA source (Vite + TypeScript + Tailwind)
site/               Marketing website (Next.js, deployed to GitHub Pages)
```

## Development

```bash
# Backend (HTTP mode)
make dev

# Frontend dev server
cd web && npm run dev

# Run tests
make test

# Lint
make lint
```

**Requirements:** Go 1.25+, Node 20+

## Tech Stack

- **Backend:** Go, SQLite ([modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)), net/http
- **Frontend:** React 19, TypeScript, Vite, Tailwind CSS v4, shadcn/ui
- **Encryption:** [filippo.io/age](https://filippo.io/age) for secrets at rest
- **Config:** YAML ([gopkg.in/yaml.v3](https://pkg.go.dev/gopkg.in/yaml.v3))

## License

MIT
