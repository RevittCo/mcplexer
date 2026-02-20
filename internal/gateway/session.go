package gateway

import (
	"context"
	"log/slog"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/revittco/mcplexer/internal/routing"
	"github.com/revittco/mcplexer/internal/store"
)

// TransportMode controls how the client root is determined.
type TransportMode int

const (
	// TransportStdio uses os.Getwd() as the trusted client root. This is
	// secure because the CWD is inherited from the parent process and cannot
	// be spoofed via the MCP protocol.
	TransportStdio TransportMode = iota

	// TransportSocket accepts client-reported MCP roots. Used for Unix socket
	// and HTTP connections where the server process CWD is unrelated to the
	// client's working directory.
	TransportSocket
)

// sessionManager manages the current MCP client session.
type sessionManager struct {
	store      store.Store
	transport  TransportMode
	session    *store.Session
	clientPath string                     // trusted client CWD
	wsChain    []routing.WorkspaceAncestor // resolved workspace ancestors, most specific first

	// Active tool set for dynamic tool loading.
	activeTools map[string]Tool
	toolsMu     sync.RWMutex
}

func newSessionManager(s store.Store, t TransportMode) *sessionManager {
	return &sessionManager{store: s, transport: t}
}

func (sm *sessionManager) create(ctx context.Context, clientInfo ClientInfo, roots []Root) error {
	sm.session = &store.Session{
		ID:         uuid.NewString(),
		ClientType: clientInfo.Name,
		ModelHint:  clientInfo.Version,
	}

	sm.wsChain = sm.resolveWorkspaceChain(ctx, roots)
	if len(sm.wsChain) > 0 {
		sm.session.WorkspaceID = &sm.wsChain[0].ID
	}

	return sm.store.CreateSession(ctx, sm.session)
}

// resolveWorkspaceChain finds all workspaces whose root path is an ancestor
// of the client's working directory, ordered from most specific to least.
//
// Security: in stdio mode the client root is determined from os.Getwd()
// (inherited from the parent process) which cannot be spoofed via MCP. In
// socket/HTTP mode the server CWD is unrelated to the client, so we accept
// client-reported roots with a log for auditability.
func (sm *sessionManager) resolveWorkspaceChain(ctx context.Context, roots []Root) []routing.WorkspaceAncestor {
	clientRoot := sm.detectClientRoot(roots)
	sm.clientPath = clientRoot

	workspaces, err := sm.store.ListWorkspaces(ctx)
	if err != nil {
		slog.Warn("failed to list workspaces for session binding", "error", err)
		return nil
	}

	// Collect all ancestor workspaces.
	var ancestors []store.Workspace
	for _, ws := range workspaces {
		if ws.RootPath != "" && isPathAncestor(ws.RootPath, clientRoot) {
			ancestors = append(ancestors, ws)
		}
	}

	// Sort by path length descending (most specific first).
	sort.Slice(ancestors, func(i, j int) bool {
		return len(ancestors[i].RootPath) > len(ancestors[j].RootPath)
	})

	chain := make([]routing.WorkspaceAncestor, len(ancestors))
	for i, ws := range ancestors {
		chain[i] = routing.WorkspaceAncestor{ID: ws.ID, Name: ws.Name, RootPath: ws.RootPath}
	}
	return chain
}

// detectClientRoot determines the client's working directory based on the
// transport mode.
//
// stdio:  os.Getwd() — inherited from parent, tamper-proof via MCP.
// socket: client-reported MCP roots — logged for audit trail.
func (sm *sessionManager) detectClientRoot(roots []Root) string {
	if sm.transport == TransportStdio {
		cwd, err := os.Getwd()
		if err != nil {
			slog.Error("failed to detect working directory", "error", err)
			return ""
		}
		// Warn if client-reported roots disagree with the actual CWD.
		sm.validateClientRoots(cwd, roots)
		return cwd
	}

	// Socket/HTTP mode: use client-reported roots.
	var clientRoot string
	for _, root := range roots {
		if p := uriToPath(root.URI); p != "" {
			clientRoot = p
			break
		}
	}
	if clientRoot == "" {
		slog.Warn("no client root detected from MCP roots",
			"roots_count", len(roots), "transport", "socket")
	} else {
		slog.Info("session bound from client-reported root",
			"root", clientRoot, "transport", "socket")
	}
	return clientRoot
}

// validateClientRoots checks that client-reported MCP roots are consistent
// with the actual process CWD. Logs a warning on mismatch — this could
// indicate a spoofing attempt in stdio mode.
func (sm *sessionManager) validateClientRoots(cwd string, roots []Root) {
	if len(roots) == 0 {
		return
	}
	for _, root := range roots {
		p := uriToPath(root.URI)
		if p == "" {
			continue
		}
		if p == cwd || isPathAncestor(p, cwd) || isPathAncestor(cwd, p) {
			return // at least one root is consistent
		}
	}
	var reported []string
	for _, root := range roots {
		reported = append(reported, root.URI)
	}
	slog.Warn("client-reported roots do not match process CWD",
		"cwd", cwd, "reported_roots", reported)
}

// isPathAncestor returns true if ancestor is a path ancestor of (or equal to) path.
// It checks path boundaries to prevent "/users/m" matching "/users/max".
func isPathAncestor(ancestor, path string) bool {
	ancestor = strings.TrimSuffix(ancestor, "/")
	path = strings.TrimSuffix(path, "/")

	if ancestor == path {
		return true
	}
	if ancestor == "" { // Was "/"
		return true
	}
	return strings.HasPrefix(path, ancestor+"/")
}

// uriToPath extracts a filesystem path from a file:// URI.
func uriToPath(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return uri // best-effort: treat as raw path
	}
	if u.Scheme == "file" {
		return u.Path
	}
	return uri
}

func (sm *sessionManager) disconnect(ctx context.Context) error {
	if sm.session == nil {
		return nil
	}
	return sm.store.DisconnectSession(ctx, sm.session.ID)
}

func (sm *sessionManager) sessionID() string {
	if sm.session == nil {
		return ""
	}
	return sm.session.ID
}

func (sm *sessionManager) workspaceID() string {
	if len(sm.wsChain) == 0 {
		return ""
	}
	return sm.wsChain[0].ID
}

func (sm *sessionManager) workspaceName() string {
	if len(sm.wsChain) == 0 {
		return ""
	}
	return sm.wsChain[0].Name
}

func (sm *sessionManager) workspaceAncestors() []routing.WorkspaceAncestor {
	return sm.wsChain
}

func (sm *sessionManager) clientRoot() string {
	return sm.clientPath
}

func (sm *sessionManager) clientType() string {
	if sm.session == nil {
		return ""
	}
	return sm.session.ClientType
}

func (sm *sessionManager) modelHint() string {
	if sm.session == nil {
		return ""
	}
	return sm.session.ModelHint
}

// loadTools adds tools to the session's active set.
func (sm *sessionManager) loadTools(tools []Tool) {
	sm.toolsMu.Lock()
	defer sm.toolsMu.Unlock()
	if sm.activeTools == nil {
		sm.activeTools = make(map[string]Tool, len(tools))
	}
	for _, t := range tools {
		sm.activeTools[t.Name] = t
	}
}

// unloadTools removes tools from the session's active set by name.
// Returns the number of tools actually removed.
func (sm *sessionManager) unloadTools(names []string) int {
	sm.toolsMu.Lock()
	defer sm.toolsMu.Unlock()
	removed := 0
	for _, name := range names {
		if _, ok := sm.activeTools[name]; ok {
			delete(sm.activeTools, name)
			removed++
		}
	}
	return removed
}

// getActiveTools returns a snapshot of all loaded tools.
func (sm *sessionManager) getActiveTools() []Tool {
	sm.toolsMu.RLock()
	defer sm.toolsMu.RUnlock()
	tools := make([]Tool, 0, len(sm.activeTools))
	for _, t := range sm.activeTools {
		tools = append(tools, t)
	}
	return tools
}

