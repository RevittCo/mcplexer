package gateway

import (
	"context"
	"encoding/json"
	"sort"
	"testing"
	"time"

	"github.com/revittco/mcplexer/internal/routing"
	"github.com/revittco/mcplexer/internal/store"
)

// --- Test doubles ---

// mockToolLister implements ToolLister for testing.
type mockToolLister struct {
	tools map[string]json.RawMessage
	err   error
}

func (m *mockToolLister) ListAllTools(_ context.Context) (map[string]json.RawMessage, error) {
	return m.tools, m.err
}

func (m *mockToolLister) ListToolsForServers(_ context.Context, serverIDs []string) (map[string]json.RawMessage, error) {
	result := make(map[string]json.RawMessage)
	for _, id := range serverIDs {
		if tools, ok := m.tools[id]; ok {
			result[id] = tools
		}
	}
	return result, m.err
}

func (m *mockToolLister) Call(_ context.Context, _, _, _ string, _ json.RawMessage) (json.RawMessage, error) {
	return nil, nil
}

// mockStore implements store.Store with minimal stubs for handler tests.
type mockStore struct {
	servers    []store.DownstreamServer
	capUpdates map[string]json.RawMessage
	workspaces []mockWorkspace
	routeRules map[string][]store.RouteRule // keyed by workspace ID
}

// mockWorkspace is a lightweight workspace definition for tests.
type mockWorkspace struct {
	id       string
	rootPath string
}

func (m *mockStore) ListDownstreamServers(_ context.Context) ([]store.DownstreamServer, error) {
	return m.servers, nil
}

func (m *mockStore) UpdateCapabilitiesCache(_ context.Context, id string, cache json.RawMessage) error {
	if m.capUpdates != nil {
		m.capUpdates[id] = cache
	}
	return nil
}

// Stubs — WorkspaceStore.
func (m *mockStore) CreateWorkspace(_ context.Context, _ *store.Workspace) error             { return nil }
func (m *mockStore) GetWorkspace(_ context.Context, _ string) (*store.Workspace, error)      { return nil, nil }
func (m *mockStore) GetWorkspaceByName(_ context.Context, _ string) (*store.Workspace, error) { return nil, nil }
func (m *mockStore) ListWorkspaces(_ context.Context) ([]store.Workspace, error) {
	out := make([]store.Workspace, len(m.workspaces))
	for i, w := range m.workspaces {
		out[i] = store.Workspace{ID: w.id, RootPath: w.rootPath}
	}
	return out, nil
}
func (m *mockStore) UpdateWorkspace(_ context.Context, _ *store.Workspace) error { return nil }
func (m *mockStore) DeleteWorkspace(_ context.Context, _ string) error           { return nil }

// Stubs — AuthScopeStore.
func (m *mockStore) CreateAuthScope(_ context.Context, _ *store.AuthScope) error               { return nil }
func (m *mockStore) GetAuthScope(_ context.Context, _ string) (*store.AuthScope, error)        { return nil, nil }
func (m *mockStore) GetAuthScopeByName(_ context.Context, _ string) (*store.AuthScope, error)  { return nil, nil }
func (m *mockStore) ListAuthScopes(_ context.Context) ([]store.AuthScope, error)               { return nil, nil }
func (m *mockStore) UpdateAuthScope(_ context.Context, _ *store.AuthScope) error               { return nil }
func (m *mockStore) DeleteAuthScope(_ context.Context, _ string) error                         { return nil }
func (m *mockStore) UpdateAuthScopeTokenData(_ context.Context, _ string, _ []byte) error      { return nil }

// Stubs — OAuthProviderStore.
func (m *mockStore) CreateOAuthProvider(_ context.Context, _ *store.OAuthProvider) error               { return nil }
func (m *mockStore) GetOAuthProvider(_ context.Context, _ string) (*store.OAuthProvider, error)        { return nil, nil }
func (m *mockStore) GetOAuthProviderByName(_ context.Context, _ string) (*store.OAuthProvider, error)  { return nil, nil }
func (m *mockStore) ListOAuthProviders(_ context.Context) ([]store.OAuthProvider, error)               { return nil, nil }
func (m *mockStore) UpdateOAuthProvider(_ context.Context, _ *store.OAuthProvider) error               { return nil }
func (m *mockStore) DeleteOAuthProvider(_ context.Context, _ string) error                             { return nil }

// Stubs — DownstreamServerStore (remaining).
func (m *mockStore) CreateDownstreamServer(_ context.Context, _ *store.DownstreamServer) error { return nil }
func (m *mockStore) GetDownstreamServer(_ context.Context, _ string) (*store.DownstreamServer, error) { return nil, nil }
func (m *mockStore) GetDownstreamServerByName(_ context.Context, _ string) (*store.DownstreamServer, error) { return nil, nil }
func (m *mockStore) UpdateDownstreamServer(_ context.Context, _ *store.DownstreamServer) error { return nil }
func (m *mockStore) DeleteDownstreamServer(_ context.Context, _ string) error                  { return nil }

// Stubs — RouteRuleStore.
func (m *mockStore) CreateRouteRule(_ context.Context, _ *store.RouteRule) error        { return nil }
func (m *mockStore) GetRouteRule(_ context.Context, _ string) (*store.RouteRule, error) { return nil, nil }
func (m *mockStore) ListRouteRules(_ context.Context, wsID string) ([]store.RouteRule, error) {
	if m.routeRules != nil {
		return m.routeRules[wsID], nil
	}
	return nil, nil
}
func (m *mockStore) UpdateRouteRule(_ context.Context, _ *store.RouteRule) error { return nil }
func (m *mockStore) DeleteRouteRule(_ context.Context, _ string) error           { return nil }

// Stubs — SessionStore.
func (m *mockStore) CreateSession(_ context.Context, _ *store.Session) error          { return nil }
func (m *mockStore) GetSession(_ context.Context, _ string) (*store.Session, error)   { return nil, nil }
func (m *mockStore) DisconnectSession(_ context.Context, _ string) error              { return nil }
func (m *mockStore) ListActiveSessions(_ context.Context) ([]store.Session, error)    { return nil, nil }
func (m *mockStore) CleanupStaleSessions(_ context.Context, _ time.Time) (int, error) { return 0, nil }

// Stubs — AuditStore.
func (m *mockStore) InsertAuditRecord(_ context.Context, _ *store.AuditRecord) error { return nil }
func (m *mockStore) QueryAuditRecords(_ context.Context, _ store.AuditFilter) ([]store.AuditRecord, int, error) {
	return nil, 0, nil
}
func (m *mockStore) GetAuditStats(_ context.Context, _ string, _, _ time.Time) (*store.AuditStats, error) {
	return nil, nil
}
func (m *mockStore) GetDashboardTimeSeries(_ context.Context, _, _ time.Time) ([]store.TimeSeriesPoint, error) {
	return nil, nil
}

// Stubs — ToolApprovalStore.
func (m *mockStore) CreateToolApproval(_ context.Context, _ *store.ToolApproval) error   { return nil }
func (m *mockStore) GetToolApproval(_ context.Context, _ string) (*store.ToolApproval, error) { return nil, nil }
func (m *mockStore) ListPendingApprovals(_ context.Context) ([]store.ToolApproval, error) { return nil, nil }
func (m *mockStore) ResolveToolApproval(_ context.Context, _, _, _, _, _ string) error    { return nil }
func (m *mockStore) ExpirePendingApprovals(_ context.Context, _ time.Time) (int, error)   { return 0, nil }

// Stubs — Store top-level.
func (m *mockStore) Tx(_ context.Context, _ func(store.Store) error) error { return nil }
func (m *mockStore) Ping(_ context.Context) error                         { return nil }
func (m *mockStore) Close() error                                         { return nil }

// --- Helpers ---

func toolsJSON(tools ...Tool) json.RawMessage {
	data, _ := json.Marshal(map[string]any{"tools": tools})
	return data
}

func newTestHandler(lister ToolLister, servers []store.DownstreamServer) (*handler, *mockStore) {
	ms := &mockStore{
		servers:    servers,
		capUpdates: make(map[string]json.RawMessage),
		workspaces: []mockWorkspace{{id: "ws-global", rootPath: "/"}},
		routeRules: map[string][]store.RouteRule{
			"ws-global": {
				{
					ID: "allow-all", WorkspaceID: "ws-global",
					Priority: 1, PathGlob: "**", Policy: "allow",
					ToolMatch: json.RawMessage(`["*"]`),
				},
			},
		},
	}
	engine := routing.NewEngine(ms)
	h := newHandler(ms, engine, lister, nil, TransportSocket, nil)
	// Bind session to the global workspace so tool filtering passes.
	h.sessions.clientPath = "/test"
	h.sessions.wsChain = []routing.WorkspaceAncestor{{ID: "ws-global", RootPath: "/"}}
	return h, ms
}

func toolNames(result json.RawMessage) []string {
	var parsed struct {
		Tools []Tool `json:"tools"`
	}
	json.Unmarshal(result, &parsed) //nolint:errcheck
	names := make([]string, len(parsed.Tools))
	for i, t := range parsed.Tools {
		names[i] = t.Name
	}
	sort.Strings(names)
	return names
}

// --- Tests ---

func TestExtractNamespacedTools(t *testing.T) {
	schema := json.RawMessage(`{"type":"object"}`)

	tests := []struct {
		name      string
		namespace string
		input     json.RawMessage
		wantNames []string
		wantErr   bool
	}{
		{"nil input", "ns", nil, nil, false},
		{"empty object", "ns", json.RawMessage(`{}`), nil, false},
		{"invalid json", "ns", json.RawMessage(`not json`), nil, true},
		{
			"single tool", "github",
			toolsJSON(Tool{Name: "create_issue", Description: "Create", InputSchema: schema}),
			[]string{"github__create_issue"}, false,
		},
		{
			"multiple tools", "slack",
			toolsJSON(
				Tool{Name: "post_message", Description: "Post"},
				Tool{Name: "list_channels", Description: "List"},
			),
			[]string{"slack__list_channels", "slack__post_message"}, false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractNamespacedTools(tt.namespace, tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantNames == nil {
				if len(got) != 0 {
					t.Fatalf("got %d tools, want 0", len(got))
				}
				return
			}
			names := make([]string, len(got))
			for i, tool := range got {
				names[i] = tool.Name
			}
			sort.Strings(names)
			sort.Strings(tt.wantNames)
			if len(names) != len(tt.wantNames) {
				t.Fatalf("got %v, want %v", names, tt.wantNames)
			}
			for i := range names {
				if names[i] != tt.wantNames[i] {
					t.Errorf("name[%d] = %q, want %q", i, names[i], tt.wantNames[i])
				}
			}
		})
	}
}

func TestHandleToolsList_AggregatesFromLiveServers(t *testing.T) {
	servers := []store.DownstreamServer{
		{ID: "gh-server", ToolNamespace: "github", Discovery: "static"},
		{ID: "slack-server", ToolNamespace: "slack", Discovery: "static"},
	}
	lister := &mockToolLister{
		tools: map[string]json.RawMessage{
			"gh-server":    toolsJSON(Tool{Name: "create_issue", Description: "Create issue"}),
			"slack-server": toolsJSON(Tool{Name: "post_message", Description: "Post message"}),
		},
	}

	h, ms := newTestHandler(lister, servers)
	result, rpcErr := h.handleToolsList(context.Background())
	if rpcErr != nil {
		t.Fatalf("unexpected error: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}

	names := toolNames(result)
	want := []string{"github__create_issue", "slack__post_message"}
	if len(names) != len(want) {
		t.Fatalf("got %v, want %v", names, want)
	}
	for i := range names {
		if names[i] != want[i] {
			t.Errorf("tool[%d] = %q, want %q", i, names[i], want[i])
		}
	}

	// Verify capabilities cache was updated for both servers.
	for _, id := range []string{"gh-server", "slack-server"} {
		if _, ok := ms.capUpdates[id]; !ok {
			t.Errorf("capabilities cache not updated for %s", id)
		}
	}
}

func TestHandleToolsList_PartialFailure(t *testing.T) {
	servers := []store.DownstreamServer{
		{ID: "gh-server", ToolNamespace: "github", Discovery: "static"},
		{ID: "broken-server", ToolNamespace: "broken", Discovery: "static"},
	}
	lister := &mockToolLister{
		tools: map[string]json.RawMessage{
			"gh-server": toolsJSON(Tool{Name: "create_issue", Description: "Create issue"}),
		},
	}

	h, _ := newTestHandler(lister, servers)
	result, rpcErr := h.handleToolsList(context.Background())
	if rpcErr != nil {
		t.Fatalf("unexpected error: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}

	names := toolNames(result)
	if len(names) != 1 {
		t.Fatalf("got %d tools, want 1", len(names))
	}
	if names[0] != "github__create_issue" {
		t.Errorf("tool = %q, want %q", names[0], "github__create_issue")
	}
}

func TestHandleToolsList_EmptyResult(t *testing.T) {
	lister := &mockToolLister{
		tools: map[string]json.RawMessage{},
	}

	h, _ := newTestHandler(lister, nil)
	result, rpcErr := h.handleToolsList(context.Background())
	if rpcErr != nil {
		t.Fatalf("unexpected error: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}

	names := toolNames(result)
	if len(names) != 0 {
		t.Fatalf("got %d tools, want 0", len(names))
	}
}

func TestHandleToolsList_FiltersStaticOnly(t *testing.T) {
	servers := []store.DownstreamServer{
		{ID: "gh-server", ToolNamespace: "github", Discovery: "static"},
		{ID: "dyn-server", ToolNamespace: "dynns", Discovery: "dynamic"},
	}
	lister := &mockToolLister{
		tools: map[string]json.RawMessage{
			"gh-server":  toolsJSON(Tool{Name: "create_issue", Description: "Create issue"}),
			"dyn-server": toolsJSON(Tool{Name: "hidden_tool", Description: "Hidden"}),
		},
	}

	h, _ := newTestHandler(lister, servers)
	result, rpcErr := h.handleToolsList(context.Background())
	if rpcErr != nil {
		t.Fatalf("unexpected error: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}

	names := toolNames(result)
	// Should have the static tool + the built-in search tool.
	want := []string{"github__create_issue", "mcplexer__search_tools"}
	if len(names) != len(want) {
		t.Fatalf("got %v, want %v", names, want)
	}
	for i := range names {
		if names[i] != want[i] {
			t.Errorf("tool[%d] = %q, want %q", i, names[i], want[i])
		}
	}
}

func TestHandleToolsList_NoSearchToolWhenNoDynamic(t *testing.T) {
	servers := []store.DownstreamServer{
		{ID: "gh-server", ToolNamespace: "github", Discovery: "static"},
		{ID: "slack-server", ToolNamespace: "slack", Discovery: "static"},
	}
	lister := &mockToolLister{
		tools: map[string]json.RawMessage{
			"gh-server":    toolsJSON(Tool{Name: "create_issue", Description: "Create issue"}),
			"slack-server": toolsJSON(Tool{Name: "post_message", Description: "Post message"}),
		},
	}

	h, _ := newTestHandler(lister, servers)
	result, rpcErr := h.handleToolsList(context.Background())
	if rpcErr != nil {
		t.Fatalf("unexpected error: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}

	names := toolNames(result)
	for _, name := range names {
		if name == "mcplexer__search_tools" {
			t.Error("mcplexer__search_tools should not appear when no dynamic servers exist")
		}
	}
}

func TestHandleSearchTools_FindsMatchingTools(t *testing.T) {
	servers := []store.DownstreamServer{
		{ID: "static-server", ToolNamespace: "github", Discovery: "static"},
		{ID: "dyn-server", ToolNamespace: "dynns", Discovery: "dynamic"},
	}
	lister := &mockToolLister{
		tools: map[string]json.RawMessage{
			"static-server": toolsJSON(Tool{Name: "create_issue", Description: "Create issue"}),
			"dyn-server": toolsJSON(
				Tool{Name: "search_code", Description: "Search code in repo"},
				Tool{Name: "list_files", Description: "List files in directory"},
			),
		},
	}

	h, _ := newTestHandler(lister, servers)
	result, rpcErr := h.handleSearchTools(context.Background(), "search")
	if rpcErr != nil {
		t.Fatalf("unexpected error: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}

	var tr CallToolResult
	if err := json.Unmarshal(result, &tr); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(tr.Content) == 0 {
		t.Fatal("expected content in result")
	}
	text := tr.Content[0].Text
	if !contains(text, "dynns__search_code") {
		t.Errorf("expected matching tool in result, got: %s", text)
	}
	if contains(text, "dynns__list_files") {
		t.Errorf("non-matching tool should not appear, got: %s", text)
	}
}

func TestHandleSearchTools_NoDynamicServers(t *testing.T) {
	servers := []store.DownstreamServer{
		{ID: "gh-server", ToolNamespace: "github", Discovery: "static"},
	}
	lister := &mockToolLister{
		tools: map[string]json.RawMessage{
			"gh-server": toolsJSON(Tool{Name: "create_issue", Description: "Create issue"}),
		},
	}

	h, _ := newTestHandler(lister, servers)
	result, rpcErr := h.handleSearchTools(context.Background(), "anything")
	if rpcErr != nil {
		t.Fatalf("unexpected error: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}

	var tr CallToolResult
	if err := json.Unmarshal(result, &tr); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(tr.Content) == 0 || !contains(tr.Content[0].Text, "No dynamic servers") {
		t.Errorf("expected 'No dynamic servers' message, got: %v", tr.Content)
	}
}

func TestHandleToolsCall_InterceptsBuiltin(t *testing.T) {
	servers := []store.DownstreamServer{
		{ID: "dyn-server", ToolNamespace: "dynns", Discovery: "dynamic"},
	}
	lister := &mockToolLister{
		tools: map[string]json.RawMessage{
			"dyn-server": toolsJSON(Tool{Name: "some_tool", Description: "A tool"}),
		},
	}

	h, _ := newTestHandler(lister, servers)
	params, _ := json.Marshal(CallToolRequest{
		Name:      "mcplexer__search_tools",
		Arguments: json.RawMessage(`{"query":"some"}`),
	})
	result, rpcErr := h.handleToolsCall(context.Background(), params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}

	var tr CallToolResult
	if err := json.Unmarshal(result, &tr); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(tr.Content) == 0 {
		t.Fatal("expected content in result")
	}
	if !contains(tr.Content[0].Text, "dynns__some_tool") {
		t.Errorf("expected tool in search result, got: %s", tr.Content[0].Text)
	}
}

// contains is a test helper for substring matching.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > 0 && stringContains(s, substr))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
