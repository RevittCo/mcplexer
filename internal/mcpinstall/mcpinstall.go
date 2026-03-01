package mcpinstall

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// ClientID identifies a supported MCP client application.
type ClientID string

const (
	ClaudeDesktop ClientID = "claude_desktop"
	ClaudeCode    ClientID = "claude_code"
	Cursor        ClientID = "cursor"
	Windsurf      ClientID = "windsurf"
	Codex         ClientID = "codex"
	OpenCode      ClientID = "opencode"
	GeminiCLI     ClientID = "gemini_cli"
)

// ClientInfo describes a single MCP client and its install status.
type ClientInfo struct {
	ID         ClientID `json:"id"`
	Name       string   `json:"name"`
	ConfigPath string   `json:"config_path"`
	Detected   bool     `json:"detected"`   // parent dir exists
	Configured bool     `json:"configured"` // "mcplexer" key present (or legacy "mx")
}

// StatusResult is the response for the status endpoint.
type StatusResult struct {
	Clients     []ClientInfo   `json:"clients"`
	BinaryPath  string         `json:"binary_path"`
	ServerEntry map[string]any `json:"server_entry"`
}

// PreviewResult is the response for the preview endpoint.
type PreviewResult struct {
	ConfigPath string `json:"config_path"`
	Content    string `json:"content"`
}

type clientDef struct {
	ID         ClientID
	Name       string
	configPath func(home string) string
}

var knownClients = []clientDef{
	{ClaudeDesktop, "Claude Desktop", claudeDesktopPath},
	{ClaudeCode, "Claude Code", claudeCodePath},
	{Cursor, "Cursor", cursorPath},
	{Windsurf, "Windsurf", windsurfPath},
	{Codex, "Codex", codexPath},
	{OpenCode, "OpenCode", openCodePath},
	{GeminiCLI, "Gemini CLI", geminiPath},
}

const (
	serverName       = "mcplexer"
	legacyServerName = "mx"
)

// Manager handles MCP client installation and detection.
type Manager struct {
	home    string
	exePath string
}

// New creates a Manager, resolving the home directory and binary path.
func New() (*Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir: %w", err)
	}

	exe, err := os.Executable()
	if err != nil {
		exe = "mcplexer"
	}
	// Prefer stable installed binary path if it exists.
	stablePath := filepath.Join(home, ".mcplexer", "bin", "mcplexer")
	if _, err := os.Stat(stablePath); err == nil {
		exe = stablePath
	}

	return &Manager{home: home, exePath: exe}, nil
}

// Status returns detection and configuration info for all known clients.
func (m *Manager) Status() (*StatusResult, error) {
	var clients []ClientInfo
	for _, c := range knownClients {
		ci := m.clientInfo(c)
		clients = append(clients, ci)
	}
	return &StatusResult{
		Clients:     clients,
		BinaryPath:  m.exePath,
		ServerEntry: m.serverEntry(),
	}, nil
}

// Install writes the MCPlexer server entry into a client's config file.
func (m *Manager) Install(id ClientID) (*ClientInfo, error) {
	def, err := m.findClient(id)
	if err != nil {
		return nil, err
	}
	ci := m.clientInfo(def)
	if !ci.Detected {
		return nil, fmt.Errorf("client %q not detected (config dir does not exist)", id)
	}

	if err := m.mergeMCPConfig(ci.ConfigPath); err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}

	ci.Configured = true
	return &ci, nil
}

// Uninstall removes the MCPlexer server entry from a client's config file.
func (m *Manager) Uninstall(id ClientID) (*ClientInfo, error) {
	def, err := m.findClient(id)
	if err != nil {
		return nil, err
	}
	ci := m.clientInfo(def)
	if !ci.Configured {
		return nil, fmt.Errorf("client %q is not configured", id)
	}

	if err := m.removeMCPConfig(ci.ConfigPath); err != nil {
		return nil, fmt.Errorf("update config: %w", err)
	}

	ci.Configured = false
	return &ci, nil
}

// Preview returns what the config file would look like after install.
func (m *Manager) Preview(id ClientID) (*PreviewResult, error) {
	def, err := m.findClient(id)
	if err != nil {
		return nil, err
	}
	ci := m.clientInfo(def)
	if !ci.Detected {
		return nil, fmt.Errorf("client %q not detected", id)
	}

	merged, err := m.previewMerge(ci.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("preview: %w", err)
	}
	return &PreviewResult{ConfigPath: ci.ConfigPath, Content: merged}, nil
}

func (m *Manager) findClient(id ClientID) (clientDef, error) {
	for _, c := range knownClients {
		if c.ID == id {
			return c, nil
		}
	}
	return clientDef{}, fmt.Errorf("unknown client %q", id)
}

func (m *Manager) clientInfo(c clientDef) ClientInfo {
	path := c.configPath(m.home)
	detected := false
	configured := false

	if path != "" {
		dir := filepath.Dir(path)
		if _, err := os.Stat(dir); err == nil {
			detected = true
		}
		if detected {
			configured = m.hasConfigured(path)
		}
	}

	return ClientInfo{
		ID:         c.ID,
		Name:       c.Name,
		ConfigPath: path,
		Detected:   detected,
		Configured: configured,
	}
}

func (m *Manager) hasConfigured(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return false
	}
	servers, ok := cfg["mcpServers"].(map[string]any)
	if !ok {
		return false
	}
	if _, exists := servers[serverName]; exists {
		return true
	}
	_, exists := servers[legacyServerName]
	return exists
}

func (m *Manager) serverEntry() map[string]any {
	return map[string]any{
		"command": m.exePath,
		"args":    []string{"connect", "--socket=/tmp/mcplexer.sock"},
	}
}

// ServerEntryJSON returns the full mcpServers snippet as formatted JSON.
func (m *Manager) ServerEntryJSON() string {
	entry := map[string]any{
		"mcpServers": map[string]any{
			serverName: m.serverEntry(),
		},
	}
	out, _ := json.MarshalIndent(entry, "", "  ")
	return string(out)
}

func (m *Manager) mergeMCPConfig(path string) error {
	cfg, err := m.readOrCreateConfig(path)
	if err != nil {
		return err
	}

	servers, ok := cfg["mcpServers"].(map[string]any)
	if !ok {
		servers = make(map[string]any)
	}
	servers[serverName] = m.serverEntry()
	delete(servers, legacyServerName)
	cfg["mcpServers"] = servers

	return m.writeConfig(path, cfg)
}

func (m *Manager) removeMCPConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	servers, ok := cfg["mcpServers"].(map[string]any)
	if !ok {
		return nil
	}
	delete(servers, serverName)
	delete(servers, legacyServerName)
	cfg["mcpServers"] = servers

	return m.writeConfig(path, cfg)
}

func (m *Manager) previewMerge(path string) (string, error) {
	cfg, err := m.readOrCreateConfig(path)
	if err != nil {
		return "", err
	}

	servers, ok := cfg["mcpServers"].(map[string]any)
	if !ok {
		servers = make(map[string]any)
	}
	servers[serverName] = m.serverEntry()
	delete(servers, legacyServerName)
	cfg["mcpServers"] = servers

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (m *Manager) readOrCreateConfig(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]any), nil
		}
		return nil, err
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse existing config: %w", err)
	}
	return cfg, nil
}

func (m *Manager) writeConfig(path string, cfg map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0644)
}

// Path functions — one per supported client.

func claudeDesktopPath(home string) string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
	case "linux":
		return filepath.Join(home, ".config", "Claude", "claude_desktop_config.json")
	default:
		return ""
	}
}

func claudeCodePath(home string) string {
	return filepath.Join(home, ".claude", "settings.json")
}

func cursorPath(home string) string {
	return filepath.Join(home, ".cursor", "mcp.json")
}

func windsurfPath(home string) string {
	return filepath.Join(home, ".codeium", "windsurf", "mcp_config.json")
}

func codexPath(home string) string {
	return filepath.Join(home, ".codex", "mcp.json")
}

func openCodePath(home string) string {
	return filepath.Join(home, ".opencode", "mcp.json")
}

func geminiPath(home string) string {
	return filepath.Join(home, ".gemini", "settings.json")
}
