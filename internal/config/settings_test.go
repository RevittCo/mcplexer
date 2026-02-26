package config

import (
	"context"
	"encoding/json"
	"testing"
)

type mockSettingsStore struct {
	raw json.RawMessage
}

func (m *mockSettingsStore) GetSettings(_ context.Context) (json.RawMessage, error) {
	return m.raw, nil
}

func (m *mockSettingsStore) UpdateSettings(_ context.Context, data json.RawMessage) error {
	m.raw = data
	return nil
}

func TestDefaultSettings_CodexDynamicToolCompatEnabled(t *testing.T) {
	s := DefaultSettings()
	if !s.CodexDynamicToolCompat {
		t.Fatal("CodexDynamicToolCompat should default to true")
	}
}

func TestLoadSettings_MissingCompatFieldUsesDefaultTrue(t *testing.T) {
	st := &mockSettingsStore{
		raw: json.RawMessage(`{"slim_tools":false,"tools_cache_ttl_sec":30,"log_level":"warn"}`),
	}
	svc := NewSettingsService(st)

	got := svc.Load(context.Background())
	if !got.CodexDynamicToolCompat {
		t.Fatal("CodexDynamicToolCompat should remain true when missing from stored JSON")
	}
}

func TestApplyEnvOverrides_CodexDynamicToolCompat(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "false string", value: "false", want: false},
		{name: "zero", value: "0", want: false},
		{name: "no", value: "no", want: false},
		{name: "off", value: "off", want: false},
		{name: "true string", value: "true", want: true},
		{name: "one", value: "1", want: true},
		{name: "yes", value: "yes", want: true},
		{name: "on", value: "on", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("MCPLEXER_CODEX_DYNAMIC_TOOL_COMPAT", tt.value)
			got := applyEnvOverrides(DefaultSettings()).CodexDynamicToolCompat
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}
