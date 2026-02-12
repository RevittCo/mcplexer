package auth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/revitteth/mcplexer/internal/oauth"
	"github.com/revitteth/mcplexer/internal/secrets"
	"github.com/revitteth/mcplexer/internal/store"
)

// Injector resolves credentials from the secrets manager and provides them
// as environment variables or HTTP headers for downstream servers.
type Injector struct {
	secrets     *secrets.Manager
	flowManager *oauth.FlowManager   // may be nil
	store       store.AuthScopeStore // may be nil
}

// NewInjector creates a credential Injector.
func NewInjector(sm *secrets.Manager, fm *oauth.FlowManager, as store.AuthScopeStore) *Injector {
	return &Injector{secrets: sm, flowManager: fm, store: as}
}

// EnvForDownstream decrypts all secrets for the given auth scope and returns
// them as a string map suitable for use as environment variables.
// For OAuth2 scopes, it returns a valid access token instead.
func (inj *Injector) EnvForDownstream(ctx context.Context, authScopeID string) (map[string]string, error) {
	if authScopeID == "" {
		return nil, nil
	}

	// Check if this is an OAuth2 scope
	if inj.store != nil {
		scope, err := inj.store.GetAuthScope(ctx, authScopeID)
		if err == nil && scope.Type == "oauth2" && inj.flowManager != nil {
			token, err := inj.flowManager.GetValidToken(ctx, authScopeID)
			if err != nil {
				return nil, fmt.Errorf("get oauth token for scope %s: %w", authScopeID, err)
			}
			return map[string]string{"ACCESS_TOKEN": token}, nil
		}
	}

	// Existing env-based flow
	if inj.secrets == nil {
		return nil, nil
	}

	keys, err := inj.secrets.List(ctx, authScopeID)
	if err != nil {
		return nil, fmt.Errorf("list secrets for scope %s: %w", authScopeID, err)
	}

	env := make(map[string]string, len(keys))
	for _, key := range keys {
		val, err := inj.secrets.Get(ctx, authScopeID, key)
		if err != nil {
			return nil, fmt.Errorf("get secret %s/%s: %w", authScopeID, key, err)
		}
		env[key] = string(val)
	}
	return env, nil
}

// HeadersForDownstream decrypts all secrets for the given auth scope and
// returns them as HTTP headers (for future HTTP remote transports).
// For OAuth2 scopes, it returns an Authorization bearer header.
func (inj *Injector) HeadersForDownstream(ctx context.Context, authScopeID string) (http.Header, error) {
	if authScopeID == "" {
		return nil, nil
	}

	// Check if this is an OAuth2 scope
	if inj.store != nil {
		scope, err := inj.store.GetAuthScope(ctx, authScopeID)
		if err == nil && scope.Type == "oauth2" && inj.flowManager != nil {
			token, err := inj.flowManager.GetValidToken(ctx, authScopeID)
			if err != nil {
				return nil, fmt.Errorf("get oauth token for scope %s: %w", authScopeID, err)
			}
			h := make(http.Header)
			h.Set("Authorization", "Bearer "+token)
			return h, nil
		}
	}

	// Existing env-based flow
	if inj.secrets == nil {
		return nil, nil
	}

	keys, err := inj.secrets.List(ctx, authScopeID)
	if err != nil {
		return nil, fmt.Errorf("list secrets for scope %s: %w", authScopeID, err)
	}

	headers := make(http.Header, len(keys))
	for _, key := range keys {
		val, err := inj.secrets.Get(ctx, authScopeID, key)
		if err != nil {
			return nil, fmt.Errorf("get secret %s/%s: %w", authScopeID, key, err)
		}
		headers.Set(key, string(val))
	}
	return headers, nil
}
