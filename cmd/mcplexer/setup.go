package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/revittco/mcplexer/internal/mcpinstall"
)

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
	mgr, err := mcpinstall.New()
	if err != nil {
		return fmt.Errorf("init install manager: %w", err)
	}

	status, err := mgr.Status()
	if err != nil {
		return fmt.Errorf("detect clients: %w", err)
	}

	var detected []mcpinstall.ClientInfo
	for _, c := range status.Clients {
		if c.Detected {
			detected = append(detected, c)
		}
	}

	if len(detected) == 0 {
		fmt.Println("\nNo MCP clients detected. Add this to your MCP client config manually:")
		fmt.Println(mgr.ServerEntryJSON())
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
				if _, err := mgr.Install(c.ID); err != nil {
					fmt.Printf("  ✗ %s: %v\n", c.Name, err)
				} else {
					fmt.Printf("  ✓ %s\n", c.Name)
				}
			}
			fmt.Println("Restart your MCP clients to pick up the changes.")
		} else {
			fmt.Println("Skipped. Add this to your MCP client config manually:")
			fmt.Println(mgr.ServerEntryJSON())
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
	exec.Command(cmd, url).Start() //nolint:errcheck
}
