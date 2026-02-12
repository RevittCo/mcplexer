package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/revitteth/mcplexer/internal/store"
	"gopkg.in/yaml.v3"
)

// FileConfig represents the top-level mcplexer.yaml structure.
type FileConfig struct {
	OAuthProviders    []oauthProviderConfig    `yaml:"oauth_providers"`
	Workspaces        []workspaceConfig        `yaml:"workspaces"`
	AuthScopes        []authScopeConfig        `yaml:"auth_scopes"`
	DownstreamServers []downstreamServerConfig `yaml:"downstream_servers"`
	RouteRules        []routeRuleConfig        `yaml:"route_rules"`
}

type workspaceConfig struct {
	ID            string   `yaml:"id"`
	Name          string   `yaml:"name"`
	RootPath      string   `yaml:"root_path"`
	Tags          []string `yaml:"tags,omitempty"`
	DefaultPolicy string   `yaml:"default_policy"`
}

type oauthProviderConfig struct {
	ID           string   `yaml:"id"`
	Name         string   `yaml:"name"`
	AuthorizeURL string   `yaml:"authorize_url"`
	TokenURL     string   `yaml:"token_url"`
	ClientID     string   `yaml:"client_id"`
	Scopes       []string `yaml:"scopes,omitempty"`
	UsePKCE      bool     `yaml:"use_pkce"`
}

type authScopeConfig struct {
	ID              string   `yaml:"id"`
	Name            string   `yaml:"name"`
	Type            string   `yaml:"type"`
	OAuthProviderID string   `yaml:"oauth_provider_id,omitempty"`
	RedactionHints  []string `yaml:"redaction_hints,omitempty"`
}

type downstreamServerConfig struct {
	ID             string   `yaml:"id"`
	Name           string   `yaml:"name"`
	Transport      string   `yaml:"transport"`
	Command        string   `yaml:"command"`
	Args           []string `yaml:"args,omitempty"`
	URL            string   `yaml:"url,omitempty"`
	ToolNamespace  string   `yaml:"tool_namespace"`
	Discovery      string   `yaml:"discovery,omitempty"` // "static" (default) or "dynamic"
	IdleTimeoutSec int      `yaml:"idle_timeout_sec"`
	MaxInstances   int      `yaml:"max_instances"`
	RestartPolicy  string   `yaml:"restart_policy"`
}

type routeRuleConfig struct {
	ID                 string `yaml:"id"`
	Priority           int    `yaml:"priority"`
	WorkspaceID        string `yaml:"workspace_id"`
	PathGlob           string `yaml:"path_glob"`
	ToolMatch          string `yaml:"tool_match"`
	DownstreamServerID string `yaml:"downstream_server_id"`
	AuthScopeID        string `yaml:"auth_scope_id"`
	Policy             string `yaml:"policy"`
	LogLevel           string `yaml:"log_level"`
}

// LoadFile reads, parses, and validates a YAML config file.
func LoadFile(path string) (*FileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	return Parse(data)
}

// Parse parses and validates YAML config data.
func Parse(data []byte) (*FileConfig, error) {
	var cfg FileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	if err := validate(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Apply upserts all config entities into the store within a transaction.
// Items from YAML are tagged with source="yaml". Stale yaml-sourced rows
// that no longer appear in the file are deleted automatically.
func Apply(ctx context.Context, s store.Store, cfg *FileConfig) error {
	return s.Tx(ctx, func(tx store.Store) error {
		if err := applyOAuthProviders(ctx, tx, cfg.OAuthProviders); err != nil {
			return err
		}
		if err := applyWorkspaces(ctx, tx, cfg.Workspaces); err != nil {
			return err
		}
		if err := applyAuthScopes(ctx, tx, cfg.AuthScopes); err != nil {
			return err
		}
		if err := applyDownstreamServers(ctx, tx, cfg.DownstreamServers); err != nil {
			return err
		}
		return applyRouteRules(ctx, tx, cfg.RouteRules)
	})
}

func applyWorkspaces(ctx context.Context, tx store.Store, items []workspaceConfig) error {
	yamlIDs := make(map[string]bool, len(items))
	for _, w := range items {
		yamlIDs[w.ID] = true
		tags, _ := json.Marshal(w.Tags)
		ws := &store.Workspace{
			ID: w.ID, Name: w.Name, RootPath: w.RootPath,
			Tags: tags, DefaultPolicy: w.DefaultPolicy, Source: "yaml",
			UpdatedAt: time.Now().UTC(),
		}
		existing, err := tx.GetWorkspace(ctx, w.ID)
		if err != nil {
			ws.CreatedAt = time.Now().UTC()
			if err := tx.CreateWorkspace(ctx, ws); err != nil {
				return fmt.Errorf("create workspace %s: %w", w.ID, err)
			}
			continue
		}
		ws.CreatedAt = existing.CreatedAt
		if err := tx.UpdateWorkspace(ctx, ws); err != nil {
			return fmt.Errorf("update workspace %s: %w", w.ID, err)
		}
	}
	return pruneStaleWorkspaces(ctx, tx, yamlIDs)
}

func applyOAuthProviders(ctx context.Context, tx store.Store, items []oauthProviderConfig) error {
	yamlIDs := make(map[string]bool, len(items))
	for _, o := range items {
		yamlIDs[o.ID] = true
		scopes, _ := json.Marshal(o.Scopes)
		p := &store.OAuthProvider{
			ID: o.ID, Name: o.Name,
			AuthorizeURL: o.AuthorizeURL, TokenURL: o.TokenURL,
			ClientID: o.ClientID, Scopes: scopes,
			UsePKCE: o.UsePKCE, Source: "yaml",
			UpdatedAt: time.Now().UTC(),
		}
		existing, err := tx.GetOAuthProvider(ctx, o.ID)
		if err != nil {
			p.CreatedAt = time.Now().UTC()
			if err := tx.CreateOAuthProvider(ctx, p); err != nil {
				return fmt.Errorf("create oauth provider %s: %w", o.ID, err)
			}
			continue
		}
		p.CreatedAt = existing.CreatedAt
		p.EncryptedClientSecret = existing.EncryptedClientSecret // preserve secret
		if err := tx.UpdateOAuthProvider(ctx, p); err != nil {
			return fmt.Errorf("update oauth provider %s: %w", o.ID, err)
		}
	}
	return pruneStaleOAuthProviders(ctx, tx, yamlIDs)
}

func applyAuthScopes(ctx context.Context, tx store.Store, items []authScopeConfig) error {
	yamlIDs := make(map[string]bool, len(items))
	for _, a := range items {
		yamlIDs[a.ID] = true
		hints, _ := json.Marshal(a.RedactionHints)
		as := &store.AuthScope{
			ID: a.ID, Name: a.Name, Type: a.Type,
			OAuthProviderID: a.OAuthProviderID,
			RedactionHints:  hints, Source: "yaml",
			UpdatedAt: time.Now().UTC(),
		}
		existing, err := tx.GetAuthScope(ctx, a.ID)
		if err != nil {
			as.CreatedAt = time.Now().UTC()
			if err := tx.CreateAuthScope(ctx, as); err != nil {
				return fmt.Errorf("create auth scope %s: %w", a.ID, err)
			}
			continue
		}
		as.CreatedAt = existing.CreatedAt
		as.EncryptedData = existing.EncryptedData   // preserve secrets
		as.OAuthTokenData = existing.OAuthTokenData // preserve token data
		if err := tx.UpdateAuthScope(ctx, as); err != nil {
			return fmt.Errorf("update auth scope %s: %w", a.ID, err)
		}
	}
	return pruneStaleAuthScopes(ctx, tx, yamlIDs)
}

func applyDownstreamServers(ctx context.Context, tx store.Store, items []downstreamServerConfig) error {
	yamlIDs := make(map[string]bool, len(items))
	for _, d := range items {
		yamlIDs[d.ID] = true
		args, _ := json.Marshal(d.Args)
		ds := &store.DownstreamServer{
			ID: d.ID, Name: d.Name, Transport: d.Transport,
			Command: d.Command, Args: args, ToolNamespace: d.ToolNamespace,
			Discovery: d.Discovery, IdleTimeoutSec: d.IdleTimeoutSec,
			MaxInstances: d.MaxInstances, RestartPolicy: d.RestartPolicy,
			Source: "yaml", UpdatedAt: time.Now().UTC(),
		}
		if d.URL != "" {
			ds.URL = &d.URL
		}
		existing, err := tx.GetDownstreamServer(ctx, d.ID)
		if err != nil {
			ds.CreatedAt = time.Now().UTC()
			if err := tx.CreateDownstreamServer(ctx, ds); err != nil {
				return fmt.Errorf("create downstream %s: %w", d.ID, err)
			}
			continue
		}
		ds.CreatedAt = existing.CreatedAt
		ds.CapabilitiesCache = existing.CapabilitiesCache
		if err := tx.UpdateDownstreamServer(ctx, ds); err != nil {
			return fmt.Errorf("update downstream %s: %w", d.ID, err)
		}
	}
	return pruneStaleDownstreams(ctx, tx, yamlIDs)
}

func applyRouteRules(ctx context.Context, tx store.Store, items []routeRuleConfig) error {
	yamlIDs := make(map[string]bool, len(items))
	for _, r := range items {
		yamlIDs[r.ID] = true
		toolMatch, _ := json.Marshal([]string{r.ToolMatch})
		rr := &store.RouteRule{
			ID: r.ID, Priority: r.Priority, WorkspaceID: r.WorkspaceID,
			PathGlob: r.PathGlob, ToolMatch: toolMatch,
			DownstreamServerID: r.DownstreamServerID,
			AuthScopeID: r.AuthScopeID, Policy: r.Policy,
			LogLevel: r.LogLevel, Source: "yaml",
			UpdatedAt: time.Now().UTC(),
		}
		existing, err := tx.GetRouteRule(ctx, r.ID)
		if err != nil {
			rr.CreatedAt = time.Now().UTC()
			if err := tx.CreateRouteRule(ctx, rr); err != nil {
				return fmt.Errorf("create route rule %s: %w", r.ID, err)
			}
			continue
		}
		rr.CreatedAt = existing.CreatedAt
		if err := tx.UpdateRouteRule(ctx, rr); err != nil {
			return fmt.Errorf("update route rule %s: %w", r.ID, err)
		}
	}
	return pruneStaleRouteRules(ctx, tx, yamlIDs)
}

func pruneStaleWorkspaces(ctx context.Context, tx store.Store, yamlIDs map[string]bool) error {
	all, err := tx.ListWorkspaces(ctx)
	if err != nil {
		return fmt.Errorf("list workspaces for prune: %w", err)
	}
	for _, w := range all {
		if w.Source == "yaml" && !yamlIDs[w.ID] {
			slog.Info("pruning stale yaml workspace", "id", w.ID)
			if err := tx.DeleteWorkspace(ctx, w.ID); err != nil {
				return fmt.Errorf("delete stale workspace %s: %w", w.ID, err)
			}
		}
	}
	return nil
}

func pruneStaleOAuthProviders(ctx context.Context, tx store.Store, yamlIDs map[string]bool) error {
	all, err := tx.ListOAuthProviders(ctx)
	if err != nil {
		return fmt.Errorf("list oauth providers for prune: %w", err)
	}
	for _, p := range all {
		if p.Source == "yaml" && !yamlIDs[p.ID] {
			slog.Info("pruning stale yaml oauth provider", "id", p.ID)
			if err := tx.DeleteOAuthProvider(ctx, p.ID); err != nil {
				return fmt.Errorf("delete stale oauth provider %s: %w", p.ID, err)
			}
		}
	}
	return nil
}

func pruneStaleAuthScopes(ctx context.Context, tx store.Store, yamlIDs map[string]bool) error {
	all, err := tx.ListAuthScopes(ctx)
	if err != nil {
		return fmt.Errorf("list auth scopes for prune: %w", err)
	}
	for _, a := range all {
		if a.Source == "yaml" && !yamlIDs[a.ID] {
			slog.Info("pruning stale yaml auth scope", "id", a.ID)
			if err := tx.DeleteAuthScope(ctx, a.ID); err != nil {
				return fmt.Errorf("delete stale auth scope %s: %w", a.ID, err)
			}
		}
	}
	return nil
}

func pruneStaleDownstreams(ctx context.Context, tx store.Store, yamlIDs map[string]bool) error {
	all, err := tx.ListDownstreamServers(ctx)
	if err != nil {
		return fmt.Errorf("list downstreams for prune: %w", err)
	}
	for _, d := range all {
		if d.Source == "yaml" && !yamlIDs[d.ID] {
			slog.Info("pruning stale yaml downstream", "id", d.ID)
			if err := tx.DeleteDownstreamServer(ctx, d.ID); err != nil {
				return fmt.Errorf("delete stale downstream %s: %w", d.ID, err)
			}
		}
	}
	return nil
}

func pruneStaleRouteRules(ctx context.Context, tx store.Store, yamlIDs map[string]bool) error {
	all, err := tx.ListRouteRules(ctx, "")
	if err != nil {
		return fmt.Errorf("list route rules for prune: %w", err)
	}
	for _, r := range all {
		if r.Source == "yaml" && !yamlIDs[r.ID] {
			slog.Info("pruning stale yaml route rule", "id", r.ID)
			if err := tx.DeleteRouteRule(ctx, r.ID); err != nil {
				return fmt.Errorf("delete stale route rule %s: %w", r.ID, err)
			}
		}
	}
	return nil
}
