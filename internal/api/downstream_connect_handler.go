package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/revitteth/mcplexer/internal/oauth"
	"github.com/revitteth/mcplexer/internal/secrets"
	"github.com/revitteth/mcplexer/internal/store"
)

type downstreamConnectHandler struct {
	store       store.Store
	flowManager *oauth.FlowManager
	encryptor   *secrets.AgeEncryptor
}

type connectRequest struct {
	WorkspaceID  string `json:"workspace_id"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	ScopeName    string `json:"scope_name"`
}

type connectResponse struct {
	AuthScope    store.AuthScope       `json:"auth_scope"`
	Provider     oauthProviderResponse `json:"provider"`
	RouteRule    store.RouteRule       `json:"route_rule"`
	AuthorizeURL string                `json:"authorize_url"`
}

// POST /api/v1/downstreams/{id}/connect
func (h *downstreamConnectHandler) connect(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx := r.Context()

	var req connectRequest
	if err := decodeJSON(r, &req); err != nil {
		req = connectRequest{}
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = "global"
	}

	server, err := h.store.GetDownstreamServer(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "downstream server not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get server")
		return
	}

	if server.Transport != "http" || server.URL == nil {
		writeError(w, http.StatusBadRequest,
			"connect only works for HTTP transport servers")
		return
	}

	// Verify workspace exists.
	if _, err := h.store.GetWorkspace(ctx, req.WorkspaceID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusBadRequest,
				"workspace \""+req.WorkspaceID+"\" not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get workspace")
		return
	}

	var (
		provider *store.OAuthProvider
		scope    *store.AuthScope
		rule     *store.RouteRule
		authURL  string
	)

	txErr := h.store.Tx(ctx, func(tx store.Store) error {
		var txErr error

		// Step 1: Find or configure the OAuth provider.
		provider, txErr = h.findOrConfigureProvider(ctx, tx, server, &req)
		if txErr != nil {
			return txErr
		}

		// Step 2: Create or find auth scope.
		scopeName := req.ScopeName
		if scopeName == "" {
			scopeName = server.ToolNamespace + "_oauth"
		}
		scope, txErr = h.findOrCreateScope(ctx, tx, scopeName, provider.ID)
		if txErr != nil {
			return txErr
		}

		// Step 3: Create route rule (idempotent).
		rule, txErr = h.findOrCreateRoute(
			ctx, tx, req.WorkspaceID, server.ID, scope.ID)
		if txErr != nil {
			return txErr
		}

		return nil
	})
	if txErr != nil {
		writeError(w, http.StatusBadRequest, txErr.Error())
		return
	}

	// Build authorize URL (outside tx, uses FlowManager).
	authURL, err = h.flowManager.AuthorizeURL(ctx, scope.ID)
	if err != nil {
		writeErrorDetail(w, http.StatusInternalServerError,
			"failed to build authorize URL", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, connectResponse{
		AuthScope:    *scope,
		Provider:     newOAuthProviderResponse(provider),
		RouteRule:    *rule,
		AuthorizeURL: authURL,
	})
}

// findOrConfigureProvider finds a seeded template provider and updates it
// with credentials, or falls back to auto-discovery + DCR.
func (h *downstreamConnectHandler) findOrConfigureProvider(
	ctx context.Context,
	tx store.Store,
	server *store.DownstreamServer,
	req *connectRequest,
) (*store.OAuthProvider, error) {
	// Look for a seeded provider whose template_id matches the server ID.
	providers, err := tx.ListOAuthProviders(ctx)
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}

	var tmplProvider *store.OAuthProvider
	for i := range providers {
		if providers[i].TemplateID == server.ID {
			tmplProvider = &providers[i]
			break
		}
	}

	if tmplProvider != nil {
		// Template provider found — update with client credentials.
		if req.ClientID == "" {
			return nil, fmt.Errorf(
				"client_id is required for %s (template-based provider)", server.Name)
		}
		tmplProvider.ClientID = req.ClientID
		tmplProvider.UpdatedAt = time.Now().UTC()

		if req.ClientSecret != "" {
			if h.encryptor == nil {
				return nil, fmt.Errorf("encryption not configured")
			}
			enc, err := h.encryptor.Encrypt(
				[]byte(strings.TrimSpace(req.ClientSecret)))
			if err != nil {
				return nil, fmt.Errorf("encrypt client secret: %w", err)
			}
			tmplProvider.EncryptedClientSecret = enc
		}

		if err := tx.UpdateOAuthProvider(ctx, tmplProvider); err != nil {
			return nil, fmt.Errorf("update provider: %w", err)
		}
		return tmplProvider, nil
	}

	// No template provider — try auto-discovery + DCR.
	return h.autoDiscoverAndRegister(ctx, tx, server)
}

// autoDiscoverAndRegister runs MCP OAuth discovery and DCR for the server.
func (h *downstreamConnectHandler) autoDiscoverAndRegister(
	ctx context.Context,
	tx store.Store,
	server *store.DownstreamServer,
) (*store.OAuthProvider, error) {
	metadata, err := oauth.DiscoverOAuthServer(ctx, *server.URL)
	if err != nil {
		return nil, fmt.Errorf("OAuth discovery failed for %s: %w",
			server.Name, err)
	}

	if metadata.RegistrationEndpoint == "" {
		return nil, fmt.Errorf(
			"server %s does not support dynamic client registration; "+
				"configure OAuth provider manually", server.Name)
	}

	callbackURL := h.flowManager.CallbackURL()
	dcr, err := oauth.DynamicClientRegister(
		ctx, metadata.RegistrationEndpoint, callbackURL)
	if err != nil {
		return nil, fmt.Errorf("dynamic client registration failed: %w", err)
	}

	usePKCE := true
	if len(metadata.CodeChallengeMethods) > 0 {
		usePKCE = false
		for _, m := range metadata.CodeChallengeMethods {
			if m == "S256" {
				usePKCE = true
				break
			}
		}
	}

	now := time.Now().UTC()
	providerName := fmt.Sprintf("%s (auto)", server.Name)
	provider := store.OAuthProvider{
		Name:         providerName,
		AuthorizeURL: metadata.AuthorizationEndpoint,
		TokenURL:     metadata.TokenEndpoint,
		ClientID:     dcr.ClientID,
		UsePKCE:      usePKCE,
		Source:       "auto-discovery",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := tx.CreateOAuthProvider(ctx, &provider); err != nil {
		if errors.Is(err, store.ErrAlreadyExists) {
			existing, lookupErr := tx.GetOAuthProviderByName(ctx, providerName)
			if lookupErr != nil {
				return nil, fmt.Errorf("provider exists, lookup failed: %w", lookupErr)
			}
			return existing, nil
		}
		return nil, fmt.Errorf("create provider: %w", err)
	}
	return &provider, nil
}

// findOrCreateScope creates an auth scope or returns an existing one.
func (h *downstreamConnectHandler) findOrCreateScope(
	ctx context.Context,
	tx store.Store,
	name, providerID string,
) (*store.AuthScope, error) {
	now := time.Now().UTC()
	scope := store.AuthScope{
		Name:            name,
		Type:            "oauth2",
		OAuthProviderID: providerID,
		Source:          "api",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := tx.CreateAuthScope(ctx, &scope); err != nil {
		if errors.Is(err, store.ErrAlreadyExists) {
			existing, lookupErr := tx.GetAuthScopeByName(ctx, name)
			if lookupErr != nil {
				return nil, fmt.Errorf("scope exists, lookup failed: %w", lookupErr)
			}
			// Update provider link if changed.
			if existing.OAuthProviderID != providerID {
				existing.OAuthProviderID = providerID
				existing.UpdatedAt = now
				if err := tx.UpdateAuthScope(ctx, existing); err != nil {
					return nil, fmt.Errorf("update scope: %w", err)
				}
			}
			return existing, nil
		}
		return nil, fmt.Errorf("create scope: %w", err)
	}
	return &scope, nil
}

// findOrCreateRoute creates a route rule or returns an existing one.
func (h *downstreamConnectHandler) findOrCreateRoute(
	ctx context.Context,
	tx store.Store,
	workspaceID, serverID, scopeID string,
) (*store.RouteRule, error) {
	// Check for existing rule with same server + workspace.
	rules, err := tx.ListRouteRules(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list routes: %w", err)
	}
	for i := range rules {
		if rules[i].DownstreamServerID == serverID &&
			rules[i].WorkspaceID == workspaceID {
			// Update scope if changed.
			if rules[i].AuthScopeID != scopeID {
				rules[i].AuthScopeID = scopeID
				rules[i].UpdatedAt = time.Now().UTC()
				if err := tx.UpdateRouteRule(ctx, &rules[i]); err != nil {
					return nil, fmt.Errorf("update route: %w", err)
				}
			}
			return &rules[i], nil
		}
	}

	now := time.Now().UTC()
	rule := store.RouteRule{
		Priority:           100,
		WorkspaceID:        workspaceID,
		PathGlob:           "**",
		ToolMatch:          json.RawMessage(`["*"]`),
		DownstreamServerID: serverID,
		AuthScopeID:        scopeID,
		Policy:             "allow",
		LogLevel:           "info",
		Source:             "api",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := tx.CreateRouteRule(ctx, &rule); err != nil {
		return nil, fmt.Errorf("create route: %w", err)
	}
	return &rule, nil
}
