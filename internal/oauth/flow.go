package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/revittco/mcplexer/internal/secrets"
	"github.com/revittco/mcplexer/internal/store"
)

// FlowManager orchestrates OAuth2 authorization code flows.
type FlowManager struct {
	store       store.Store
	encryptor   *secrets.AgeEncryptor
	stateStore  *StateStore
	externalURL string
}

// NewFlowManager creates a FlowManager.
func NewFlowManager(s store.Store, enc *secrets.AgeEncryptor, externalURL string) *FlowManager {
	return &FlowManager{
		store:       s,
		encryptor:   enc,
		stateStore:  NewStateStore(),
		externalURL: strings.TrimRight(externalURL, "/"),
	}
}

// AuthorizeURL builds the OAuth2 authorization URL for an auth scope.
func (fm *FlowManager) AuthorizeURL(ctx context.Context, authScopeID string) (string, error) {
	scope, err := fm.store.GetAuthScope(ctx, authScopeID)
	if err != nil {
		return "", fmt.Errorf("get auth scope: %w", err)
	}
	if scope.OAuthProviderID == "" {
		return "", fmt.Errorf("auth scope %q has no oauth provider", authScopeID)
	}

	provider, err := fm.store.GetOAuthProvider(ctx, scope.OAuthProviderID)
	if err != nil {
		return "", fmt.Errorf("get oauth provider: %w", err)
	}

	var codeVerifier string
	if provider.UsePKCE {
		codeVerifier, err = GenerateCodeVerifier()
		if err != nil {
			return "", fmt.Errorf("generate pkce verifier: %w", err)
		}
	}

	state, err := fm.stateStore.Create(authScopeID, codeVerifier)
	if err != nil {
		return "", fmt.Errorf("create oauth state: %w", err)
	}
	return fm.buildAuthorizeURL(provider, state, codeVerifier)
}

func (fm *FlowManager) buildAuthorizeURL(
	p *store.OAuthProvider, state, codeVerifier string,
) (string, error) {
	u, err := parseOAuthURL(p.AuthorizeURL)
	if err != nil {
		return "", fmt.Errorf("invalid authorize url: %w", err)
	}

	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", p.ClientID)
	q.Set("redirect_uri", fm.CallbackURL())
	q.Set("state", state)

	var scopes []string
	if len(p.Scopes) > 0 {
		if err := json.Unmarshal(p.Scopes, &scopes); err != nil {
			return "", fmt.Errorf("parse provider scopes: %w", err)
		}
	}
	if len(scopes) > 0 {
		q.Set("scope", strings.Join(scopes, " "))
	}

	if codeVerifier != "" {
		q.Set("code_challenge", CodeChallenge(codeVerifier))
		q.Set("code_challenge_method", "S256")
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}

func parseOAuthURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("url must use http or https")
	}
	if u.Host == "" {
		return nil, fmt.Errorf("url must include host")
	}
	return u, nil
}

// CallbackURL returns the full OAuth callback URL for this instance.
func (fm *FlowManager) CallbackURL() string {
	return fm.externalURL + "/api/v1/oauth/callback"
}

// HandleCallback processes the OAuth2 callback, exchanging the code for tokens.
func (fm *FlowManager) HandleCallback(
	ctx context.Context, state, code string,
) (authScopeID string, err error) {
	entry, ok := fm.stateStore.Validate(state)
	if !ok {
		return "", fmt.Errorf("invalid or expired oauth state")
	}

	scope, err := fm.store.GetAuthScope(ctx, entry.AuthScopeID)
	if err != nil {
		return "", fmt.Errorf("get auth scope: %w", err)
	}

	provider, err := fm.store.GetOAuthProvider(ctx, scope.OAuthProviderID)
	if err != nil {
		return "", fmt.Errorf("get oauth provider: %w", err)
	}

	clientSecret, err := fm.decryptClientSecret(provider)
	if err != nil {
		return "", err
	}

	td, err := fm.exchangeCode(ctx, provider, clientSecret, code, entry.CodeVerifier)
	if err != nil {
		return "", err
	}

	encrypted, err := fm.encryptTokenData(td)
	if err != nil {
		return "", err
	}

	if err := fm.store.UpdateAuthScopeTokenData(ctx, entry.AuthScopeID, encrypted); err != nil {
		return "", fmt.Errorf("store token data: %w", err)
	}
	return entry.AuthScopeID, nil
}
