package gateway

import (
	"encoding/json"
	"testing"
)

func TestGitHubScopePolicy_OrgAllowlist(t *testing.T) {
	p, err := newGitHubScopePolicy(json.RawMessage(`[
		"acme"
	]`), nil)
	if err != nil {
		t.Fatalf("new policy: %v", err)
	}

	if err := p.Enforce(json.RawMessage(`{"owner":"acme","repo":"mcplexer"}`)); err != nil {
		t.Fatalf("expected allow, got %v", err)
	}
	if err := p.Enforce(json.RawMessage(`{"owner":"evil","repo":"x"}`)); err == nil {
		t.Fatal("expected deny for non-allowlisted org")
	}
}

func TestGitHubScopePolicy_ExtractsFromURLAndQuery(t *testing.T) {
	p, err := newGitHubScopePolicy(nil, json.RawMessage(`[
		"acme/mcplexer"
	]`))
	if err != nil {
		t.Fatalf("new policy: %v", err)
	}

	if err := p.Enforce(json.RawMessage(`{"url":"https://github.com/acme/mcplexer/issues/1"}`)); err != nil {
		t.Fatalf("expected allow from URL extraction, got %v", err)
	}
	if err := p.Enforce(json.RawMessage(`{"query":"is:issue repo:evil/private bug"}`)); err == nil {
		t.Fatal("expected deny from query extraction")
	}
}

func TestGitHubScopePolicy_InvalidConfig(t *testing.T) {
	_, err := newGitHubScopePolicy(json.RawMessage(`{"not":"an array"}`), nil)
	if err == nil {
		t.Fatal("expected parse error")
	}
}
