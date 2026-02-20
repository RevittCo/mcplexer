package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// cmdConnect bridges stdin/stdout to the MCPlexer daemon's Unix socket.
// It supports two modes:
//   - Direct: --socket=<path> dials the socket directly (native/Linux)
//   - Docker: --docker=<container> uses "docker exec" to reach the
//     socket inside the container (required on macOS Docker Desktop
//     where bind-mounted Unix sockets don't work)
func cmdConnect(args []string) error {
	var socketPath, container string
	for _, arg := range args {
		if len(arg) > 9 && arg[:9] == "--socket=" {
			socketPath = arg[9:]
		}
		if len(arg) > 9 && arg[:9] == "--docker=" {
			container = arg[9:]
		}
	}
	if socketPath == "" {
		socketPath = os.Getenv("MCPLEXER_SOCKET_PATH")
	}
	if container == "" {
		container = os.Getenv("MCPLEXER_DOCKER_CONTAINER")
	}

	if container != "" {
		return connectViaDocker(container, socketPath)
	}

	if socketPath == "" {
		return fmt.Errorf("socket path required: use --socket=<path> or --docker=<container>")
	}
	return connectDirect(socketPath)
}

// connectDirect dials the Unix socket directly.
func connectDirect(socketPath string) error {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return fmt.Errorf("connect to socket: %w", err)
	}
	defer func() { _ = conn.Close() }()

	ctx, cancel := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM,
	)
	defer cancel()

	readDone := make(chan error, 1)

	// socket -> stdout
	go func() {
		_, err := io.Copy(os.Stdout, conn)
		readDone <- err
	}()

	// stdin -> socket (inject CWD root, then half-close on EOF)
	go func() {
		injectAndBridge(os.Stdin, conn)
		if uc, ok := conn.(*net.UnixConn); ok {
			uc.CloseWrite() //nolint:errcheck
		}
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-readDone:
		return err
	}
}

// injectAndBridge reads the first line from src, injects the host CWD
// as a root into the MCP initialize message (if applicable), writes it
// to dst, then copies all remaining traffic verbatim.
func injectAndBridge(src io.Reader, dst io.Writer) {
	// Prefer explicit CWD from host (set by connectViaDocker) over
	// os.Getwd() which returns /app inside Docker containers.
	cwd := os.Getenv("MCPLEXER_CLIENT_CWD")
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	br := bufio.NewReaderSize(src, 1024*1024)

	line, err := br.ReadBytes('\n')
	if err != nil && len(line) == 0 {
		io.Copy(dst, br) //nolint:errcheck
		return
	}
	trimmed := bytes.TrimSuffix(line, []byte{'\n'})
	modified := maybeInjectRoots(trimmed, cwd)
	dst.Write(modified)     //nolint:errcheck
	dst.Write([]byte{'\n'}) //nolint:errcheck

	io.Copy(dst, br) //nolint:errcheck
}

// maybeInjectRoots parses a JSON-RPC line; if it is an "initialize"
// request without roots, it injects [{"uri":"file://<cwd>"}].
// Returns the original line unchanged on any error or non-initialize.
func maybeInjectRoots(line []byte, cwd string) []byte {
	if cwd == "" {
		return line
	}

	var msg struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id,omitempty"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params,omitempty"`
	}
	if err := json.Unmarshal(line, &msg); err != nil || msg.Method != "initialize" {
		return line
	}

	var params map[string]json.RawMessage
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return line
	}

	// Don't overwrite existing roots.
	if _, ok := params["roots"]; ok {
		return line
	}

	root := map[string]string{"uri": "file://" + cwd}
	rootsJSON, err := json.Marshal([]map[string]string{root})
	if err != nil {
		return line
	}
	params["roots"] = rootsJSON

	msg.Params, err = json.Marshal(params)
	if err != nil {
		return line
	}

	out, err := json.Marshal(msg)
	if err != nil {
		return line
	}
	return out
}

// connectViaDocker runs "docker exec -i <container> mcplexer connect
// --socket=<path>" and bridges stdin/stdout to the exec process.
func connectViaDocker(container, socketPath string) error {
	if socketPath == "" {
		socketPath = "/run/mcplexer.sock"
	}

	ctx, cancel := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM,
	)
	defer cancel()

	// Pass the host CWD to the inner connect process so it injects the
	// correct workspace root (not /app from inside the container).
	hostCWD, _ := os.Getwd()

	cmd := exec.CommandContext(ctx,
		"docker", "exec", "-i",
		"-e", "MCPLEXER_CLIENT_CWD="+hostCWD,
		container,
		"mcplexer", "connect", "--socket="+socketPath,
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Suppress exit errors from signal-based shutdown.
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("docker exec: %w", err)
	}
	return nil
}
