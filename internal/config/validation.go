package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidationError holds all validation failures for a config file.
type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("config validation failed: %s", strings.Join(e.Errors, "; "))
}

// validate checks the parsed config for correctness.
func validate(cfg *FileConfig) error {
	var errs []string

	providerIDs := make(map[string]bool, len(cfg.OAuthProviders))
	for i, p := range cfg.OAuthProviders {
		if p.ID == "" {
			errs = append(errs, fmt.Sprintf("oauth_providers[%d]: id is required", i))
		}
		if providerIDs[p.ID] {
			errs = append(errs, fmt.Sprintf("oauth_providers[%d]: duplicate id %q", i, p.ID))
		}
		providerIDs[p.ID] = true
		if p.Name == "" {
			errs = append(errs, fmt.Sprintf("oauth_providers[%d]: name is required", i))
		}
		if p.AuthorizeURL == "" {
			errs = append(errs, fmt.Sprintf("oauth_providers[%d]: authorize_url is required", i))
		}
		if p.TokenURL == "" {
			errs = append(errs, fmt.Sprintf("oauth_providers[%d]: token_url is required", i))
		}
	}

	wsIDs := make(map[string]bool, len(cfg.Workspaces))
	for i, ws := range cfg.Workspaces {
		if ws.ID == "" {
			errs = append(errs, fmt.Sprintf("workspaces[%d]: id is required", i))
		}
		if wsIDs[ws.ID] {
			errs = append(errs, fmt.Sprintf("workspaces[%d]: duplicate id %q", i, ws.ID))
		}
		wsIDs[ws.ID] = true
		if ws.Name == "" {
			errs = append(errs, fmt.Sprintf("workspaces[%d]: name is required", i))
		}
		if err := validatePolicy(ws.DefaultPolicy); err != nil {
			errs = append(errs, fmt.Sprintf("workspaces[%d]: %v", i, err))
		}
	}

	scopeIDs := make(map[string]bool, len(cfg.AuthScopes))
	for i, s := range cfg.AuthScopes {
		if s.ID == "" {
			errs = append(errs, fmt.Sprintf("auth_scopes[%d]: id is required", i))
		}
		if scopeIDs[s.ID] {
			errs = append(errs, fmt.Sprintf("auth_scopes[%d]: duplicate id %q", i, s.ID))
		}
		scopeIDs[s.ID] = true
		if s.OAuthProviderID != "" && !providerIDs[s.OAuthProviderID] {
			errs = append(errs, fmt.Sprintf("auth_scopes[%d]: oauth_provider_id %q not found", i, s.OAuthProviderID))
		}
	}

	dsIDs := make(map[string]bool, len(cfg.DownstreamServers))
	nsSet := make(map[string]bool, len(cfg.DownstreamServers))
	for i, ds := range cfg.DownstreamServers {
		if ds.ID == "" {
			errs = append(errs, fmt.Sprintf("downstream_servers[%d]: id is required", i))
		}
		if dsIDs[ds.ID] {
			errs = append(errs, fmt.Sprintf("downstream_servers[%d]: duplicate id %q", i, ds.ID))
		}
		dsIDs[ds.ID] = true
		if ds.ToolNamespace == "" {
			errs = append(errs, fmt.Sprintf("downstream_servers[%d]: tool_namespace is required", i))
		}
		if nsSet[ds.ToolNamespace] {
			errs = append(errs, fmt.Sprintf("downstream_servers[%d]: duplicate namespace %q", i, ds.ToolNamespace))
		}
		nsSet[ds.ToolNamespace] = true
		if err := validateTransport(ds.Transport); err != nil {
			errs = append(errs, fmt.Sprintf("downstream_servers[%d]: %v", i, err))
		}
	}

	errs = append(errs, validateRouteRules(cfg.RouteRules, wsIDs, dsIDs, scopeIDs)...)

	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}
	return nil
}

func validateRouteRules(rules []routeRuleConfig, wsIDs, dsIDs, scopeIDs map[string]bool) []string {
	var errs []string
	ruleIDs := make(map[string]bool, len(rules))

	for i, r := range rules {
		if r.ID == "" {
			errs = append(errs, fmt.Sprintf("route_rules[%d]: id is required", i))
		}
		if ruleIDs[r.ID] {
			errs = append(errs, fmt.Sprintf("route_rules[%d]: duplicate id %q", i, r.ID))
		}
		ruleIDs[r.ID] = true

		if r.WorkspaceID != "" && !wsIDs[r.WorkspaceID] {
			errs = append(errs, fmt.Sprintf("route_rules[%d]: workspace_id %q not found", i, r.WorkspaceID))
		}
		if r.DownstreamServerID != "" && !dsIDs[r.DownstreamServerID] {
			errs = append(errs, fmt.Sprintf("route_rules[%d]: downstream_server_id %q not found", i, r.DownstreamServerID))
		}
		if r.AuthScopeID != "" && !scopeIDs[r.AuthScopeID] {
			errs = append(errs, fmt.Sprintf("route_rules[%d]: auth_scope_id %q not found", i, r.AuthScopeID))
		}
		if err := validateGlob(r.PathGlob); err != nil {
			errs = append(errs, fmt.Sprintf("route_rules[%d]: %v", i, err))
		}
		if err := validatePolicy(r.Policy); err != nil {
			errs = append(errs, fmt.Sprintf("route_rules[%d]: %v", i, err))
		}
	}
	return errs
}

func validatePolicy(p string) error {
	switch p {
	case "allow", "deny", "":
		return nil
	default:
		return fmt.Errorf("invalid policy %q (must be allow or deny)", p)
	}
}

func validateTransport(t string) error {
	switch t {
	case "stdio", "http", "":
		return nil
	default:
		return fmt.Errorf("invalid transport %q (must be stdio or http)", t)
	}
}

func validateGlob(pattern string) error {
	if pattern == "" {
		return nil
	}
	_, err := filepath.Match(pattern, "test")
	if err != nil {
		return fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
	}
	return nil
}
