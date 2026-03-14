package gateway

import (
	"context"
	"encoding/json"
	"sort"
	"testing"
	"time"

	"github.com/revittco/mcplexer/internal/config"
	"github.com/revittco/mcplexer/internal/routing"
	"github.com/revittco/mcplexer/internal/store"
)

// --- Test doubles ---

// mockToolLister implements ToolLister for testing.
type mockToolLister struct {
	tools map[string]json.RawMessage
	err   error

	callCount int
	lastCall  struct {
		serverID    string
		authScopeID string
		toolName    string
		args        json.RawMessage
	}
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

func (m *mockToolLister) Call(_ context.Context, serverID, authScopeID, toolName string, args json.RawMessage) (json.RawMessage, error) {
	m.callCount++
	m.lastCall.serverID = serverID
	m.lastCall.authScopeID = authScopeID
	m.lastCall.toolName = toolName
	m.lastCall.args = args
	return nil, nil
}

// mockStore implements store.Store with minimal stubs for handler tests.
type mockStore struct {
	servers    []store.DownstreamServer
	capUpdates map[string]json.RawMessage
	settings   json.RawMessage
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
func (m *mockStore) CreateWorkspace(_ context.Context, _ *store.Workspace) error { return nil }
func (m *mockStore) GetWorkspace(_ context.Context, _ string) (*store.Workspace, error) {
	return nil, nil
}
func (m *mockStore) GetWorkspaceByName(_ context.Context, _ string) (*store.Workspace, error) {
	return nil, nil
}
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
func (m *mockStore) CreateAuthScope(_ context.Context, _ *store.AuthScope) error { return nil }
func (m *mockStore) GetAuthScope(_ context.Context, _ string) (*store.AuthScope, error) {
	return nil, nil
}
func (m *mockStore) GetAuthScopeByName(_ context.Context, _ string) (*store.AuthScope, error) {
	return nil, nil
}
func (m *mockStore) ListAuthScopes(_ context.Context) ([]store.AuthScope, error)          { return nil, nil }
func (m *mockStore) UpdateAuthScope(_ context.Context, _ *store.AuthScope) error          { return nil }
func (m *mockStore) DeleteAuthScope(_ context.Context, _ string) error                    { return nil }
func (m *mockStore) UpdateAuthScopeTokenData(_ context.Context, _ string, _ []byte) error { return nil }
func (m *mockStore) UpdateAuthScopeEncryptedData(_ context.Context, _ string, _ []byte) error {
	return nil
}

// Stubs — OAuthProviderStore.
func (m *mockStore) CreateOAuthProvider(_ context.Context, _ *store.OAuthProvider) error { return nil }
func (m *mockStore) GetOAuthProvider(_ context.Context, _ string) (*store.OAuthProvider, error) {
	return nil, nil
}
func (m *mockStore) GetOAuthProviderByName(_ context.Context, _ string) (*store.OAuthProvider, error) {
	return nil, nil
}
func (m *mockStore) ListOAuthProviders(_ context.Context) ([]store.OAuthProvider, error) {
	return nil, nil
}
func (m *mockStore) UpdateOAuthProvider(_ context.Context, _ *store.OAuthProvider) error { return nil }
func (m *mockStore) DeleteOAuthProvider(_ context.Context, _ string) error               { return nil }

// Stubs — DownstreamServerStore (remaining).
func (m *mockStore) CreateDownstreamServer(_ context.Context, _ *store.DownstreamServer) error {
	return nil
}
func (m *mockStore) GetDownstreamServer(_ context.Context, id string) (*store.DownstreamServer, error) {
	for i := range m.servers {
		if m.servers[i].ID == id {
			return &m.servers[i], nil
		}
	}
	return nil, nil
}
func (m *mockStore) GetDownstreamServerByName(_ context.Context, _ string) (*store.DownstreamServer, error) {
	return nil, nil
}
func (m *mockStore) UpdateDownstreamServer(_ context.Context, _ *store.DownstreamServer) error {
	return nil
}
func (m *mockStore) DeleteDownstreamServer(_ context.Context, _ string) error { return nil }

// Stubs — RouteRuleStore.
func (m *mockStore) CreateRouteRule(_ context.Context, _ *store.RouteRule) error { return nil }
func (m *mockStore) GetRouteRule(_ context.Context, _ string) (*store.RouteRule, error) {
	return nil, nil
}
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
func (m *mockStore) GetDashboardTimeSeriesBucketed(_ context.Context, _, _ time.Time, _ int) ([]store.TimeSeriesPoint, error) {
	return nil, nil
}
func (m *mockStore) GetToolLeaderboard(_ context.Context, _, _ time.Time, _ int) ([]store.ToolLeaderboardEntry, error) {
	return nil, nil
}
func (m *mockStore) GetServerHealth(_ context.Context, _, _ time.Time) ([]store.ServerHealthEntry, error) {
	return nil, nil
}
func (m *mockStore) GetErrorBreakdown(_ context.Context, _, _ time.Time, _ int) ([]store.ErrorBreakdownEntry, error) {
	return nil, nil
}
func (m *mockStore) GetRouteHitMap(_ context.Context, _, _ time.Time) ([]store.RouteHitEntry, error) {
	return nil, nil
}
func (m *mockStore) GetAuditCacheStats(_ context.Context, _, _ time.Time) (*store.AuditCacheStats, error) {
	return nil, nil
}

// Stubs — ToolApprovalStore.
func (m *mockStore) CreateToolApproval(_ context.Context, _ *store.ToolApproval) error { return nil }
func (m *mockStore) GetToolApproval(_ context.Context, _ string) (*store.ToolApproval, error) {
	return nil, nil
}
func (m *mockStore) ListPendingApprovals(_ context.Context) ([]store.ToolApproval, error) {
	return nil, nil
}
func (m *mockStore) ResolveToolApproval(_ context.Context, _, _, _, _, _ string) error { return nil }
func (m *mockStore) ExpirePendingApprovals(_ context.Context, _ time.Time) (int, error) {
	return 0, nil
}
func (m *mockStore) GetApprovalMetrics(_ context.Context, _, _ time.Time) (*store.ApprovalMetrics, error) {
	return nil, nil
}

// Stubs — SettingsStore.
func (m *mockStore) GetSettings(_ context.Context) (json.RawMessage, error) {
	if len(m.settings) > 0 {
		return m.settings, nil
	}
	return json.RawMessage("{}"), nil
}
func (m *mockStore) UpdateSettings(_ context.Context, _ json.RawMessage) error { return nil }

// Stubs — Store top-level.
func (m *mockStore) Tx(_ context.Context, _ func(store.Store) error) error { return nil }
func (m *mockStore) Ping(_ context.Context) error                          { return nil }
func (m *mockStore) Close() error                                          { return nil }

// --- Helpers ---

func toolsJSON(tools ...Tool) json.RawMessage {
	data, _ := json.Marshal(map[string]any{"tools": tools})
	return data
}

func newTestHandler(lister ToolLister, servers []store.DownstreamServer) (*handler, *mockStore) {
	// Always include the mcpx-builtin virtual server for built-in tool routing.
	allServers := append(servers, store.DownstreamServer{
		ID: "mcpx-builtin", Name: "MCPlexer Built-in Tools",
		Transport: "internal", ToolNamespace: "mcpx", Discovery: "static",
	})
	ms := &mockStore{
		servers:    allServers,
		capUpdates: make(map[string]json.RawMessage),
		workspaces: []mockWorkspace{{id: "ws-global", rootPath: "/"}},
		routeRules: map[string][]store.RouteRule{
			"ws-global": {
				{
					ID: "builtin-allow", WorkspaceID: "ws-global",
					Priority: 100, PathGlob: "**", Policy: "allow",
					ToolMatch:          json.RawMessage(`["mcpx__*"]`),
					DownstreamServerID: "mcpx-builtin",
				},
				{
					ID: "allow-all", WorkspaceID: "ws-global",
					Priority: 1, PathGlob: "**", Policy: "allow",
					ToolMatch: json.RawMessage(`["*"]`),
				},
			},
		},
	}
	engine := routing.NewEngine(ms)
	h := newHandler(ms, engine, lister, nil, TransportSocket, nil, nil, nil, nil)
	// Bind session to the global workspace so tool filtering passes.
	h.sessions.clientPath = "/test"
	h.sessions.wsChain = []routing.WorkspaceAncestor{{ID: "ws-global", RootPath: "/"}}
	return h, ms
}

func enableCodeMode(h *handler, ms *mockStore) {
	ms.settings = json.RawMessage(`{"code_mode_enabled":true}`)
	h.settingsSvc = config.NewSettingsService(ms)
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

func TestToolExtrasRoundTrip(t *testing.T) {
	// Downstream tool with annotations, title, outputSchema — extras must survive.
	raw := json.RawMessage(`{
		"name": "read_file",
		"description": "Read a file",
		"inputSchema": {"type":"object"},
		"annotations": {"readOnlyHint": true, "openWorldHint": false},
		"title": "Read File",
		"outputSchema": {"type":"object","properties":{"content":{"type":"string"}}}
	}`)

	var tool Tool
	if err := json.Unmarshal(raw, &tool); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if tool.Name != "read_file" {
		t.Errorf("name = %q, want %q", tool.Name, "read_file")
	}
	if tool.Description != "Read a file" {
		t.Errorf("description = %q, want %q", tool.Description, "Read a file")
	}
	if tool.Extras == nil {
		t.Fatal("extras is nil, expected annotations/title/outputSchema")
	}
	for _, key := range []string{"annotations", "title", "outputSchema"} {
		if _, ok := tool.Extras[key]; !ok {
			t.Errorf("extras missing key %q", key)
		}
	}

	// Re-marshal and verify extras appear in the output.
	out, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var flat map[string]json.RawMessage
	if err := json.Unmarshal(out, &flat); err != nil {
		t.Fatalf("unmarshal flat: %v", err)
	}
	for _, key := range []string{"name", "description", "inputSchema", "annotations", "title", "outputSchema"} {
		if _, ok := flat[key]; !ok {
			t.Errorf("marshalled output missing key %q", key)
		}
	}
}

func TestExtractNamespacedToolsPreservesExtras(t *testing.T) {
	// Simulate downstream tools/list response with extras.
	raw := json.RawMessage(`{"tools":[{
		"name": "create_issue",
		"description": "Create an issue",
		"inputSchema": {"type":"object"},
		"annotations": {"readOnlyHint": false}
	}]}`)

	tools, err := extractNamespacedTools("github", raw)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("got %d tools, want 1", len(tools))
	}
	if tools[0].Name != "github__create_issue" {
		t.Errorf("name = %q, want %q", tools[0].Name, "github__create_issue")
	}
	if tools[0].Extras == nil {
		t.Fatal("extras nil after extractNamespacedTools")
	}
	if _, ok := tools[0].Extras["annotations"]; !ok {
		t.Error("annotations not preserved through extractNamespacedTools")
	}
}

func TestHandleToolsList_ExtrasPassthrough(t *testing.T) {
	servers := []store.DownstreamServer{
		{ID: "gh-server", ToolNamespace: "github", Discovery: "static"},
	}
	lister := &mockToolLister{
		tools: map[string]json.RawMessage{
			"gh-server": json.RawMessage(`{"tools":[{
				"name": "create_issue",
				"description": "Create issue",
				"inputSchema": {"type":"object"},
				"annotations": {"readOnlyHint": false},
				"title": "Create Issue"
			}]}`),
		},
	}

	// Disable slim tools to avoid minification stripping extras.
	t.Setenv("MCPLEXER_SLIM_TOOLS", "false")

	h, _ := newTestHandler(lister, servers)
	result, rpcErr := h.handleToolsList(context.Background())
	if rpcErr != nil {
		t.Fatalf("unexpected error: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}

	// Parse the result and check that extras are present.
	var parsed struct {
		Tools []json.RawMessage `json:"tools"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(parsed.Tools) != 1 {
		t.Fatalf("got %d tools, want 1", len(parsed.Tools))
	}

	var flat map[string]json.RawMessage
	if err := json.Unmarshal(parsed.Tools[0], &flat); err != nil {
		t.Fatalf("unmarshal tool: %v", err)
	}
	if _, ok := flat["annotations"]; !ok {
		t.Error("annotations missing from tools/list output")
	}
	if _, ok := flat["title"]; !ok {
		t.Error("title missing from tools/list output")
	}
}

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
	want := []string{"github__create_issue", "mcpx__load_tools", "mcpx__search_tools", "mcpx__unload_tools"}
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
		if name == "mcpx__search_tools" {
			t.Error("mcpx__search_tools should not appear when no dynamic servers exist")
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
	result, rpcErr := h.handleSearchTools(context.Background(), "search", "")
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
	result, rpcErr := h.handleSearchTools(context.Background(), "anything", "")
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
		Name:      "mcpx__search_tools",
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

func TestHandleToolsList_DynamicCacheNotLeaked(t *testing.T) {
	servers := []store.DownstreamServer{
		{
			ID: "dyn-server", ToolNamespace: "dynns", Discovery: "dynamic",
			CapabilitiesCache: toolsJSON(
				Tool{Name: "hidden_tool", Description: "Should not appear"},
				Tool{Name: "secret_tool", Description: "Also hidden"},
			),
		},
	}
	lister := &mockToolLister{
		tools: map[string]json.RawMessage{},
	}

	h, _ := newTestHandler(lister, servers)
	result, rpcErr := h.handleToolsList(context.Background())
	if rpcErr != nil {
		t.Fatalf("unexpected error: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}

	names := toolNames(result)
	// Should only have built-in tools, NOT the cached dynamic tools.
	for _, name := range names {
		if name == "dynns__hidden_tool" || name == "dynns__secret_tool" {
			t.Errorf("dynamic cached tool %q leaked into tools/list", name)
		}
	}
	want := []string{"mcpx__load_tools", "mcpx__search_tools", "mcpx__unload_tools"}
	if len(names) != len(want) {
		t.Fatalf("got %v, want %v", names, want)
	}
}

func TestHandleToolsList_ExplicitlyLoadedToolsAppear(t *testing.T) {
	servers := []store.DownstreamServer{
		{ID: "dyn-server", ToolNamespace: "dynns", Discovery: "dynamic"},
	}
	lister := &mockToolLister{
		tools: map[string]json.RawMessage{},
	}

	h, _ := newTestHandler(lister, servers)

	// Simulate loading tools via load_tools.
	h.sessions.loadTools([]Tool{
		{Name: "dynns__loaded_tool", Description: "Explicitly loaded"},
	})

	result, rpcErr := h.handleToolsList(context.Background())
	if rpcErr != nil {
		t.Fatalf("unexpected error: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}

	names := toolNames(result)
	found := false
	for _, name := range names {
		if name == "dynns__loaded_tool" {
			found = true
		}
	}
	if !found {
		t.Errorf("explicitly loaded tool not found in tools/list, got: %v", names)
	}
}

func TestHandleToolsList_CodeModeOnlyExposesCodeTools(t *testing.T) {
	servers := []store.DownstreamServer{
		{ID: "gh-server", ToolNamespace: "github", Discovery: "static"},
		{ID: "dyn-server", ToolNamespace: "linear", Discovery: "dynamic"},
	}
	lister := &mockToolLister{
		tools: map[string]json.RawMessage{
			"gh-server":  toolsJSON(Tool{Name: "create_issue", Description: "Create issue"}),
			"dyn-server": toolsJSON(Tool{Name: "list_issues", Description: "List issues"}),
		},
	}

	h, ms := newTestHandler(lister, servers)
	enableCodeMode(h, ms)

	result, rpcErr := h.handleToolsList(context.Background())
	if rpcErr != nil {
		t.Fatalf("unexpected error: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}

	names := toolNames(result)
	want := []string{"mcpx__execute_code", "mcpx__get_code_api"}
	if len(names) != len(want) {
		t.Fatalf("got %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("tool[%d] = %q, want %q", i, names[i], want[i])
		}
	}
}

func TestHandleToolsCall_CodeModeBlocksDirectDownstreamCalls(t *testing.T) {
	servers := []store.DownstreamServer{
		{ID: "gh-server", ToolNamespace: "github", Discovery: "static"},
	}
	lister := &mockToolLister{
		tools: map[string]json.RawMessage{
			"gh-server": toolsJSON(Tool{Name: "create_issue", Description: "Create issue"}),
		},
	}

	h, ms := newTestHandler(lister, servers)
	enableCodeMode(h, ms)

	params, _ := json.Marshal(CallToolRequest{
		Name:      "github__create_issue",
		Arguments: json.RawMessage(`{"title":"bug"}`),
	})
	result, rpcErr := h.handleToolsCall(context.Background(), params)
	if rpcErr != nil {
		t.Fatalf("unexpected RPC error: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}
	if lister.callCount != 0 {
		t.Fatalf("downstream tool should not be called directly in code mode, got %d calls", lister.callCount)
	}

	var tr CallToolResult
	if err := json.Unmarshal(result, &tr); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if !tr.IsError || len(tr.Content) == 0 || !contains(tr.Content[0].Text, "mcpx__execute_code") {
		t.Fatalf("expected direct-call block message, got: %+v", tr)
	}
}

func TestHandleToolsCall_CodeModeBlocksDirectBuiltinCalls(t *testing.T) {
	servers := []store.DownstreamServer{
		{ID: "dyn-server", ToolNamespace: "dynns", Discovery: "dynamic"},
	}
	lister := &mockToolLister{
		tools: map[string]json.RawMessage{
			"dyn-server": toolsJSON(Tool{Name: "search_stuff", Description: "Search stuff"}),
		},
	}

	h, ms := newTestHandler(lister, servers)
	enableCodeMode(h, ms)

	params, _ := json.Marshal(CallToolRequest{
		Name:      "mcpx__search_tools",
		Arguments: json.RawMessage(`{"query":"search"}`),
	})
	result, rpcErr := h.handleToolsCall(context.Background(), params)
	if rpcErr != nil {
		t.Fatalf("unexpected RPC error: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}

	var tr CallToolResult
	if err := json.Unmarshal(result, &tr); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if !tr.IsError || len(tr.Content) == 0 || !contains(tr.Content[0].Text, "mcpx__execute_code") {
		t.Fatalf("expected direct-call block message, got: %+v", tr)
	}
}

func TestHandleToolsCall_CodeModeExecuteCodeCanCallDownstreamTools(t *testing.T) {
	lister := &mockToolLister{
		tools: map[string]json.RawMessage{
			"gh-server": toolsJSON(Tool{
				Name:        "create_issue",
				Description: "Create issue",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"title":{"type":"string"}}}`),
			}),
		},
	}

	ms := &mockStore{
		servers: []store.DownstreamServer{
			{ID: "gh-server", ToolNamespace: "github", Discovery: "static"},
			{
				ID: "mcpx-builtin", Name: "MCPlexer Built-in Tools",
				Transport: "internal", ToolNamespace: "mcpx", Discovery: "static",
			},
		},
		capUpdates: make(map[string]json.RawMessage),
		settings:   json.RawMessage(`{"code_mode_enabled":true}`),
		workspaces: []mockWorkspace{{id: "ws-global", rootPath: "/"}},
		routeRules: map[string][]store.RouteRule{
			"ws-global": {
				{
					ID: "builtin-allow", WorkspaceID: "ws-global",
					Priority: 100, PathGlob: "**", Policy: "allow",
					ToolMatch:          json.RawMessage(`["mcpx__*"]`),
					DownstreamServerID: "mcpx-builtin",
				},
				{
					ID: "allow-gh", WorkspaceID: "ws-global",
					Priority: 10, PathGlob: "**", Policy: "allow",
					ToolMatch:          json.RawMessage(`["github__*"]`),
					DownstreamServerID: "gh-server",
				},
			},
		},
	}
	h := newHandler(
		ms,
		routing.NewEngine(ms),
		lister,
		nil,
		TransportSocket,
		nil,
		config.NewSettingsService(ms),
		nil,
		nil,
	)
	h.sessions.clientPath = "/test"
	h.sessions.wsChain = []routing.WorkspaceAncestor{{ID: "ws-global", RootPath: "/"}}

	params, _ := json.Marshal(CallToolRequest{
		Name: "mcpx__execute_code",
		Arguments: json.RawMessage(`{
			"code": "github.create_issue({ title: 'bug' }); print('ok');"
		}`),
	})
	result, rpcErr := h.handleToolsCall(context.Background(), params)
	if rpcErr != nil {
		t.Fatalf("unexpected RPC error: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}
	if lister.callCount != 1 {
		t.Fatalf("expected 1 downstream call via execute_code, got %d", lister.callCount)
	}
	if lister.lastCall.toolName != "create_issue" {
		t.Fatalf("tool name = %q, want %q", lister.lastCall.toolName, "create_issue")
	}

	var tr CallToolResult
	if err := json.Unmarshal(result, &tr); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(tr.Content) == 0 || !contains(tr.Content[0].Text, "ok") {
		t.Fatalf("expected execute_code output, got: %+v", tr)
	}
}

func TestHandleToolsList_CodexCompatIncludesDynamicTools(t *testing.T) {
	servers := []store.DownstreamServer{
		{ID: "dyn-server", ToolNamespace: "dynns", Discovery: "dynamic"},
	}
	lister := &mockToolLister{
		tools: map[string]json.RawMessage{
			"dyn-server": toolsJSON(Tool{Name: "hidden_tool", Description: "Hidden"}),
		},
	}
	ms := &mockStore{
		servers: append(servers, store.DownstreamServer{
			ID: "mcpx-builtin", Name: "MCPlexer Built-in Tools",
			Transport: "internal", ToolNamespace: "mcpx", Discovery: "static",
		}),
		capUpdates: make(map[string]json.RawMessage),
		settings:   json.RawMessage(`{"codex_dynamic_tool_compat":true}`),
		workspaces: []mockWorkspace{{id: "ws-global", rootPath: "/"}},
		routeRules: map[string][]store.RouteRule{
			"ws-global": {
				{
					ID: "builtin-allow", WorkspaceID: "ws-global",
					Priority: 100, PathGlob: "**", Policy: "allow",
					ToolMatch:          json.RawMessage(`["mcpx__*"]`),
					DownstreamServerID: "mcpx-builtin",
				},
				{
					ID: "allow-all", WorkspaceID: "ws-global",
					Priority: 1, PathGlob: "**", Policy: "allow",
					ToolMatch: json.RawMessage(`["*"]`),
				},
			},
		},
	}

	h := newHandler(
		ms,
		routing.NewEngine(ms),
		lister,
		nil,
		TransportSocket,
		nil,
		config.NewSettingsService(ms),
		nil,
		nil,
	)
	h.sessions.clientPath = "/test"
	h.sessions.wsChain = []routing.WorkspaceAncestor{{ID: "ws-global", RootPath: "/"}}
	h.sessions.session = &store.Session{ClientType: "codex-mcp-client"}

	result, rpcErr := h.handleToolsList(context.Background())
	if rpcErr != nil {
		t.Fatalf("unexpected error: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}

	names := toolNames(result)
	found := false
	for _, name := range names {
		if name == "dynns__hidden_tool" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected dynamic tool to be visible for Codex compat mode, got: %v", names)
	}
}

func TestHandleToolsList_CodexCompatDisabledHidesDynamicTools(t *testing.T) {
	servers := []store.DownstreamServer{
		{ID: "dyn-server", ToolNamespace: "dynns", Discovery: "dynamic"},
	}
	lister := &mockToolLister{
		tools: map[string]json.RawMessage{
			"dyn-server": toolsJSON(Tool{Name: "hidden_tool", Description: "Hidden"}),
		},
	}
	ms := &mockStore{
		servers: append(servers, store.DownstreamServer{
			ID: "mcpx-builtin", Name: "MCPlexer Built-in Tools",
			Transport: "internal", ToolNamespace: "mcpx", Discovery: "static",
		}),
		capUpdates: make(map[string]json.RawMessage),
		settings:   json.RawMessage(`{"codex_dynamic_tool_compat":false}`),
		workspaces: []mockWorkspace{{id: "ws-global", rootPath: "/"}},
		routeRules: map[string][]store.RouteRule{
			"ws-global": {
				{
					ID: "builtin-allow", WorkspaceID: "ws-global",
					Priority: 100, PathGlob: "**", Policy: "allow",
					ToolMatch:          json.RawMessage(`["mcpx__*"]`),
					DownstreamServerID: "mcpx-builtin",
				},
				{
					ID: "allow-all", WorkspaceID: "ws-global",
					Priority: 1, PathGlob: "**", Policy: "allow",
					ToolMatch: json.RawMessage(`["*"]`),
				},
			},
		},
	}

	h := newHandler(
		ms,
		routing.NewEngine(ms),
		lister,
		nil,
		TransportSocket,
		nil,
		config.NewSettingsService(ms),
		nil,
		nil,
	)
	h.sessions.clientPath = "/test"
	h.sessions.wsChain = []routing.WorkspaceAncestor{{ID: "ws-global", RootPath: "/"}}
	h.sessions.session = &store.Session{ClientType: "codex-mcp-client"}

	result, rpcErr := h.handleToolsList(context.Background())
	if rpcErr != nil {
		t.Fatalf("unexpected error: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}

	names := toolNames(result)
	for _, name := range names {
		if name == "dynns__hidden_tool" {
			t.Fatalf("dynamic tool should be hidden when Codex compat is disabled, got: %v", names)
		}
	}
}

func TestHandleToolsList_CodexCompatSkipsNonCodexClients(t *testing.T) {
	servers := []store.DownstreamServer{
		{ID: "dyn-server", ToolNamespace: "dynns", Discovery: "dynamic"},
	}
	lister := &mockToolLister{
		tools: map[string]json.RawMessage{
			"dyn-server": toolsJSON(Tool{Name: "hidden_tool", Description: "Hidden"}),
		},
	}
	ms := &mockStore{
		servers: append(servers, store.DownstreamServer{
			ID: "mcpx-builtin", Name: "MCPlexer Built-in Tools",
			Transport: "internal", ToolNamespace: "mcpx", Discovery: "static",
		}),
		capUpdates: make(map[string]json.RawMessage),
		settings:   json.RawMessage(`{"codex_dynamic_tool_compat":true}`),
		workspaces: []mockWorkspace{{id: "ws-global", rootPath: "/"}},
		routeRules: map[string][]store.RouteRule{
			"ws-global": {
				{
					ID: "builtin-allow", WorkspaceID: "ws-global",
					Priority: 100, PathGlob: "**", Policy: "allow",
					ToolMatch:          json.RawMessage(`["mcpx__*"]`),
					DownstreamServerID: "mcpx-builtin",
				},
				{
					ID: "allow-all", WorkspaceID: "ws-global",
					Priority: 1, PathGlob: "**", Policy: "allow",
					ToolMatch: json.RawMessage(`["*"]`),
				},
			},
		},
	}

	h := newHandler(
		ms,
		routing.NewEngine(ms),
		lister,
		nil,
		TransportSocket,
		nil,
		config.NewSettingsService(ms),
		nil,
		nil,
	)
	h.sessions.clientPath = "/test"
	h.sessions.wsChain = []routing.WorkspaceAncestor{{ID: "ws-global", RootPath: "/"}}
	h.sessions.session = &store.Session{ClientType: "claude-code"}

	result, rpcErr := h.handleToolsList(context.Background())
	if rpcErr != nil {
		t.Fatalf("unexpected error: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}

	names := toolNames(result)
	for _, name := range names {
		if name == "dynns__hidden_tool" {
			t.Fatalf("dynamic tool should remain hidden for non-Codex clients, got: %v", names)
		}
	}
}

func TestHandleToolsCall_GitHubRepoAllowlistBlocksDisallowedRepo(t *testing.T) {
	lister := &mockToolLister{}
	ms := &mockStore{
		servers: []store.DownstreamServer{{ID: "gh", ToolNamespace: "github", Discovery: "static"}},
		workspaces: []mockWorkspace{
			{id: "ws-global", rootPath: "/"},
		},
		routeRules: map[string][]store.RouteRule{
			"ws-global": {
				{
					ID: "allow-gh", WorkspaceID: "ws-global",
					Priority: 1, PathGlob: "**", Policy: "allow",
					ToolMatch:          json.RawMessage(`["github__*"]`),
					DownstreamServerID: "gh",
					AllowedRepos:       json.RawMessage(`["acme/mcplexer"]`),
				},
			},
		},
	}

	h := newHandler(ms, routing.NewEngine(ms), lister, nil, TransportSocket, nil, nil, nil, nil)
	h.sessions.clientPath = "/test"
	h.sessions.wsChain = []routing.WorkspaceAncestor{{ID: "ws-global", RootPath: "/"}}

	params, _ := json.Marshal(CallToolRequest{
		Name:      "github__create_issue",
		Arguments: json.RawMessage(`{"owner":"evilco","repo":"private-repo"}`),
	})
	_, rpcErr := h.handleToolsCall(context.Background(), params)
	if rpcErr == nil {
		t.Fatal("expected route policy error")
	}
	if rpcErr.Code != CodeInvalidParams {
		t.Fatalf("code = %d, want %d", rpcErr.Code, CodeInvalidParams)
	}
	if lister.callCount != 0 {
		t.Fatalf("downstream call count = %d, want 0", lister.callCount)
	}
}

func TestHandleToolsCall_GitHubRepoAllowlistAllowsConfiguredRepo(t *testing.T) {
	lister := &mockToolLister{}
	ms := &mockStore{
		servers: []store.DownstreamServer{{ID: "gh", ToolNamespace: "github", Discovery: "static"}},
		workspaces: []mockWorkspace{
			{id: "ws-global", rootPath: "/"},
		},
		routeRules: map[string][]store.RouteRule{
			"ws-global": {
				{
					ID: "allow-gh", WorkspaceID: "ws-global",
					Priority: 1, PathGlob: "**", Policy: "allow",
					ToolMatch:          json.RawMessage(`["github__*"]`),
					DownstreamServerID: "gh",
					AllowedRepos:       json.RawMessage(`["acme/mcplexer"]`),
				},
			},
		},
	}

	h := newHandler(ms, routing.NewEngine(ms), lister, nil, TransportSocket, nil, nil, nil, nil)
	h.sessions.clientPath = "/test"
	h.sessions.wsChain = []routing.WorkspaceAncestor{{ID: "ws-global", RootPath: "/"}}

	params, _ := json.Marshal(CallToolRequest{
		Name:      "github__create_issue",
		Arguments: json.RawMessage(`{"owner":"acme","repo":"mcplexer"}`),
	})
	_, rpcErr := h.handleToolsCall(context.Background(), params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}
	if lister.callCount != 1 {
		t.Fatalf("downstream call count = %d, want 1", lister.callCount)
	}
}

func TestExtractAndRemoveCacheBust(t *testing.T) {
	tests := []struct {
		name     string
		input    json.RawMessage
		wantBust bool
		wantArgs string
	}{
		{"nil args", nil, false, ""},
		{"empty args", json.RawMessage(`{}`), false, "{}"},
		{"no _cache_bust", json.RawMessage(`{"id":"1"}`), false, `{"id":"1"}`},
		{"_cache_bust true", json.RawMessage(`{"id":"1","_cache_bust":true}`), true, `{"id":"1"}`},
		{"_cache_bust false", json.RawMessage(`{"id":"1","_cache_bust":false}`), false, `{"id":"1","_cache_bust":false}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := tt.input
			got := extractAndRemoveCacheBust(&args)
			if got != tt.wantBust {
				t.Errorf("bust = %v, want %v", got, tt.wantBust)
			}
			if tt.wantArgs != "" && string(args) != tt.wantArgs {
				t.Errorf("args = %s, want %s", args, tt.wantArgs)
			}
		})
	}
}

func TestInjectCacheMeta(t *testing.T) {
	// Cache miss: should inject _meta.cache.cached=false.
	result := json.RawMessage(`{"content":[{"type":"text","text":"hello"}]}`)
	got := injectCacheMeta(result, false, 0)
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(got, &envelope); err != nil {
		t.Fatal(err)
	}
	var meta map[string]json.RawMessage
	if err := json.Unmarshal(envelope["_meta"], &meta); err != nil {
		t.Fatal(err)
	}
	var cacheMeta map[string]any
	if err := json.Unmarshal(meta["cache"], &cacheMeta); err != nil {
		t.Fatal(err)
	}
	if cacheMeta["cached"] != false {
		t.Errorf("cached = %v, want false", cacheMeta["cached"])
	}

	// Cache hit: should include age_seconds.
	got2 := injectCacheMeta(result, true, 45*time.Second)
	json.Unmarshal(got2, &envelope)           //nolint:errcheck
	json.Unmarshal(envelope["_meta"], &meta)  //nolint:errcheck
	json.Unmarshal(meta["cache"], &cacheMeta) //nolint:errcheck
	if cacheMeta["cached"] != true {
		t.Errorf("cached = %v, want true", cacheMeta["cached"])
	}
	if cacheMeta["age_seconds"] != float64(45) {
		t.Errorf("age_seconds = %v, want 45", cacheMeta["age_seconds"])
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
