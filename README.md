# MCPlexer

MCP Gateway that multiplexes tool calls across downstream MCP servers with workspace-based routing, auth scoping, and observability.

## Features

- **Unified tool surface** — single MCP endpoint for all downstream servers
- **Workspace routing** — route tool calls based on directory context
- **Auth scoping** — different credentials per workspace/server combination
- **Lazy loading** — downstream servers started on demand, stopped on idle
- **Audit logging** — every call logged with redaction
- **Web UI** — dashboard, audit viewer, config editor, dry-run simulator

## Quick Start

```bash
# Build
make build

# Initialize config
./bin/mcplexer init

# Run as MCP server (for Claude Code, etc.)
./bin/mcplexer serve --mode=stdio

# Run with web UI
./bin/mcplexer serve --mode=http
```

## Configuration

Copy `.env.example` to `.env` and edit. Create `mcplexer.yaml` for workspace/routing config.
