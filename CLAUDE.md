# MCPlexer

MCP Gateway (Multiplexer) with Web UI.

## Stack
- **Backend**: Go, SQLite (modernc.org/sqlite, no CGO), net/http
- **Frontend**: React, TypeScript, Vite, shadcn/ui, Tailwind CSS
- **Encryption**: filippo.io/age for secrets at rest

## Project Layout
- `cmd/mcplexer/` — entry point, config loading, DI wiring
- `internal/store/` — Store interface + domain models (DB-agnostic)
- `internal/store/sqlite/` — SQLite implementation
- `internal/gateway/` — MCP server (stdio), tool aggregation, dispatch
- `internal/routing/` — route matching engine
- `internal/downstream/` — process lifecycle manager for downstream MCP servers
- `internal/auth/` — credential injection
- `internal/secrets/` — age encryption + secret storage
- `internal/audit/` — audit logging with redaction
- `internal/config/` — YAML config loader, validation
- `internal/api/` — REST API handlers
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
- `make build` — build Go binary + web UI
- `make dev` — run in HTTP mode for development
- `make test` — run Go tests
- `make lint` — run golangci-lint
- `cd web && npm run dev` — run web UI dev server
