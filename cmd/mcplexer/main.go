package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"

	"github.com/revittco/mcplexer/internal/api"
	"github.com/revittco/mcplexer/internal/approval"
	"github.com/revittco/mcplexer/internal/audit"
	"github.com/revittco/mcplexer/internal/auth"
	"github.com/revittco/mcplexer/internal/config"
	"github.com/revittco/mcplexer/internal/control"
	"github.com/revittco/mcplexer/internal/downstream"
	"github.com/revittco/mcplexer/internal/gateway"
	"github.com/revittco/mcplexer/internal/oauth"
	"github.com/revittco/mcplexer/internal/routing"
	"github.com/revittco/mcplexer/internal/secrets"
	"github.com/revittco/mcplexer/internal/store/sqlite"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "mcplexer: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Parse subcommand from os.Args
	subcmd := "serve"
	args := os.Args[1:]
	if len(args) > 0 && args[0] != "" && args[0][0] != '-' {
		subcmd = args[0]
		args = args[1:]
	}

	switch subcmd {
	case "serve":
		return cmdServe(args)
	case "connect":
		return cmdConnect(args)
	case "init":
		return cmdInit()
	case "status":
		return cmdStatus()
	case "dry-run":
		return cmdDryRun(args)
	case "secret":
		return cmdSecret(args)
	case "daemon":
		return cmdDaemon(args)
	case "setup":
		return cmdSetup()
	case "control-server":
		return cmdControlServer()
	default:
		return fmt.Errorf("unknown command: %s\nUsage: mcplexer [serve|connect|init|status|dry-run|secret|daemon|setup|control-server]", subcmd)
	}
}

func cmdServe(args []string) error {
	ctx, cancel := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM,
	)
	defer cancel()

	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	applyFlags(cfg, args)

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	slog.SetDefault(logger)

	db, err := sqlite.New(ctx, cfg.DBDSN)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	// Seed default "Global" workspace on first run
	if err := config.SeedDefaultWorkspaces(ctx, db); err != nil {
		return fmt.Errorf("seed workspace defaults: %w", err)
	}

	// Seed default OAuth providers from built-in templates on first run
	if err := config.SeedDefaultOAuthProviders(ctx, db); err != nil {
		return fmt.Errorf("seed oauth defaults: %w", err)
	}

	// Seed default downstream servers (Linear, ClickUp, GitHub, SQLite, Postgres)
	if err := config.SeedDefaultDownstreamServers(ctx, db); err != nil {
		return fmt.Errorf("seed downstream defaults: %w", err)
	}

	// Seed global deny-all route rule for deny-first routing
	if err := config.SeedDefaultRouteRules(ctx, db); err != nil {
		return fmt.Errorf("seed route defaults: %w", err)
	}

	// Load YAML config into store if file exists
	if cfg.ConfigFile != "" {
		if _, err := os.Stat(cfg.ConfigFile); err == nil {
			fileCfg, err := config.LoadFile(cfg.ConfigFile)
			if err != nil {
				return fmt.Errorf("load config file: %w", err)
			}
			if err := config.Apply(ctx, db, fileCfg); err != nil {
				return fmt.Errorf("apply config: %w", err)
			}
			logger.Info("loaded config", "file", cfg.ConfigFile)
		}
	}

	cfgSvc := config.NewService(db)

	switch cfg.Mode {
	case "stdio":
		logger.Info("starting in stdio mode")
		return runStdio(ctx, cfg, db)
	case "http":
		if cfg.SocketPath != "" {
			return runHTTPAndSocket(ctx, cfg, db, cfgSvc)
		}
		return runHTTP(ctx, cfg, db, cfgSvc)
	default:
		return fmt.Errorf("unknown mode: %s", cfg.Mode)
	}
}

func runHTTP(ctx context.Context, cfg *Config, db *sqlite.DB, cfgSvc *config.Service) error {
	authInj, fm, enc, err := buildAuthInjector(cfg, db)
	if err != nil {
		return fmt.Errorf("build auth injector: %w", err)
	}

	engine := routing.NewEngine(db)
	manager := downstream.NewManager(db, authInj)
	defer manager.Shutdown(ctx) //nolint:errcheck

	approvalBus := approval.NewBus()
	approvalMgr := approval.NewManager(db, approvalBus)
	approvalMgr.ExpireStale(ctx)
	defer approvalMgr.Shutdown()

	auditBus := audit.NewBus()
	router := api.NewRouter(api.RouterDeps{
		Store:           db,
		ConfigSvc:       cfgSvc,
		Engine:          engine,
		Manager:         manager,
		FlowManager:     fm,
		Encryptor:       enc,
		AuditBus:        auditBus,
		ApprovalManager: approvalMgr,
		ApprovalBus:     approvalBus,
	})

	srv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: router,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("http server listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutting down http server")
		return srv.Shutdown(context.Background())
	case err := <-errCh:
		return err
	}
}

func runStdio(ctx context.Context, cfg *Config, db *sqlite.DB) error {
	authInj, _, _, err := buildAuthInjector(cfg, db)
	if err != nil {
		return fmt.Errorf("build auth injector: %w", err)
	}

	engine := routing.NewEngine(db)
	manager := downstream.NewManager(db, authInj)
	defer manager.Shutdown(ctx) //nolint:errcheck

	approvalBus := approval.NewBus()
	approvalMgr := approval.NewManager(db, approvalBus)
	approvalMgr.ExpireStale(ctx)
	defer approvalMgr.Shutdown()

	auditor := audit.NewLogger(db, db, nil)
	gw := gateway.NewServer(db, engine, manager, auditor, gateway.TransportStdio,
		gateway.WithApprovals(approvalMgr))
	return gw.RunStdio(ctx)
}

// buildAuthInjector creates an auth.Injector and optionally an oauth.FlowManager.
// Returns nil injector (safe to pass) if no key path is set.
func buildAuthInjector(cfg *Config, db *sqlite.DB) (*auth.Injector, *oauth.FlowManager, *secrets.AgeEncryptor, error) {
	var enc *secrets.AgeEncryptor
	var sm *secrets.Manager
	var fm *oauth.FlowManager

	if cfg.AgeKeyPath != "" {
		var err error
		enc, err = secrets.NewAgeEncryptor(cfg.AgeKeyPath)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("create age encryptor: %w", err)
		}
		sm = secrets.NewManager(db, enc)
	}

	// Auto-generate a persistent age key alongside the DB if none configured.
	if enc == nil {
		keyPath := cfg.DBDSN + ".age"
		var err error
		enc, err = secrets.EnsureKeyFile(keyPath)
		if err != nil {
			slog.Warn("failed to create auto key file, falling back to ephemeral",
				"path", keyPath, "error", err)
			enc, _ = secrets.NewEphemeralEncryptor()
		} else {
			sm = secrets.NewManager(db, enc)
			slog.Info("using auto-generated age key", "path", keyPath)
		}
	}

	externalURL := cfg.ExternalURL
	if externalURL == "" && cfg.Mode == "http" {
		externalURL = "http://localhost" + cfg.HTTPAddr
	}

	if externalURL != "" && enc != nil {
		fm = oauth.NewFlowManager(db, enc, externalURL)
	}

	return auth.NewInjector(sm, fm, db), fm, enc, nil
}

func cmdInit() error {
	ctx := context.Background()

	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Create database
	db, err := sqlite.New(ctx, cfg.DBDSN)
	if err != nil {
		return fmt.Errorf("create database: %w", err)
	}
	db.Close()
	fmt.Printf("Database created: %s\n", cfg.DBDSN)

	// Create default config if not exists
	if _, err := os.Stat(cfg.ConfigFile); os.IsNotExist(err) {
		defaultCfg := `# MCPlexer Configuration
# OAuth providers are seeded from built-in templates on first startup.
# Configure workspaces, servers, and routes via the web UI.

oauth_providers: []
workspaces: []
auth_scopes: []
downstream_servers: []
route_rules: []
`
		if err := os.WriteFile(cfg.ConfigFile, []byte(defaultCfg), 0644); err != nil {
			return fmt.Errorf("write config: %w", err)
		}
		fmt.Printf("Config file created: %s\n", cfg.ConfigFile)
	} else {
		fmt.Printf("Config file already exists: %s\n", cfg.ConfigFile)
	}

	return nil
}

func cmdStatus() error {
	ctx := context.Background()

	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	db, err := sqlite.New(ctx, cfg.DBDSN)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	workspaces, err := db.ListWorkspaces(ctx)
	if err != nil {
		return fmt.Errorf("list workspaces: %w", err)
	}

	downstreams, err := db.ListDownstreamServers(ctx)
	if err != nil {
		return fmt.Errorf("list downstreams: %w", err)
	}

	sessions, err := db.ListActiveSessions(ctx)
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	scopes, err := db.ListAuthScopes(ctx)
	if err != nil {
		return fmt.Errorf("list auth scopes: %w", err)
	}

	fmt.Printf("MCPlexer Status (db: %s)\n", cfg.DBDSN)
	fmt.Printf("  Workspaces:         %d\n", len(workspaces))
	fmt.Printf("  Downstream servers: %d\n", len(downstreams))
	fmt.Printf("  Auth scopes:        %d\n", len(scopes))
	fmt.Printf("  Active sessions:    %d\n", len(sessions))

	return nil
}

func cmdDryRun(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: mcplexer dry-run <workspace-id> <tool-name>")
	}
	workspaceID := args[0]
	toolName := args[1]

	ctx := context.Background()
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	db, err := sqlite.New(ctx, cfg.DBDSN)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	ws, err := db.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("workspace %q not found: %w", workspaceID, err)
	}

	rules, err := db.ListRouteRules(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("list rules: %w", err)
	}

	fmt.Printf("Dry-run: workspace=%s tool=%s\n", workspaceID, toolName)
	fmt.Printf("  Default policy: %s\n", ws.DefaultPolicy)
	fmt.Printf("  Matching against %d rules\n\n", len(rules))

	for _, rule := range rules {
		fmt.Printf("  Rule %s (priority=%d, policy=%s)\n", rule.ID, rule.Priority, rule.Policy)
		fmt.Printf("    downstream=%s auth_scope=%s\n", rule.DownstreamServerID, rule.AuthScopeID)
	}

	if len(rules) == 0 {
		fmt.Printf("  No matching rules. Default policy: %s\n", ws.DefaultPolicy)
	}

	return nil
}

func cmdSecret(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: mcplexer secret <put|get|list|delete> [args...]")
	}

	ctx := context.Background()
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg.AgeKeyPath == "" {
		return fmt.Errorf("MCPLEXER_AGE_KEY must be set to manage secrets")
	}

	db, err := sqlite.New(ctx, cfg.DBDSN)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	enc, err := secrets.NewAgeEncryptor(cfg.AgeKeyPath)
	if err != nil {
		return fmt.Errorf("create encryptor: %w", err)
	}
	sm := secrets.NewManager(db, enc)

	sub := args[0]
	rest := args[1:]

	switch sub {
	case "put":
		if len(rest) < 3 {
			return fmt.Errorf("usage: mcplexer secret put <scope-id> <key> <value>")
		}
		if err := sm.Put(ctx, rest[0], rest[1], []byte(rest[2])); err != nil {
			return fmt.Errorf("put secret: %w", err)
		}
		fmt.Printf("Secret %q set on auth scope %q\n", rest[1], rest[0])

	case "get":
		if len(rest) < 2 {
			return fmt.Errorf("usage: mcplexer secret get <scope-id> <key>")
		}
		val, err := sm.Get(ctx, rest[0], rest[1])
		if err != nil {
			return fmt.Errorf("get secret: %w", err)
		}
		fmt.Print(string(val))

	case "list":
		if len(rest) < 1 {
			return fmt.Errorf("usage: mcplexer secret list <scope-id>")
		}
		keys, err := sm.List(ctx, rest[0])
		if err != nil {
			return fmt.Errorf("list secrets: %w", err)
		}
		for _, k := range keys {
			fmt.Println(k)
		}

	case "delete":
		if len(rest) < 2 {
			return fmt.Errorf("usage: mcplexer secret delete <scope-id> <key>")
		}
		if err := sm.Delete(ctx, rest[0], rest[1]); err != nil {
			return fmt.Errorf("delete secret: %w", err)
		}
		fmt.Printf("Secret %q deleted from auth scope %q\n", rest[1], rest[0])

	default:
		return fmt.Errorf("unknown secret command: %s\nUsage: mcplexer secret <put|get|list|delete>", sub)
	}

	return nil
}

func cmdControlServer() error {
	ctx, cancel := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM,
	)
	defer cancel()

	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	slog.SetDefault(logger)

	db, err := sqlite.New(ctx, cfg.DBDSN)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	readOnly := os.Getenv("MCPLEXER_CONTROL_READONLY") != "false"
	srv := control.New(db, readOnly)
	return srv.RunStdio(ctx)
}

// applyFlags parses --mode=X flags from the args list.
func applyFlags(cfg *Config, args []string) {
	for _, arg := range args {
		if len(arg) > 7 && arg[:7] == "--mode=" {
			cfg.Mode = arg[7:]
		}
		if len(arg) > 7 && arg[:7] == "--addr=" {
			cfg.HTTPAddr = arg[7:]
		}
		if len(arg) > 9 && arg[:9] == "--socket=" {
			cfg.SocketPath = arg[9:]
		}
	}
}

// runHTTPAndSocket runs both the HTTP server and Unix socket listener
// concurrently using an errgroup. They share the same store, routing
// engine, and downstream manager.
func runHTTPAndSocket(ctx context.Context, cfg *Config, db *sqlite.DB, cfgSvc *config.Service) error {
	authInj, fm, enc, err := buildAuthInjector(cfg, db)
	if err != nil {
		return fmt.Errorf("build auth injector: %w", err)
	}

	engine := routing.NewEngine(db)
	manager := downstream.NewManager(db, authInj)
	defer manager.Shutdown(ctx) //nolint:errcheck

	approvalBus := approval.NewBus()
	approvalMgr := approval.NewManager(db, approvalBus)
	approvalMgr.ExpireStale(ctx)
	defer approvalMgr.Shutdown()

	auditBus := audit.NewBus()
	auditor := audit.NewLogger(db, db, auditBus)
	g, ctx := errgroup.WithContext(ctx)

	// HTTP server
	g.Go(func() error {
		router := api.NewRouter(api.RouterDeps{
			Store:           db,
			ConfigSvc:       cfgSvc,
			Engine:          engine,
			Manager:         manager,
			FlowManager:     fm,
			Encryptor:       enc,
			AuditBus:        auditBus,
			ApprovalManager: approvalMgr,
			ApprovalBus:     approvalBus,
		})
		srv := &http.Server{Addr: cfg.HTTPAddr, Handler: router}
		errCh := make(chan error, 1)
		go func() {
			slog.Info("http server listening", "addr", cfg.HTTPAddr)
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- err
			}
			close(errCh)
		}()
		select {
		case <-ctx.Done():
			return srv.Shutdown(context.Background())
		case err := <-errCh:
			return err
		}
	})

	// Unix socket listener
	g.Go(func() error {
		return runSocket(ctx, cfg.SocketPath, db, engine, manager, auditor, approvalMgr)
	})

	return g.Wait()
}

// runSocket listens on a Unix domain socket and spawns a fresh gateway
// session for each accepted connection.
func runSocket(
	ctx context.Context,
	path string,
	s *sqlite.DB,
	engine *routing.Engine,
	manager *downstream.Manager,
	auditor *audit.Logger,
	approvalMgr *approval.Manager,
) error {
	// Clean up stale socket file
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale socket: %w", err)
	}

	ln, err := net.Listen("unix", path)
	if err != nil {
		return fmt.Errorf("listen unix: %w", err)
	}
	defer ln.Close()

	// Restrict socket to owner only (best-effort; fails on some
	// Docker volume mounts but socket is still usable)
	if err := os.Chmod(path, 0600); err != nil {
		slog.Warn("chmod socket failed (continuing)", "path", path, "err", err)
	}

	slog.Info("unix socket listening", "path", path)

	// Close listener when context is cancelled to unblock Accept
	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil // clean shutdown
			}
			return fmt.Errorf("accept: %w", err)
		}
		go handleSocketConn(ctx, conn, s, engine, manager, auditor, approvalMgr)
	}
}

func handleSocketConn(
	ctx context.Context,
	conn net.Conn,
	s *sqlite.DB,
	engine *routing.Engine,
	manager *downstream.Manager,
	auditor *audit.Logger,
	approvalMgr *approval.Manager,
) {
	defer conn.Close()
	slog.Info("socket connection accepted", "remote", conn.RemoteAddr())

	gw := gateway.NewServer(s, engine, manager, auditor, gateway.TransportSocket,
		gateway.WithApprovals(approvalMgr))
	if err := gw.RunConn(ctx, conn, conn); err != nil {
		slog.Error("socket connection error", "err", err)
	}
	slog.Info("socket connection closed", "remote", conn.RemoteAddr())
}

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
	defer conn.Close()

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
	dst.Write(modified)  //nolint:errcheck
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

	// Don't overwrite existing roots
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
		// Suppress exit errors from signal-based shutdown
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("docker exec: %w", err)
	}
	return nil
}
