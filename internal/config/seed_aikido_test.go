package config

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/revittco/mcplexer/internal/store"
	"github.com/revittco/mcplexer/internal/store/sqlite"
)

func TestSeedDefaultAuthScopes_EnsuresAikidoScopeWhenScopesExist(t *testing.T) {
	ctx := context.Background()
	db := newSeedTestDB(t, ctx)

	if err := db.CreateAuthScope(ctx, &store.AuthScope{
		ID:     "custom-scope",
		Name:   "Custom Scope",
		Type:   "env",
		Source: "api",
	}); err != nil {
		t.Fatalf("create custom auth scope: %v", err)
	}

	if err := SeedDefaultAuthScopes(ctx, db); err != nil {
		t.Fatalf("seed default auth scopes: %v", err)
	}

	scope, err := db.GetAuthScope(ctx, aikidoAuthScopeID)
	if err != nil {
		t.Fatalf("expected aikido auth scope to exist: %v", err)
	}
	if scope.Type != "env" {
		t.Fatalf("aikido auth scope type = %q, want env", scope.Type)
	}
}

func TestSeedDefaultDownstreamServers_EnsuresAikidoServerWhenServersExist(t *testing.T) {
	ctx := context.Background()
	db := newSeedTestDB(t, ctx)

	if err := db.CreateDownstreamServer(ctx, &store.DownstreamServer{
		ID:            "custom-server",
		Name:          "Custom Server",
		Transport:     "stdio",
		Command:       "custom-mcp",
		ToolNamespace: "custom",
		Discovery:     "dynamic",
		Source:        "api",
	}); err != nil {
		t.Fatalf("create custom downstream server: %v", err)
	}

	if err := SeedDefaultDownstreamServers(ctx, db); err != nil {
		t.Fatalf("seed default downstream servers: %v", err)
	}

	aikido, err := db.GetDownstreamServer(ctx, aikidoServerID)
	if err != nil {
		t.Fatalf("expected aikido downstream server to exist: %v", err)
	}
	if aikido.Command != "aikido-mcp" {
		t.Fatalf("aikido command = %q, want aikido-mcp", aikido.Command)
	}
	if !aikido.Disabled {
		t.Fatalf("aikido disabled = %v, want true", aikido.Disabled)
	}
}

func TestSeedDefaultRouteRules_EnsuresAikidoRulesWhenRoutesExist(t *testing.T) {
	ctx := context.Background()
	db := newSeedTestDB(t, ctx)

	if err := db.CreateWorkspace(ctx, &store.Workspace{
		ID:            "global",
		Name:          "Global",
		RootPath:      "/",
		DefaultPolicy: "deny",
		Source:        "default",
	}); err != nil {
		t.Fatalf("create global workspace: %v", err)
	}

	if err := db.CreateDownstreamServer(ctx, &store.DownstreamServer{
		ID:            "mcpx-builtin",
		Name:          "MCPlexer Built-in Tools",
		Transport:     "internal",
		ToolNamespace: "mcpx",
		Discovery:     "static",
		Source:        "default",
	}); err != nil {
		t.Fatalf("create builtin server: %v", err)
	}

	if err := db.CreateDownstreamServer(ctx, &store.DownstreamServer{
		ID:            aikidoServerID,
		Name:          "Aikido",
		Transport:     "stdio",
		Command:       "aikido-mcp",
		ToolNamespace: "aikido",
		Discovery:     "dynamic",
		Source:        "default",
	}); err != nil {
		t.Fatalf("create aikido server: %v", err)
	}

	if err := db.CreateAuthScope(ctx, &store.AuthScope{
		ID:     aikidoAuthScopeID,
		Name:   "Aikido Client Credentials",
		Type:   "env",
		Source: "default",
	}); err != nil {
		t.Fatalf("create aikido auth scope: %v", err)
	}

	if err := db.CreateRouteRule(ctx, &store.RouteRule{
		ID:                 "custom-route",
		Name:               "Custom Route",
		Priority:           10,
		WorkspaceID:        "global",
		PathGlob:           "**",
		ToolMatch:          json.RawMessage(`["custom__*"]`),
		DownstreamServerID: aikidoServerID,
		AuthScopeID:        aikidoAuthScopeID,
		Policy:             "allow",
		Source:             "api",
	}); err != nil {
		t.Fatalf("create custom route: %v", err)
	}

	if err := SeedDefaultRouteRules(ctx, db); err != nil {
		t.Fatalf("seed default route rules: %v", err)
	}

	read, err := db.GetRouteRule(ctx, aikidoReadRouteID)
	if err != nil {
		t.Fatalf("expected aikido read route to exist: %v", err)
	}
	if string(read.ToolMatch) != `["aikido__read_*"]` {
		t.Fatalf("aikido read tool_match = %s, want [\"aikido__read_*\"]", string(read.ToolMatch))
	}
	if read.AuthScopeID != aikidoAuthScopeID {
		t.Fatalf("aikido read auth_scope_id = %q, want %q", read.AuthScopeID, aikidoAuthScopeID)
	}

	mutate, err := db.GetRouteRule(ctx, aikidoMutateRouteID)
	if err != nil {
		t.Fatalf("expected aikido mutate route to exist: %v", err)
	}
	if !mutate.RequiresApproval {
		t.Fatalf("aikido mutate requires_approval = %v, want true", mutate.RequiresApproval)
	}
	if mutate.ApprovalTimeout != 300 {
		t.Fatalf("aikido mutate approval_timeout = %d, want 300", mutate.ApprovalTimeout)
	}
}

func newSeedTestDB(t *testing.T, ctx context.Context) *sqlite.DB {
	t.Helper()

	db, err := sqlite.New(ctx, t.TempDir()+"/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
