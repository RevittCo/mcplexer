package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/revittco/mcplexer/internal/addon"
	"github.com/revittco/mcplexer/internal/api"
	"github.com/revittco/mcplexer/internal/approval"
	"github.com/revittco/mcplexer/internal/audit"
	"github.com/revittco/mcplexer/internal/auth"
	"github.com/revittco/mcplexer/internal/cache"
	"github.com/revittco/mcplexer/internal/config"
	"github.com/revittco/mcplexer/internal/downstream"
	"github.com/revittco/mcplexer/internal/gateway"
	"github.com/revittco/mcplexer/internal/mcpinstall"
	"github.com/revittco/mcplexer/internal/oauth"
	"github.com/revittco/mcplexer/internal/routing"
	"github.com/revittco/mcplexer/internal/secrets"
	"github.com/revittco/mcplexer/internal/store/sqlite"
	"golang.org/x/sync/errgroup"
)

func cmdServe(args []string) error {
	ctx, cancel := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM,
	)
	defer cancel()

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	applyFlags(cfg, args)

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	slog.SetDefault(logger)

	db, err := sqlite.New(ctx, cfg.DBDSN)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	if err := config.SeedDefaultWorkspaces(ctx, db); err != nil {
		return err
	}
	if err := config.SeedDefaultOAuthProviders(ctx, db); err != nil {
		return err
	}
	if err := config.SeedDefaultDownstreamServers(ctx, db); err != nil {
		return err
	}
	if err := config.SeedDefaultAuthScopes(ctx, db); err != nil {
		return err
	}
	if err := config.SeedDefaultRouteRules(ctx, db); err != nil {
		return err
	}

	// Load YAML config into store if file exists.
	if cfg.ConfigFile != "" {
		if _, err := os.Stat(cfg.ConfigFile); err == nil {
			fileCfg, err := config.LoadFile(cfg.ConfigFile)
			if err != nil {
				return err
			}
			if err := config.Apply(ctx, db, fileCfg); err != nil {
				return err
			}
			logger.Info("loaded config", "file", cfg.ConfigFile)
		}
	}

	cfgSvc := config.NewService(db)
	settingsSvc := config.NewSettingsService(db)

	switch cfg.Mode {
	case "stdio":
		logger.Info("starting in stdio mode")
		return runStdio(ctx, cfg, db, settingsSvc)
	case "http":
		if cfg.SocketPath != "" {
			return runHTTPAndSocket(ctx, cfg, db, cfgSvc, settingsSvc)
		}
		return runHTTP(ctx, cfg, db, cfgSvc, settingsSvc)
	default:
		return err
	}
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

func runHTTP(ctx context.Context, cfg *Config, db *sqlite.DB, cfgSvc *config.Service, settingsSvc *config.SettingsService) error {
	authInj, fm, enc, err := buildAuthInjector(cfg, db)
	if err != nil {
		return err
	}

	engine := routing.NewEngine(db)
	manager := downstream.NewManager(db, authInj)
	defer manager.Shutdown(ctx) //nolint:errcheck

	tc := buildToolCache(ctx, db)

	approvalBus := approval.NewBus()
	approvalMgr := approval.NewManager(db, approvalBus)
	approvalMgr.ExpireStale(ctx)
	defer approvalMgr.Shutdown()

	installMgr, err := mcpinstall.New()
	if err != nil {
		slog.Warn("mcp install manager unavailable", "error", err)
	}

	addonReg, _ := loadAddons(ctx, cfg, db, authInj)

	auditBus := audit.NewBus()
	router := api.NewRouter(api.RouterDeps{
		Store:           db,
		ConfigSvc:       cfgSvc,
		SettingsSvc:     settingsSvc,
		Engine:          engine,
		Manager:         manager,
		FlowManager:     fm,
		Encryptor:       enc,
		AuditBus:        auditBus,
		ApprovalManager: approvalMgr,
		ApprovalBus:     approvalBus,
		ToolCache:       tc,
		InstallManager:  installMgr,
		AddonRegistry:   addonReg,
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MiB
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

func runStdio(ctx context.Context, cfg *Config, db *sqlite.DB, settingsSvc *config.SettingsService) error {
	authInj, _, _, err := buildAuthInjector(cfg, db)
	if err != nil {
		return err
	}

	engine := routing.NewEngine(db)
	manager := downstream.NewManager(db, authInj)
	defer manager.Shutdown(ctx) //nolint:errcheck

	tc := buildToolCache(ctx, db)
	lister := cache.NewCachingToolLister(manager, tc)

	approvalBus := approval.NewBus()
	approvalMgr := approval.NewManager(db, approvalBus)
	approvalMgr.ExpireStale(ctx)
	defer approvalMgr.Shutdown()

	addonReg, addonExec := loadAddons(ctx, cfg, db, authInj)

	auditor := audit.NewLogger(db, db, nil)
	gwOpts := []gateway.ServerOption{
		gateway.WithApprovals(approvalMgr),
		gateway.WithSettings(settingsSvc),
	}
	if addonReg != nil {
		gwOpts = append(gwOpts, gateway.WithAddons(addonReg, addonExec))
	}
	gw := gateway.NewServer(db, engine, lister, auditor, gateway.TransportStdio, gwOpts...)

	manager.OnToolsChanged = gw.InvalidateAndNotifyToolsChanged

	return gw.RunStdio(ctx)
}

// buildToolCache loads per-server cache configs from the DB and creates a ToolCache.
func buildToolCache(ctx context.Context, db *sqlite.DB) *cache.ToolCache {
	servers, err := db.ListDownstreamServers(ctx)
	if err != nil {
		slog.Warn("failed to load servers for cache config, using defaults", "error", err)
		return cache.NewToolCache(nil)
	}

	configs := make(map[string]cache.ServerCacheConfig, len(servers))
	for _, srv := range servers {
		if len(srv.CacheConfig) == 0 || string(srv.CacheConfig) == "{}" {
			continue
		}
		var cfg cache.ServerCacheConfig
		if err := json.Unmarshal(srv.CacheConfig, &cfg); err != nil {
			slog.Warn("invalid cache config for server, using defaults",
				"server", srv.ID, "error", err)
			continue
		}
		configs[srv.ID] = cfg
	}

	return cache.NewToolCache(configs)
}

// loadAddons loads addon YAML files from the addons/ directory next to the DB.
// Returns nil, nil if no addons directory exists or loading fails (non-fatal).
func loadAddons(ctx context.Context, cfg *Config, db *sqlite.DB, authInj *auth.Injector) (*addon.Registry, *addon.Executor) {
	addonDir := filepath.Join(filepath.Dir(cfg.DBDSN), "addons")
	if _, err := os.Stat(addonDir); err != nil {
		return nil, nil
	}

	resolver := func(serverID string) (string, error) {
		srv, err := db.GetDownstreamServer(ctx, serverID)
		if err != nil {
			return "", err
		}
		return srv.ToolNamespace, nil
	}

	reg, err := addon.LoadDir(addonDir, resolver)
	if err != nil {
		slog.Warn("failed to load addons", "dir", addonDir, "error", err)
		return nil, nil
	}

	if len(reg.AllTools()) == 0 {
		return nil, nil
	}

	exec := addon.NewExecutor(authInj.HeadersForDownstream)
	return reg, exec
}

// buildAuthInjector creates an auth.Injector and optionally an oauth.FlowManager.
func buildAuthInjector(cfg *Config, db *sqlite.DB) (*auth.Injector, *oauth.FlowManager, *secrets.AgeEncryptor, error) {
	var enc *secrets.AgeEncryptor
	var sm *secrets.Manager
	var fm *oauth.FlowManager

	if cfg.AgeKeyPath != "" {
		var err error
		enc, err = secrets.NewAgeEncryptor(cfg.AgeKeyPath)
		if err != nil {
			return nil, nil, nil, err
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
		externalURL = httpURLFromAddr(cfg.HTTPAddr)
	}

	if enc != nil {
		fm = oauth.NewFlowManager(db, enc, externalURL)
	}

	return auth.NewInjector(sm, fm, db), fm, enc, nil
}

// runHTTPAndSocket runs both the HTTP server and Unix socket listener
// concurrently using an errgroup.
func runHTTPAndSocket(ctx context.Context, cfg *Config, db *sqlite.DB, cfgSvc *config.Service, settingsSvc *config.SettingsService) error {
	authInj, fm, enc, err := buildAuthInjector(cfg, db)
	if err != nil {
		return err
	}

	engine := routing.NewEngine(db)
	manager := downstream.NewManager(db, authInj)
	defer manager.Shutdown(ctx) //nolint:errcheck

	tc := buildToolCache(ctx, db)
	lister := cache.NewCachingToolLister(manager, tc)

	approvalBus := approval.NewBus()
	approvalMgr := approval.NewManager(db, approvalBus)
	approvalMgr.ExpireStale(ctx)
	defer approvalMgr.Shutdown()

	addonReg, addonExec := loadAddons(ctx, cfg, db, authInj)

	installMgr2, err := mcpinstall.New()
	if err != nil {
		slog.Warn("mcp install manager unavailable", "error", err)
	}

	auditBus := audit.NewBus()
	auditor := audit.NewLogger(db, db, auditBus)
	g, ctx := errgroup.WithContext(ctx)

	// HTTP server
	g.Go(func() error {
		router := api.NewRouter(api.RouterDeps{
			Store:           db,
			ConfigSvc:       cfgSvc,
			SettingsSvc:     settingsSvc,
			Engine:          engine,
			Manager:         manager,
			FlowManager:     fm,
			Encryptor:       enc,
			AuditBus:        auditBus,
			ApprovalManager: approvalMgr,
			ApprovalBus:     approvalBus,
			ToolCache:       tc,
			InstallManager:  installMgr2,
			AddonRegistry:   addonReg,
		})
		srv := &http.Server{Addr: cfg.HTTPAddr, Handler: router}
		srv.ReadHeaderTimeout = 10 * time.Second
		srv.ReadTimeout = 15 * time.Second
		srv.WriteTimeout = 30 * time.Second
		srv.IdleTimeout = 60 * time.Second
		srv.MaxHeaderBytes = 1 << 20 // 1 MiB
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
		return runSocket(ctx, cfg.SocketPath, db, engine, lister, auditor, approvalMgr, settingsSvc, addonReg, addonExec)
	})

	return g.Wait()
}
