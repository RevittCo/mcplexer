package routing

import (
	"context"
	"errors"
	"strings"

	"github.com/revitteth/mcplexer/internal/store"
)

// WorkspaceAncestor pairs a workspace ID with its root path for subpath
// computation during routing.
type WorkspaceAncestor struct {
	ID       string
	RootPath string
}

// RouteContext is the input to the routing engine.
type RouteContext struct {
	WorkspaceID string
	Subpath     string
	ToolName    string
}

// RouteResult is the output of a successful route match.
type RouteResult struct {
	DownstreamServerID string
	AuthScopeID        string
	MatchedRuleID      string
	OriginalToolName   string
}

var (
	// ErrNoRoute means no matching route rule was found.
	ErrNoRoute = errors.New("no matching route")

	// ErrDenied means a deny rule matched.
	ErrDenied = errors.New("route denied by policy")
)

// Engine resolves tool calls to downstream servers via route rules.
type Engine struct {
	store store.Store
}

// NewEngine creates a new routing engine.
func NewEngine(s store.Store) *Engine {
	return &Engine{store: s}
}

// Route finds the best matching route for the given context.
func (e *Engine) Route(ctx context.Context, rc RouteContext) (*RouteResult, error) {
	rules, err := e.store.ListRouteRules(ctx, rc.WorkspaceID)
	if err != nil {
		return nil, err
	}

	parsed := parseRules(rules)
	sortRules(parsed)

	return matchRoute(parsed, rc)
}

// RouteWithFallback tries routing through a chain of workspace ancestors (most
// specific first), computing the subpath for each workspace from the client's
// root directory. A deny at any level stops the search. ErrNoRoute continues
// to the next ancestor. Returns the first successful match or ErrNoRoute.
func (e *Engine) RouteWithFallback(ctx context.Context, rc RouteContext, clientRoot string, ancestors []WorkspaceAncestor) (*RouteResult, error) {
	if len(ancestors) == 0 {
		return e.Route(ctx, rc)
	}

	for _, ws := range ancestors {
		rc.WorkspaceID = ws.ID
		rc.Subpath = ComputeSubpath(clientRoot, ws.RootPath)
		result, err := e.Route(ctx, rc)
		if err == nil {
			return result, nil
		}
		if err == ErrDenied {
			return nil, ErrDenied
		}
		// ErrNoRoute: continue to next ancestor.
	}
	return nil, ErrNoRoute
}

// ComputeSubpath returns the relative path of clientRoot within wsRoot.
// If the client is at the workspace root, returns "" (matches "**").
// If clientRoot is not under wsRoot, returns "".
// ComputeSubpath returns the relative path of clientRoot within wsRoot.
// If the client is at the workspace root, returns "" (matches "**").
// If clientRoot is not under wsRoot, returns "".
func ComputeSubpath(clientRoot, wsRoot string) string {
	if clientRoot == "" || wsRoot == "" {
		return ""
	}

	// Normalize by trimming trailing slashes.
	clientRoot = strings.TrimSuffix(clientRoot, "/")
	wsRoot = strings.TrimSuffix(wsRoot, "/")

	if clientRoot == wsRoot {
		return ""
	}
	// Handle root workspace "/" specially — no trailing separator needed.
	if wsRoot == "" { // Was originally "/" but after TrimSuffix it's ""
		return strings.TrimPrefix(clientRoot, "/")
	}

	if sub, ok := strings.CutPrefix(clientRoot, wsRoot+"/"); ok {
		return sub
	}
	return ""
}

// matchRoute evaluates sorted rules against the route context.
// The first rule to match (by priority and specificity) wins.
func matchRoute(rules []parsedRule, rc RouteContext) (*RouteResult, error) {
	for i := range rules {
		r := &rules[i]

		if !GlobMatch(r.PathGlob, rc.Subpath) {
			continue
		}
		if !matchTool(rc.ToolName, r.toolPatterns) {
			continue
		}

		if r.Policy == "deny" {
			return nil, ErrDenied
		}

		return &RouteResult{
			DownstreamServerID: r.DownstreamServerID,
			AuthScopeID:        r.AuthScopeID,
			MatchedRuleID:      r.ID,
			OriginalToolName:   rc.ToolName,
		}, nil
	}

	return nil, ErrNoRoute
}
