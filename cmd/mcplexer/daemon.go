package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	pidFile = "mcplexer.pid"
	logFile = "mcplexer.log"
	dbFile  = "mcplexer.db"
)

// dataDir returns ~/.mcplexer, creating it if needed.
func dataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	dir := filepath.Join(home, ".mcplexer")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create data dir: %w", err)
	}
	return dir, nil
}

func cmdDaemon(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mcplexer daemon <start|stop|status|logs>")
	}

	switch args[0] {
	case "start":
		return daemonStart(args[1:])
	case "stop":
		return daemonStop()
	case "status":
		return daemonStatus()
	case "logs":
		return daemonLogs(args[1:])
	default:
		return fmt.Errorf("unknown daemon command: %s\nUsage: mcplexer daemon <start|stop|status|logs>", args[0])
	}
}

func daemonStart(args []string) error {
	dir, err := dataDir()
	if err != nil {
		return err
	}

	// Check if already running
	if pid, ok := readPID(dir); ok {
		if processAlive(pid) {
			return fmt.Errorf("daemon already running (PID %d)", pid)
		}
		// Stale PID file
		os.Remove(filepath.Join(dir, pidFile))
	}

	// Parse flags with defaults
	addr := ":3333"
	socketPath := "/tmp/mcplexer.sock"
	for _, arg := range args {
		if strings.HasPrefix(arg, "--addr=") {
			addr = arg[7:]
		}
		if strings.HasPrefix(arg, "--socket=") {
			socketPath = arg[9:]
		}
	}

	// Open log file
	logPath := filepath.Join(dir, logFile)
	lf, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer lf.Close()

	// Build serve command arguments
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	serveArgs := []string{
		"serve",
		"--mode=http",
		"--addr=" + addr,
		"--socket=" + socketPath,
	}

	cmd := exec.Command(exe, serveArgs...)
	cmd.Stderr = lf
	cmd.Stdout = lf
	// Detach from parent process group
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}

	pid := cmd.Process.Pid

	// Write PID file
	pidPath := filepath.Join(dir, pidFile)
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0644); err != nil {
		// Kill the child if we can't write PID
		cmd.Process.Kill()
		return fmt.Errorf("write pid file: %w", err)
	}

	// Release the child process so it survives our exit
	cmd.Process.Release()

	fmt.Printf("MCPlexer daemon started on %s (PID %d)\n", addr, pid)
	fmt.Printf("  Logs: %s\n", logPath)
	fmt.Printf("  DB:   %s\n", filepath.Join(dir, dbFile))
	fmt.Printf("  UI:   http://localhost%s\n", addr)
	return nil
}

func daemonStop() error {
	dir, err := dataDir()
	if err != nil {
		return err
	}

	pid, ok := readPID(dir)
	if !ok {
		return fmt.Errorf("daemon not running (no PID file)")
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		os.Remove(filepath.Join(dir, pidFile))
		return fmt.Errorf("find process %d: %w", pid, err)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		os.Remove(filepath.Join(dir, pidFile))
		return fmt.Errorf("send SIGTERM to PID %d: %w", pid, err)
	}

	// Wait for process to exit (up to 5 seconds)
	for i := 0; i < 50; i++ {
		if !processAlive(pid) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	os.Remove(filepath.Join(dir, pidFile))
	fmt.Println("MCPlexer daemon stopped")
	return nil
}

func daemonStatus() error {
	dir, err := dataDir()
	if err != nil {
		return err
	}

	pid, ok := readPID(dir)
	if !ok {
		fmt.Println("MCPlexer daemon: not running")
		return nil
	}

	if !processAlive(pid) {
		os.Remove(filepath.Join(dir, pidFile))
		fmt.Println("MCPlexer daemon: not running (stale PID file cleaned)")
		return nil
	}

	fmt.Printf("MCPlexer daemon: running (PID %d)\n", pid)
	return nil
}

func daemonLogs(args []string) error {
	dir, err := dataDir()
	if err != nil {
		return err
	}

	logPath := filepath.Join(dir, logFile)
	follow := false
	for _, arg := range args {
		if arg == "-f" || arg == "--follow" {
			follow = true
		}
	}

	tailArgs := []string{"-n", "100"}
	if follow {
		tailArgs = append(tailArgs, "-f")
	}
	tailArgs = append(tailArgs, logPath)

	cmd := exec.Command("tail", tailArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// readPID reads the PID from the PID file. Returns 0, false if not found.
func readPID(dir string) (int, bool) {
	data, err := os.ReadFile(filepath.Join(dir, pidFile))
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, false
	}
	return pid, true
}

// processAlive checks if a process with the given PID is still running.
func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 tests existence without sending a signal
	return proc.Signal(syscall.Signal(0)) == nil
}
