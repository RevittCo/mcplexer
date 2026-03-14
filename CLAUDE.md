# MCPlexer

MCP Gateway (Multiplexer) — desktop app and CLI for managing MCP tool servers.

## Distribution

### Desktop App (primary)
Electron app that bundles the Go server and React UI. Runs as a menu-bar app with system tray, manages its own daemon lifecycle, and serves the web UI on `127.0.0.1:13333`.

### CLI Mode (headless)
Standalone Go binary for servers or headless environments. Runs as a launchd daemon (macOS) with the same web UI embedded via `go:embed`.

## Stack
- **Core**: Go, SQLite (modernc.org/sqlite, no CGO), net/http
- **UI**: React, TypeScript, Vite, shadcn/ui, Tailwind CSS
- **Desktop**: Electron (wraps Go binary + serves embedded UI)
- **Encryption**: filippo.io/age for secrets at rest

## Project Layout
- `electron/` — Electron shell (main process, tray, daemon lifecycle)
- `cmd/mcplexer/` — Go entry point, config loading, DI wiring
- `internal/gateway/` — MCP server (stdio), tool aggregation, dispatch
- `internal/api/` — REST API handlers
- `internal/store/` — Store interface + domain models (DB-agnostic)
- `internal/store/sqlite/` — SQLite implementation
- `internal/routing/` — route matching engine
- `internal/downstream/` — process lifecycle manager for downstream MCP servers
- `internal/auth/` — credential injection
- `internal/secrets/` — age encryption + secret storage
- `internal/audit/` — audit logging with redaction
- `internal/config/` — YAML config loader, validation
- `internal/web/` — go:embed for SPA static files
- `web/` — React SPA source

## Conventions
- Go: idiomatic, explicit error handling, table-driven tests
- Max 300 lines per file, max 50 lines per function
- TypeScript: strict mode, functional components, no `any`
- Tool namespacing: always `{namespace}__{toolname}`
- DB interface: all methods take context.Context, use sentinel errors (store.ErrNotFound)
- No ORM — raw database/sql with hand-written queries

## Commands

### Desktop App
- `make install` — build + package + install Electron app to /Applications
- `make electron-run` — build everything + launch desktop app (dev mode)
- `make electron-dev` — launch Electron without rebuilding Go/web (fast iteration)

### CLI / Headless
- `make install-cli` — build Go binary + web UI, run setup (launchd daemon)
- `make run` — build + start CLI daemon
- `make dev` — run Go server in HTTP mode (no daemon, no Electron)

### Shared
- `make build` — build Go binary + web UI
- `make test` — run Go tests
- `make lint` — run golangci-lint
- `cd web && npm run dev` — run web UI dev server (hot reload)
