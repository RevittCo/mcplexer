package routing

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/revitteth/mcplexer/internal/store"
)

// mockRouteStore implements store.Store for routing engine tests.
// Only ListRouteRules and GetDownstreamServer are meaningful; all other methods are stubs.
type mockRouteStore struct {
	rules       map[string][]store.RouteRule
	downstreams map[string]*store.DownstreamServer
}

func (m *mockRouteStore) ListRouteRules(_ context.Context, wsID string) ([]store.RouteRule, error) {
	return m.rules[wsID], nil
}
func (m *mockRouteStore) GetDownstreamServer(_ context.Context, id string) (*store.DownstreamServer, error) {
	if m.downstreams != nil {
		if ds, ok := m.downstreams[id]; ok {
			return ds, nil
		}
	}
	return nil, store.ErrNotFound
}
func (m *mockRouteStore) CreateRouteRule(context.Context, *store.RouteRule) error        { return nil }
func (m *mockRouteStore) GetRouteRule(context.Context, string) (*store.RouteRule, error) { return nil, nil }
func (m *mockRouteStore) UpdateRouteRule(context.Context, *store.RouteRule) error        { return nil }
func (m *mockRouteStore) DeleteRouteRule(context.Context, string) error                  { return nil }
func (m *mockRouteStore) CreateWorkspace(context.Context, *store.Workspace) error        { return nil }
func (m *mockRouteStore) GetWorkspace(context.Context, string) (*store.Workspace, error) { return nil, nil }
func (m *mockRouteStore) GetWorkspaceByName(context.Context, string) (*store.Workspace, error) { return nil, nil }
func (m *mockRouteStore) ListWorkspaces(context.Context) ([]store.Workspace, error)            { return nil, nil }
func (m *mockRouteStore) UpdateWorkspace(context.Context, *store.Workspace) error              { return nil }
func (m *mockRouteStore) DeleteWorkspace(context.Context, string) error                        { return nil }
func (m *mockRouteStore) CreateAuthScope(context.Context, *store.AuthScope) error              { return nil }
func (m *mockRouteStore) GetAuthScope(context.Context, string) (*store.AuthScope, error)       { return nil, nil }
func (m *mockRouteStore) GetAuthScopeByName(context.Context, string) (*store.AuthScope, error) { return nil, nil }
func (m *mockRouteStore) ListAuthScopes(context.Context) ([]store.AuthScope, error)            { return nil, nil }
func (m *mockRouteStore) UpdateAuthScope(context.Context, *store.AuthScope) error              { return nil }
func (m *mockRouteStore) DeleteAuthScope(context.Context, string) error                        { return nil }
func (m *mockRouteStore) UpdateAuthScopeTokenData(context.Context, string, []byte) error       { return nil }
func (m *mockRouteStore) CreateOAuthProvider(context.Context, *store.OAuthProvider) error      { return nil }
func (m *mockRouteStore) GetOAuthProvider(context.Context, string) (*store.OAuthProvider, error) { return nil, nil }
func (m *mockRouteStore) GetOAuthProviderByName(context.Context, string) (*store.OAuthProvider, error) { return nil, nil }
func (m *mockRouteStore) ListOAuthProviders(context.Context) ([]store.OAuthProvider, error)            { return nil, nil }
func (m *mockRouteStore) UpdateOAuthProvider(context.Context, *store.OAuthProvider) error              { return nil }
func (m *mockRouteStore) DeleteOAuthProvider(context.Context, string) error                            { return nil }
func (m *mockRouteStore) CreateDownstreamServer(context.Context, *store.DownstreamServer) error        { return nil }
func (m *mockRouteStore) GetDownstreamServerByName(context.Context, string) (*store.DownstreamServer, error) { return nil, nil }
func (m *mockRouteStore) ListDownstreamServers(context.Context) ([]store.DownstreamServer, error)            { return nil, nil }
func (m *mockRouteStore) UpdateDownstreamServer(context.Context, *store.DownstreamServer) error              { return nil }
func (m *mockRouteStore) DeleteDownstreamServer(context.Context, string) error                               { return nil }
func (m *mockRouteStore) UpdateCapabilitiesCache(context.Context, string, json.RawMessage) error             { return nil }
func (m *mockRouteStore) CreateSession(context.Context, *store.Session) error                                { return nil }
func (m *mockRouteStore) GetSession(context.Context, string) (*store.Session, error)                         { return nil, nil }
func (m *mockRouteStore) DisconnectSession(context.Context, string) error                                    { return nil }
func (m *mockRouteStore) ListActiveSessions(context.Context) ([]store.Session, error)                        { return nil, nil }
func (m *mockRouteStore) CleanupStaleSessions(context.Context, time.Time) (int, error)                      { return 0, nil }
func (m *mockRouteStore) InsertAuditRecord(context.Context, *store.AuditRecord) error                       { return nil }
func (m *mockRouteStore) QueryAuditRecords(context.Context, store.AuditFilter) ([]store.AuditRecord, int, error) { return nil, 0, nil }
func (m *mockRouteStore) GetAuditStats(context.Context, string, time.Time, time.Time) (*store.AuditStats, error) { return nil, nil }
func (m *mockRouteStore) GetDashboardTimeSeries(context.Context, time.Time, time.Time) ([]store.TimeSeriesPoint, error) {
	return nil, nil
}
func (m *mockRouteStore) CreateToolApproval(context.Context, *store.ToolApproval) error { return nil }
func (m *mockRouteStore) GetToolApproval(context.Context, string) (*store.ToolApproval, error) { return nil, nil }
func (m *mockRouteStore) ListPendingApprovals(context.Context) ([]store.ToolApproval, error)   { return nil, nil }
func (m *mockRouteStore) ResolveToolApproval(context.Context, string, string, string, string, string) error { return nil }
func (m *mockRouteStore) ExpirePendingApprovals(context.Context, time.Time) (int, error) { return 0, nil }
func (m *mockRouteStore) Tx(context.Context, func(store.Store) error) error { return nil }
func (m *mockRouteStore) Ping(context.Context) error                        { return nil }
func (m *mockRouteStore) Close() error                                      { return nil }

func TestGlobMatch(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"**", "", true},
		{"**", "a/b/c", true},
		{"*", "foo", true},
		{"*", "foo/bar", false},
		{"src/**", "src/main.go", true},
		{"src/**", "src/pkg/util.go", true},
		{"src/**", "lib/main.go", false},
		{"src/*/main.go", "src/pkg/main.go", true},
		{"src/*/main.go", "src/pkg/sub/main.go", false},
		{"exact/path", "exact/path", true},
		{"exact/path", "exact/other", false},
		{"**/test", "a/b/test", true},
		{"**/test", "test", true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.path, func(t *testing.T) {
			got := GlobMatch(tt.pattern, tt.path)
			if got != tt.want {
				t.Errorf("GlobMatch(%q, %q) = %v, want %v",
					tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}

func TestGlobSpecificity(t *testing.T) {
	tests := []struct {
		pattern string
		want    int
	}{
		{"**", 0},
		{"*", 1},
		{"src/**", 10},
		{"src/pkg/*", 21},
		{"src/pkg/main.go", 30},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			got := GlobSpecificity(tt.pattern)
			if got != tt.want {
				t.Errorf("GlobSpecificity(%q) = %d, want %d",
					tt.pattern, got, tt.want)
			}
		})
	}
}

func TestMatchTool(t *testing.T) {
	tests := []struct {
		name     string
		tool     string
		patterns []string
		want     bool
	}{
		{"wildcard", "anything", []string{"*"}, true},
		{"exact", "github__create_issue", []string{"github__create_issue"}, true},
		{"prefix", "github__create_issue", []string{"github__*"}, true},
		{"no match", "slack__post", []string{"github__*"}, false},
		{"multi", "slack__post", []string{"github__*", "slack__*"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchTool(tt.tool, tt.patterns)
			if got != tt.want {
				t.Errorf("matchTool(%q, %v) = %v, want %v",
					tt.tool, tt.patterns, got, tt.want)
			}
		})
	}
}

func TestMatchRoute(t *testing.T) {
	rules := []parsedRule{
		{
			RouteRule: store.RouteRule{
				ID: "deny-rule", Priority: 100, PathGlob: "**",
				DownstreamServerID: "ds1", Policy: "deny",
			},
			toolPatterns: []string{"dangerous__*"},
			specificity:  0,
		},
		{
			RouteRule: store.RouteRule{
				ID: "allow-gh", Priority: 100, PathGlob: "**",
				DownstreamServerID: "gh-server", AuthScopeID: "gh-auth",
				Policy: "allow",
			},
			toolPatterns: []string{"github__*"},
			specificity:  0,
		},
		{
			RouteRule: store.RouteRule{
				ID: "allow-slack", Priority: 50, PathGlob: "**",
				DownstreamServerID: "slack-server", Policy: "allow",
			},
			toolPatterns: []string{"slack__*"},
			specificity:  0,
		},
	}
	sortRules(rules)

	tests := []struct {
		name    string
		ctx     RouteContext
		wantDS  string
		wantErr error
	}{
		{
			"match github",
			RouteContext{ToolName: "github__create_issue", Subpath: "src"},
			"gh-server", nil,
		},
		{
			"match slack lower priority",
			RouteContext{ToolName: "slack__post_message", Subpath: "any"},
			"slack-server", nil,
		},
		{
			"deny dangerous",
			RouteContext{ToolName: "dangerous__delete_all", Subpath: "any"},
			"", ErrDenied,
		},
		{
			"specificity override: more specific allow wins over less specific deny",
			RouteContext{ToolName: "linear__search", Subpath: "work/gateway/src"},
			"linear-srv", nil,
		},
		{
			"specificity override: less specific deny still blocks others",
			RouteContext{ToolName: "linear__search", Subpath: "work/other"},
			"", ErrDenied,
		},
		{
			"no match",
			RouteContext{ToolName: "unknown__tool", Subpath: "any"},
			"", ErrNoRoute,
		},
	}

	// Add specificity override rules for the new test cases.
	rules = append(rules,
		parsedRule{
			RouteRule: store.RouteRule{
				ID: "linear-deny", Priority: 0, PathGlob: "work/**",
				Policy: "deny",
			},
			toolPatterns: []string{"linear__*"},
			specificity:  10,
		},
		parsedRule{
			RouteRule: store.RouteRule{
				ID: "linear-allow-specific", Priority: 0, PathGlob: "work/gateway/**",
				DownstreamServerID: "linear-srv", Policy: "allow",
			},
			toolPatterns: []string{"linear__*"},
			specificity:  20,
		},
	)
	sortRules(rules)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := matchRoute(rules, tt.ctx)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if result.DownstreamServerID != tt.wantDS {
				t.Errorf("ds = %q, want %q",
					result.DownstreamServerID, tt.wantDS)
			}
		})
	}
}

func TestSortRules(t *testing.T) {
	rules := []parsedRule{
		{RouteRule: store.RouteRule{ID: "c", Priority: 50, PathGlob: "**"}, specificity: 0, toolSpecificity: 0},
		{RouteRule: store.RouteRule{ID: "a", Priority: 100, PathGlob: "src/*"}, specificity: 11, toolSpecificity: 0},
		{RouteRule: store.RouteRule{ID: "b", Priority: 100, PathGlob: "**"}, specificity: 0, toolSpecificity: 0},
		{RouteRule: store.RouteRule{ID: "d", Priority: 100, PathGlob: "**"}, specificity: 0, toolSpecificity: 1}, // More specific tool wins over "b"
	}

	sortRules(rules)

	want := []string{"a", "d", "b", "c"}
	for i, r := range rules {
		if r.ID != want[i] {
			t.Errorf("rules[%d].ID = %q, want %q", i, r.ID, want[i])
		}
	}
}

func TestSortRules_SpecificityBeatsPriority(t *testing.T) {
	// A high-priority catch-all must NOT beat a low-priority specific path.
	rules := []parsedRule{
		{RouteRule: store.RouteRule{ID: "catchall", Priority: 1000, PathGlob: "**"}, specificity: 0},
		{RouteRule: store.RouteRule{ID: "specific", Priority: 1, PathGlob: "src/components/auth"}, specificity: 30},
	}

	sortRules(rules)

	if rules[0].ID != "specific" {
		t.Errorf("expected most-specific path first, got %q", rules[0].ID)
	}
	if rules[1].ID != "catchall" {
		t.Errorf("expected catch-all second, got %q", rules[1].ID)
	}
}

// Verify parseToolMatch handles edge cases.
func TestParseToolMatch(t *testing.T) {
	tests := []struct {
		name string
		raw  json.RawMessage
		want []string
	}{
		{"nil", nil, []string{"*"}},
		{"empty array", json.RawMessage(`[]`), []string{"*"}},
		{"valid", json.RawMessage(`["github__*","slack__post"]`), []string{"github__*", "slack__post"}},
		{"invalid json", json.RawMessage(`not json`), []string{"*"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseToolMatch(tt.raw)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestRouteWithFallback(t *testing.T) {
	ms := &mockRouteStore{
		rules: map[string][]store.RouteRule{
			"ws-project": {
				// Project workspace has a specific rule for github tools.
				{
					ID: "proj-gh", WorkspaceID: "ws-project",
					Priority: 100, PathGlob: "**",
					DownstreamServerID: "gh-server", AuthScopeID: "gh-auth",
					Policy: "allow", ToolMatch: json.RawMessage(`["github__*"]`),
				},
			},
			"ws-global": {
				// Global workspace allows postgres tools.
				{
					ID: "global-pg", WorkspaceID: "ws-global",
					Priority: 100, PathGlob: "**",
					DownstreamServerID: "pg-server", AuthScopeID: "pg-auth",
					Policy: "allow", ToolMatch: json.RawMessage(`["postgres__*"]`),
				},
				// Global also allows github tools (should not be reached if project matches).
				{
					ID: "global-gh", WorkspaceID: "ws-global",
					Priority: 100, PathGlob: "**",
					DownstreamServerID: "gh-server-global", AuthScopeID: "gh-auth",
					Policy: "allow", ToolMatch: json.RawMessage(`["github__*"]`),
				},
			},
			"ws-deny": {
				// Deny workspace blocks postgres tools.
				{
					ID: "deny-pg", WorkspaceID: "ws-deny",
					Priority: 100, PathGlob: "**",
					Policy: "deny", ToolMatch: json.RawMessage(`["postgres__*"]`),
				},
			},
		},
	}
	engine := NewEngine(ms)

	tests := []struct {
		name       string
		tool       string
		clientRoot string
		ancestors  []WorkspaceAncestor
		wantDS     string
		wantErr    error
	}{
		{
			"first workspace matches",
			"github__create_issue",
			"/home/user/project",
			[]WorkspaceAncestor{
				{ID: "ws-project", RootPath: "/home/user/project"},
				{ID: "ws-global", RootPath: "/"},
			},
			"gh-server", nil,
		},
		{
			"fallback to parent workspace",
			"postgres__query",
			"/home/user/project",
			[]WorkspaceAncestor{
				{ID: "ws-project", RootPath: "/home/user/project"},
				{ID: "ws-global", RootPath: "/"},
			},
			"pg-server", nil,
		},
		{
			"deny blocks fallback",
			"postgres__query",
			"/home/user/project",
			[]WorkspaceAncestor{
				{ID: "ws-deny", RootPath: "/home/user/project"},
				{ID: "ws-global", RootPath: "/"},
			},
			"", ErrDenied,
		},
		{
			"empty chain uses default route",
			"github__create_issue",
			"",
			nil,
			"", ErrNoRoute,
		},
		{
			"all workspaces miss",
			"unknown__tool",
			"/home/user/project",
			[]WorkspaceAncestor{
				{ID: "ws-project", RootPath: "/home/user/project"},
				{ID: "ws-global", RootPath: "/"},
			},
			"", ErrNoRoute,
		},
		{
			"single workspace match",
			"postgres__query",
			"/home/user/project",
			[]WorkspaceAncestor{{ID: "ws-global", RootPath: "/"}},
			"pg-server", nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.RouteWithFallback(t.Context(), RouteContext{
				ToolName: tt.tool,
			}, tt.clientRoot, tt.ancestors)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if result.DownstreamServerID != tt.wantDS {
				t.Errorf("ds = %q, want %q",
					result.DownstreamServerID, tt.wantDS)
			}
		})
	}
}

func TestRouteWithFallback_PathScoped(t *testing.T) {
	ms := &mockRouteStore{
		rules: map[string][]store.RouteRule{
			"ws-project": {
				// Only allows github tools under src/**.
				{
					ID: "src-only", WorkspaceID: "ws-project",
					Priority: 100, PathGlob: "src/**",
					DownstreamServerID: "gh-server", AuthScopeID: "gh-auth",
					Policy: "allow", ToolMatch: json.RawMessage(`["github__*"]`),
				},
				// A catch-all for slack tools from anywhere.
				{
					ID: "slack-all", WorkspaceID: "ws-project",
					Priority: 50, PathGlob: "**",
					DownstreamServerID: "slack-server",
					Policy: "allow", ToolMatch: json.RawMessage(`["slack__*"]`),
				},
			},
		},
	}
	engine := NewEngine(ms)

	tests := []struct {
		name       string
		tool       string
		clientRoot string
		wantDS     string
		wantErr    error
	}{
		{
			"path-scoped rule matches when client is under src",
			"github__create_issue",
			"/home/user/project/src/api",
			"gh-server", nil,
		},
		{
			"path-scoped rule does NOT match at workspace root",
			"github__create_issue",
			"/home/user/project",
			"", ErrNoRoute,
		},
		{
			"path-scoped rule does NOT match outside src",
			"github__create_issue",
			"/home/user/project/docs",
			"", ErrNoRoute,
		},
		{
			"catch-all glob matches from workspace root",
			"slack__post_message",
			"/home/user/project",
			"slack-server", nil,
		},
		{
			"catch-all glob matches from subdirectory",
			"slack__post_message",
			"/home/user/project/src/api",
			"slack-server", nil,
		},
	}

	ancestors := []WorkspaceAncestor{
		{ID: "ws-project", RootPath: "/home/user/project"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.RouteWithFallback(t.Context(), RouteContext{
				ToolName: tt.tool,
			}, tt.clientRoot, ancestors)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if result.DownstreamServerID != tt.wantDS {
				t.Errorf("ds = %q, want %q",
					result.DownstreamServerID, tt.wantDS)
			}
		})
	}
}

func TestComputeSubpath(t *testing.T) {
	tests := []struct {
		name       string
		clientRoot string
		wsRoot     string
		want       string
	}{
		{"same path", "/home/user/project", "/home/user/project", ""},
		{"client under workspace", "/home/user/project/src/api", "/home/user/project", "src/api"},
		{"at workspace root", "/home/user/project", "/home/user/project", ""},
		{"empty client", "", "/home/user/project", ""},
		{"empty workspace", "/home/user/project", "", ""},
		{"client outside workspace", "/opt/other", "/home/user/project", ""},
		{"root workspace", "/home/user/project", "/", "home/user/project"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeSubpath(tt.clientRoot, tt.wsRoot)
			if got != tt.want {
				t.Errorf("ComputeSubpath(%q, %q) = %q, want %q",
					tt.clientRoot, tt.wsRoot, got, tt.want)
			}
		})
	}
}

func TestMatchRoute_NamespaceAware(t *testing.T) {
	rules := []parsedRule{
		{
			RouteRule: store.RouteRule{
				ID: "allow-linear", Priority: 100, PathGlob: "**",
				DownstreamServerID: "linear-server", Policy: "allow",
			},
			toolPatterns: []string{"*"},
			specificity:  0,
			namespace:    "linear",
		},
		{
			RouteRule: store.RouteRule{
				ID: "global-deny", Priority: 0, PathGlob: "**",
				Policy: "deny",
			},
			toolPatterns: []string{"*"},
			specificity:  0,
		},
	}
	sortRules(rules)

	tests := []struct {
		name    string
		tool    string
		wantDS  string
		wantErr error
	}{
		{"linear tool matches linear rule", "linear__search", "linear-server", nil},
		{"github tool skips linear rule, hits deny", "github__get_label", "", ErrDenied},
		{"no-namespace tool skips linear rule, hits deny", "some_tool", "", ErrDenied},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := matchRoute(rules, RouteContext{ToolName: tt.tool})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if result.DownstreamServerID != tt.wantDS {
				t.Errorf("ds = %q, want %q", result.DownstreamServerID, tt.wantDS)
			}
		})
	}
}

func TestRoute_NamespaceAware(t *testing.T) {
	ms := &mockRouteStore{
		rules: map[string][]store.RouteRule{
			"ws1": {
				{
					ID: "allow-linear", WorkspaceID: "ws1",
					Priority: 100, PathGlob: "**",
					DownstreamServerID: "linear-server",
					Policy: "allow", ToolMatch: json.RawMessage(`["*"]`),
				},
				{
					ID: "allow-github", WorkspaceID: "ws1",
					Priority: 100, PathGlob: "**",
					DownstreamServerID: "github-server",
					Policy: "allow", ToolMatch: json.RawMessage(`["*"]`),
				},
				{
					ID: "global-deny", WorkspaceID: "ws1",
					Priority: 0, PathGlob: "**",
					Policy: "deny", ToolMatch: json.RawMessage(`["*"]`),
				},
			},
		},
		downstreams: map[string]*store.DownstreamServer{
			"linear-server": {ID: "linear-server", ToolNamespace: "linear"},
			"github-server": {ID: "github-server", ToolNamespace: "github"},
		},
	}
	engine := NewEngine(ms)

	tests := []struct {
		name    string
		tool    string
		wantDS  string
		wantErr error
	}{
		{"github tool routes to github", "github__get_label", "github-server", nil},
		{"linear tool routes to linear", "linear__search", "linear-server", nil},
		{"unknown tool hits deny", "slack__post", "", ErrDenied},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.Route(t.Context(), RouteContext{
				WorkspaceID: "ws1",
				ToolName:    tt.tool,
			})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if result.DownstreamServerID != tt.wantDS {
				t.Errorf("ds = %q, want %q", result.DownstreamServerID, tt.wantDS)
			}
		})
	}
}
