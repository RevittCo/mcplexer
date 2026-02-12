package sqlite_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/revitteth/mcplexer/internal/store"
	"github.com/revitteth/mcplexer/internal/store/sqlite"
)

func newTestDB(t *testing.T) *sqlite.DB {
	t.Helper()
	db, err := sqlite.New(context.Background(), t.TempDir()+"/test.db")
	if err != nil {
		t.Fatalf("new test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestPing(t *testing.T) {
	db := newTestDB(t)
	if err := db.Ping(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

func TestWorkspaceCRUD(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	w := &store.Workspace{
		Name:          "test-ws",
		RootPath:      "/tmp/test",
		Tags:          json.RawMessage(`["go","test"]`),
		DefaultPolicy: "allow",
	}

	// Create.
	if err := db.CreateWorkspace(ctx, w); err != nil {
		t.Fatalf("create: %v", err)
	}
	if w.ID == "" {
		t.Fatal("expected ID to be set")
	}

	// Get by ID.
	got, err := db.GetWorkspace(ctx, w.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "test-ws" {
		t.Fatalf("name = %q, want %q", got.Name, "test-ws")
	}

	// Get by name.
	got, err = db.GetWorkspaceByName(ctx, "test-ws")
	if err != nil {
		t.Fatalf("get by name: %v", err)
	}
	if got.ID != w.ID {
		t.Fatalf("id mismatch")
	}

	// List.
	list, err := db.ListWorkspaces(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len = %d, want 1", len(list))
	}

	// Update.
	got.Name = "updated-ws"
	if err := db.UpdateWorkspace(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	got2, _ := db.GetWorkspace(ctx, w.ID)
	if got2.Name != "updated-ws" {
		t.Fatalf("name after update = %q", got2.Name)
	}

	// Delete.
	if err := db.DeleteWorkspace(ctx, w.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err = db.GetWorkspace(ctx, w.ID)
	if err != store.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestWorkspaceDuplicate(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	w := &store.Workspace{Name: "dup", DefaultPolicy: "allow"}
	if err := db.CreateWorkspace(ctx, w); err != nil {
		t.Fatal(err)
	}
	w2 := &store.Workspace{Name: "dup", DefaultPolicy: "deny"}
	if err := db.CreateWorkspace(ctx, w2); err != store.ErrAlreadyExists {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestAuthScopeCRUD(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	a := &store.AuthScope{
		Name:          "gh-token",
		Type:          "env",
		EncryptedData: []byte("encrypted-stuff"),
	}

	if err := db.CreateAuthScope(ctx, a); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := db.GetAuthScope(ctx, a.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(got.EncryptedData) != "encrypted-stuff" {
		t.Fatalf("encrypted data mismatch")
	}

	got, err = db.GetAuthScopeByName(ctx, "gh-token")
	if err != nil {
		t.Fatalf("get by name: %v", err)
	}

	list, err := db.ListAuthScopes(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len = %d", len(list))
	}

	got.Name = "updated-token"
	if err := db.UpdateAuthScope(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}

	if err := db.DeleteAuthScope(ctx, a.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err = db.GetAuthScope(ctx, a.ID)
	if err != store.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDownstreamServerCRUD(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	ds := &store.DownstreamServer{
		Name:           "github-mcp",
		Transport:      "stdio",
		Command:        "npx",
		Args:           json.RawMessage(`["-y","@mcp/server-github"]`),
		ToolNamespace:  "github",
		IdleTimeoutSec: 300,
		MaxInstances:   1,
		RestartPolicy:  "on-failure",
	}

	if err := db.CreateDownstreamServer(ctx, ds); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := db.GetDownstreamServer(ctx, ds.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ToolNamespace != "github" {
		t.Fatalf("namespace = %q", got.ToolNamespace)
	}

	got, err = db.GetDownstreamServerByName(ctx, "github-mcp")
	if err != nil {
		t.Fatalf("get by name: %v", err)
	}

	list, err := db.ListDownstreamServers(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len = %d", len(list))
	}

	got.Name = "github-mcp-v2"
	if err := db.UpdateDownstreamServer(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}

	cache := json.RawMessage(`{"tools":["create_issue"]}`)
	if err := db.UpdateCapabilitiesCache(ctx, ds.ID, cache); err != nil {
		t.Fatalf("update caps: %v", err)
	}
	got2, _ := db.GetDownstreamServer(ctx, ds.ID)
	if string(got2.CapabilitiesCache) != `{"tools":["create_issue"]}` {
		t.Fatalf("caps = %s", got2.CapabilitiesCache)
	}

	if err := db.DeleteDownstreamServer(ctx, ds.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestRouteRuleCRUD(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	// Create prerequisites.
	ws := &store.Workspace{Name: "rule-ws", DefaultPolicy: "allow"}
	if err := db.CreateWorkspace(ctx, ws); err != nil {
		t.Fatal(err)
	}
	ds := &store.DownstreamServer{
		Name: "rule-ds", Transport: "stdio",
		ToolNamespace: "test", RestartPolicy: "on-failure",
	}
	if err := db.CreateDownstreamServer(ctx, ds); err != nil {
		t.Fatal(err)
	}

	r := &store.RouteRule{
		Priority:           100,
		WorkspaceID:        ws.ID,
		PathGlob:           "**",
		ToolMatch:          json.RawMessage(`["github__*"]`),
		DownstreamServerID: ds.ID,
		Policy:             "allow",
		LogLevel:           "info",
	}

	if err := db.CreateRouteRule(ctx, r); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := db.GetRouteRule(ctx, r.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Priority != 100 {
		t.Fatalf("priority = %d", got.Priority)
	}

	list, err := db.ListRouteRules(ctx, ws.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len = %d", len(list))
	}

	got.Priority = 200
	if err := db.UpdateRouteRule(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}

	if err := db.DeleteRouteRule(ctx, r.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestSessionCRUD(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	pid := 1234
	s := &store.Session{
		ClientType: "claude-code",
		ClientPID:  &pid,
		ModelHint:  "opus",
	}

	if err := db.CreateSession(ctx, s); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := db.GetSession(ctx, s.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ClientType != "claude-code" {
		t.Fatalf("type = %q", got.ClientType)
	}
	if got.DisconnectedAt != nil {
		t.Fatal("should not be disconnected yet")
	}

	active, err := db.ListActiveSessions(ctx)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("active len = %d", len(active))
	}

	if err := db.DisconnectSession(ctx, s.ID); err != nil {
		t.Fatalf("disconnect: %v", err)
	}

	got, _ = db.GetSession(ctx, s.ID)
	if got.DisconnectedAt == nil {
		t.Fatal("should be disconnected")
	}

	active, _ = db.ListActiveSessions(ctx)
	if len(active) != 0 {
		t.Fatalf("active after disconnect = %d", len(active))
	}
}

func TestAuditCRUD(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	// Insert a few records.
	for i, name := range []string{"github__create_issue", "slack__post_message", "github__list_prs"} {
		r := &store.AuditRecord{
			Timestamp:  time.Now().UTC().Add(time.Duration(i) * time.Second),
			ToolName:   name,
			Status:     "success",
			LatencyMs:  50 + i*10,
			WorkspaceID: "ws1",
		}
		if err := db.InsertAuditRecord(ctx, r); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	// Query all.
	records, total, err := db.QueryAuditRecords(ctx, store.AuditFilter{Limit: 10})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if total != 3 || len(records) != 3 {
		t.Fatalf("total=%d, len=%d", total, len(records))
	}

	// Query by tool name.
	tool := "github__create_issue"
	records, total, err = db.QueryAuditRecords(ctx, store.AuditFilter{
		ToolName: &tool,
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("query by tool: %v", err)
	}
	if total != 1 {
		t.Fatalf("total by tool = %d", total)
	}

	// Stats.
	stats, err := db.GetAuditStats(ctx, "ws1",
		time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.TotalRequests != 3 {
		t.Fatalf("total requests = %d", stats.TotalRequests)
	}
}

func TestDashboardTimeSeries(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	base := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Insert records across 3 different minutes.
	records := []struct {
		offset   time.Duration
		session  string
		server   string
		status   string
	}{
		{0 * time.Minute, "s1", "srv-a", "success"},
		{0 * time.Minute, "s2", "srv-a", "error"},
		{0 * time.Minute, "s1", "srv-b", "success"},
		{2 * time.Minute, "s3", "srv-a", "success"},
		{4 * time.Minute, "s1", "srv-b", "error"},
		{4 * time.Minute, "s1", "srv-b", "error"},
	}
	for i, rec := range records {
		r := &store.AuditRecord{
			Timestamp:          base.Add(rec.offset).Add(time.Duration(i) * time.Second),
			SessionID:          rec.session,
			DownstreamServerID: rec.server,
			ToolName:           "test__tool",
			Status:             rec.status,
			LatencyMs:          10,
		}
		if err := db.InsertAuditRecord(ctx, r); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	points, err := db.GetDashboardTimeSeries(ctx, base.Add(-1*time.Minute), base.Add(5*time.Minute))
	if err != nil {
		t.Fatalf("get time series: %v", err)
	}

	if len(points) != 3 {
		t.Fatalf("expected 3 buckets, got %d", len(points))
	}

	// Minute 0: 2 sessions (s1, s2), 2 servers (srv-a, srv-b), 3 total, 1 error
	p := points[0]
	if p.Sessions != 2 {
		t.Errorf("bucket 0 sessions = %d, want 2", p.Sessions)
	}
	if p.Servers != 2 {
		t.Errorf("bucket 0 servers = %d, want 2", p.Servers)
	}
	if p.Total != 3 {
		t.Errorf("bucket 0 total = %d, want 3", p.Total)
	}
	if p.Errors != 1 {
		t.Errorf("bucket 0 errors = %d, want 1", p.Errors)
	}

	// Minute 2: 1 session (s3), 1 server (srv-a), 1 total, 0 errors
	p = points[1]
	if p.Sessions != 1 || p.Servers != 1 || p.Total != 1 || p.Errors != 0 {
		t.Errorf("bucket 2: sessions=%d servers=%d total=%d errors=%d",
			p.Sessions, p.Servers, p.Total, p.Errors)
	}

	// Minute 4: 1 session (s1), 1 server (srv-b), 2 total, 2 errors
	p = points[2]
	if p.Sessions != 1 || p.Servers != 1 || p.Total != 2 || p.Errors != 2 {
		t.Errorf("bucket 4: sessions=%d servers=%d total=%d errors=%d",
			p.Sessions, p.Servers, p.Total, p.Errors)
	}
}

func TestTx(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	// Transaction commit.
	err := db.Tx(ctx, func(tx store.Store) error {
		return tx.CreateWorkspace(ctx, &store.Workspace{
			Name: "tx-ws", DefaultPolicy: "allow",
		})
	})
	if err != nil {
		t.Fatalf("tx: %v", err)
	}

	_, err = db.GetWorkspaceByName(ctx, "tx-ws")
	if err != nil {
		t.Fatalf("get after tx: %v", err)
	}
}

func TestNotFound(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	tests := []struct {
		name string
		fn   func() error
	}{
		{"workspace", func() error { _, err := db.GetWorkspace(ctx, "nope"); return err }},
		{"auth_scope", func() error { _, err := db.GetAuthScope(ctx, "nope"); return err }},
		{"downstream", func() error { _, err := db.GetDownstreamServer(ctx, "nope"); return err }},
		{"route_rule", func() error { _, err := db.GetRouteRule(ctx, "nope"); return err }},
		{"session", func() error { _, err := db.GetSession(ctx, "nope"); return err }},
		{"delete_ws", func() error { return db.DeleteWorkspace(ctx, "nope") }},
		{"update_ws", func() error {
			return db.UpdateWorkspace(ctx, &store.Workspace{ID: "nope", Name: "x", DefaultPolicy: "allow"})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.fn(); err != store.ErrNotFound {
				t.Fatalf("expected ErrNotFound, got %v", err)
			}
		})
	}
}
