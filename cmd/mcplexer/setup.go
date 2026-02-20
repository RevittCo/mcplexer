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

func cmdSetup() error {
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

	// 2. Detect Claude Desktop config location
	configPath := claudeDesktopConfigPath()
	if configPath == "" {
		fmt.Println("Could not detect Claude Desktop config location.")
		fmt.Println("Add this to your Claude Desktop config manually:")
		printMCPEntry()
		return nil
	}

	fmt.Printf("\nClaude Desktop config: %s\n", configPath)

	// 3. Generate and display entry
	fmt.Println("\nMCP server entry to add:")
	printMCPEntry()

	// 4. Prompt user
	fmt.Print("\nWrite to Claude Desktop config? [Y/n] ")
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))

	if answer == "" || answer == "y" || answer == "yes" {
		if err := mergeClaudeDesktopConfig(configPath); err != nil {
			return fmt.Errorf("write config: %w", err)
		}
		fmt.Println("Claude Desktop config updated.")
		fmt.Println("Restart Claude Desktop to pick up the changes.")
	} else {
		fmt.Println("Skipped. Copy the JSON above into your Claude Desktop config.")
	}

	// 5. Open browser (best effort)
	fmt.Println("\nSetup complete. Open http://localhost:3333 to manage MCPlexer.")
	openBrowser("http://localhost:3333")
	return nil
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

func mcpServerEntry() map[string]any {
	exe, err := os.Executable()
	if err != nil {
		exe = "mcplexer"
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

func mergeClaudeDesktopConfig(path string) error {
	// Read existing config or start fresh
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

	// Merge mcpServers
	servers, ok := cfg["mcpServers"].(map[string]any)
	if !ok {
		servers = make(map[string]any)
	}
	servers["mx"] = mcpServerEntry()
	cfg["mcpServers"] = servers

	// Ensure parent directory exists
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
