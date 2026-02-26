package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/revittco/mcplexer/internal/store"
)

// Settings holds user-configurable runtime settings.
type Settings struct {
	SlimTools                bool              `json:"slim_tools"`
	ToolsCacheTTLSec         int               `json:"tools_cache_ttl_sec"`
	LogLevel                 string            `json:"log_level"`
	CodexDynamicToolCompat   bool              `json:"codex_dynamic_tool_compat"`
	ToolDescriptionOverrides map[string]string `json:"tool_description_overrides"`
}

// DefaultSettings returns settings with sensible defaults.
func DefaultSettings() Settings {
	return Settings{
		SlimTools:                true,
		ToolsCacheTTLSec:         15,
		LogLevel:                 "info",
		CodexDynamicToolCompat:   true,
		ToolDescriptionOverrides: map[string]string{},
	}
}

// BuiltinToolDefaults returns the hardcoded descriptions for all built-in tools.
func BuiltinToolDefaults() map[string]string {
	return map[string]string{
		"mcpx__search_tools":           "Search for available tools across all connected MCP servers. Use this to discover tools that aren't listed by default. Returns tool names, descriptions, and input schemas.",
		"mcpx__load_tools":             "Load tools into the active session by name or glob pattern (e.g. \"github__*\"). Loaded tools appear in tools/list. Use search_tools first to discover available tools.",
		"mcpx__unload_tools":           "Remove tools from the active session. Accepts tool names or glob patterns.",
		"mcpx__flush_cache":            "Flush the tool call cache to force fresh data on subsequent calls. Use this when you suspect cached data is stale or after making changes that should be reflected immediately. Optionally specify a server_id to flush only that server's cache. Note: you can also pass `_cache_bust: true` as an argument to any individual tool call to bypass the cache for that specific request without flushing the entire cache.",
		"mcpx__list_pending_approvals": "List pending tool call approvals waiting for review. Returns approval IDs, tool names, justifications, and requesting agent info. Your own pending requests are excluded.",
		"mcpx__approve_tool_call":      "Approve a pending tool call request. You cannot approve your own requests.",
		"mcpx__deny_tool_call":         "Deny a pending tool call request. You cannot deny your own requests. A reason is required.",
	}
}

// SettingsService loads and saves settings, merging DB values with defaults
// and env var overrides.
type SettingsService struct {
	store store.SettingsStore
}

// NewSettingsService creates a SettingsService.
func NewSettingsService(s store.SettingsStore) *SettingsService {
	return &SettingsService{store: s}
}

// Load reads settings from the DB, merges with defaults, and applies env
// var overrides. Env vars take precedence over DB values.
func (s *SettingsService) Load(ctx context.Context) Settings {
	settings := DefaultSettings()

	raw, err := s.store.GetSettings(ctx)
	if err != nil {
		slog.Warn("failed to load settings from DB, using defaults", "error", err)
		return applyEnvOverrides(settings)
	}

	if len(raw) > 0 && string(raw) != "{}" {
		if err := json.Unmarshal(raw, &settings); err != nil {
			slog.Warn("failed to parse settings JSON, using defaults", "error", err)
			settings = DefaultSettings()
		}
	}

	// Ensure the map is never nil.
	if settings.ToolDescriptionOverrides == nil {
		settings.ToolDescriptionOverrides = map[string]string{}
	}

	return applyEnvOverrides(settings)
}

// Save validates and persists settings to the DB.
func (s *SettingsService) Save(ctx context.Context, settings Settings) error {
	if err := validateSettings(settings); err != nil {
		return err
	}

	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	return s.store.UpdateSettings(ctx, data)
}

func validateSettings(s Settings) error {
	if s.ToolsCacheTTLSec < 0 || s.ToolsCacheTTLSec > 300 {
		return fmt.Errorf("tools_cache_ttl_sec must be between 0 and 300")
	}

	validLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true,
	}
	if !validLevels[strings.ToLower(s.LogLevel)] {
		return fmt.Errorf("log_level must be one of: debug, info, warn, error")
	}

	return nil
}

// applyEnvOverrides lets env vars take precedence over DB values.
func applyEnvOverrides(s Settings) Settings {
	if v := os.Getenv("MCPLEXER_SLIM_TOOLS"); v != "" {
		s.SlimTools = envBoolDefaultTrue(v)
	}
	if v := os.Getenv("MCPLEXER_LOG_LEVEL"); v != "" {
		s.LogLevel = strings.ToLower(v)
	}
	if v := os.Getenv("MCPLEXER_CODEX_DYNAMIC_TOOL_COMPAT"); v != "" {
		s.CodexDynamicToolCompat = envBoolDefaultTrue(v)
	}
	return s
}

func envBoolDefaultTrue(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}
