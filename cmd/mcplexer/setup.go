package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type mcpClient struct {
	Name       string
	configPath func() string
}

var knownMCPClients = []mcpClient{
	{"Claude Desktop", claudeDesktopConfigPath},
	{"Claude Code", claudeCodeConfigPath},
	{"Cursor", cursorConfigPath},
	{"Windsurf", windsurfConfigPath},
	{"Codex", codexConfigPath},
	{"OpenCode", openCodeConfigPath},
	{"Gemini CLI", geminiConfigPath},
}

func cmdSetup() error {
	reader := bufio.NewReader(os.Stdin)

	// 1. Start daemon if not running
	dir, err := dataDir()
	if err != nil {
		return err
	}
	pid, ok := readPID(dir)
	if !ok || !processAlive(pid) {
		fmt.Println("Starting MCPlexer daemon...")
		if err := daemonStart(nil); err != nil {
			return fmt.Errorf("start daemon: %w", err)
		}
	} else {
		fmt.Printf("MCPlexer daemon already running (PID %d)\n", pid)
	}

	// 2. Detect installed MCP clients
	var detected []mcpClient
	for _, c := range knownMCPClients {
		if mcpClientInstalled(c) {
			detected = append(detected, c)
		}
	}

	if len(detected) == 0 {
		fmt.Println("\nNo MCP clients detected. Add this to your MCP client config manually:")
		printMCPEntry()
	} else {
		fmt.Println("\nDetected MCP clients:")
		for _, c := range detected {
			fmt.Printf("  • %s\n", c.Name)
		}

		fmt.Print("\nConfigure MCPlexer for these clients? [Y/n] ")
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))

		if answer == "" || answer == "y" || answer == "yes" {
			for _, c := range detected {
				if err := mergeMCPConfig(c.configPath()); err != nil {
					fmt.Printf("  ✗ %s: %v\n", c.Name, err)
				} else {
					fmt.Printf("  ✓ %s\n", c.Name)
				}
			}
			fmt.Println("Restart your MCP clients to pick up the changes.")
		} else {
			fmt.Println("Skipped. Add this to your MCP client config manually:")
			printMCPEntry()
		}
	}

	// 3. Offer launchd installation (macOS only)
	if runtime.GOOS == "darwin" && !launchdInstalled() {
		fmt.Print("\nInstall as launchd service (survives reboots)? [Y/n] ")
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer == "" || answer == "y" || answer == "yes" {
			daemonStop() //nolint:errcheck

			exe, err := os.Executable()
			if err != nil {
				return fmt.Errorf("resolve executable: %w", err)
			}
			if err := installLaunchd(exe, "127.0.0.1:3333", "/tmp/mcplexer.sock"); err != nil {
				return fmt.Errorf("install launchd: %w", err)
			}
			fmt.Println("Launchd agent installed. MCPlexer will start automatically on boot.")
		}
	}

	// 4. Open browser (best effort)
	fmt.Println("\nSetup complete. Open http://localhost:3333 to manage MCPlexer.")
	openBrowser("http://localhost:3333")
	return nil
}

func mcpClientInstalled(c mcpClient) bool {
	p := c.configPath()
	if p == "" {
		return false
	}
	dir := filepath.Dir(p)
	_, err := os.Stat(dir)
	return err == nil
}

func claudeDesktopConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
	case "linux":
		return filepath.Join(home, ".config", "Claude", "claude_desktop_config.json")
	default:
		return ""
	}
}

func claudeCodeConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "settings.json")
}

func cursorConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cursor", "mcp.json")
}

func windsurfConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".codeium", "windsurf", "mcp_config.json")
}

func codexConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".codex", "mcp.json")
}

func openCodeConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".opencode", "mcp.json")
}

func geminiConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".gemini", "settings.json")
}

func mcpServerEntry() map[string]any {
	exe, err := os.Executable()
	if err != nil {
		exe = "mcplexer"
	}
	// Prefer stable launchd binary path if it exists
	home, homeErr := os.UserHomeDir()
	if homeErr == nil {
		stablePath := filepath.Join(home, ".mcplexer", "bin", "mcplexer")
		if _, err := os.Stat(stablePath); err == nil {
			exe = stablePath
		}
	}
	return map[string]any{
		"command": exe,
		"args":    []string{"connect", "--socket=/tmp/mcplexer.sock"},
	}
}

func printMCPEntry() {
	entry := map[string]any{
		"mcpServers": map[string]any{
			"mx": mcpServerEntry(),
		},
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(entry)
}

func mergeMCPConfig(path string) error {
	var cfg map[string]any
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		cfg = make(map[string]any)
	} else {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("parse existing config: %w", err)
		}
	}

	servers, ok := cfg["mcpServers"].(map[string]any)
	if !ok {
		servers = make(map[string]any)
	}
	servers["mx"] = mcpServerEntry()
	cfg["mcpServers"] = servers

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0644)
}

func openBrowser(url string) {
	var cmd string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "linux":
		cmd = "xdg-open"
	default:
		return
	}
	exec.Command(cmd, url).Start()
}
