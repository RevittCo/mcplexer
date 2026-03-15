package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/revittco/mcplexer/internal/addon"
	"github.com/revittco/mcplexer/internal/approval"
	"github.com/revittco/mcplexer/internal/audit"
	"github.com/revittco/mcplexer/internal/cache"
	"github.com/revittco/mcplexer/internal/config"
	"github.com/revittco/mcplexer/internal/routing"
	"github.com/revittco/mcplexer/internal/store"
)

// ToolLister abstracts downstream tool discovery and invocation.
type ToolLister interface {
	ListAllTools(ctx context.Context) (map[string]json.RawMessage, error)
	ListToolsForServers(ctx context.Context, serverIDs []string) (map[string]json.RawMessage, error)
	Call(ctx context.Context, serverID, authScopeID, toolName string, args json.RawMessage) (json.RawMessage, error)
}

// CachingCaller extends ToolLister with cache-aware calling.
type CachingCaller interface {
	ToolLister
	CallWithMeta(ctx context.Context, serverID, authScopeID, toolName string, args json.RawMessage, cacheBust bool) (cache.CallResult, error)
	ToolCache() *cache.ToolCache
}

// handler contains the logic for each MCP method.
type handler struct {
	store          store.Store
	engine         *routing.Engine
	manager        ToolLister
	sessions       *sessionManager
	auditor        *audit.Logger
	approvals      *approval.Manager // nil = approval system disabled
	settingsSvc    *config.SettingsService
	toolsListCache *cache.Cache[string, json.RawMessage]
	notifier       Notifier // set at runtime for sending notifications
	addonRegistry  *addon.Registry // nil = no addons loaded
	addonExecutor  *addon.Executor // nil = no addons loaded

	// bgCtx is a long-lived context for background goroutines (set from run()).
	bgCtx context.Context
	// backgroundRefreshOnce ensures we only trigger one background refresh
	// after returning cached capabilities on the first tools/list call.
	backgroundRefreshOnce sync.Once
}

// setNotifier sets the notifier for sending client notifications.
func (h *handler) setNotifier(n Notifier) {
	h.notifier = n
}

func newHandler(
	s store.Store,
	e *routing.Engine,
	m ToolLister,
	a *audit.Logger,
	t TransportMode,
	approvals *approval.Manager,
	settingsSvc *config.SettingsService,
	addonReg *addon.Registry,
	addonExec *addon.Executor,
) *handler {
	ttl := 15 * time.Second
	if settingsSvc != nil {
		settings := settingsSvc.Load(context.Background())
		if settings.ToolsCacheTTLSec > 0 {
			ttl = time.Duration(settings.ToolsCacheTTLSec) * time.Second
		}
	}
	return &handler{
		store:          s,
		engine:         e,
		manager:        m,
		sessions:       newSessionManager(s, e, t),
		auditor:        a,
		approvals:      approvals,
		settingsSvc:    settingsSvc,
		toolsListCache: cache.New[string, json.RawMessage](10, ttl),
		addonRegistry:  addonReg,
		addonExecutor:  addonExec,
	}
}

func (h *handler) handleInitialize(
	ctx context.Context, params json.RawMessage,
) (json.RawMessage, *RPCError) {
	var p InitializeParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &RPCError{Code: CodeInvalidParams, Message: err.Error()}
	}

	if err := h.sessions.create(ctx, p.ClientInfo, p.Roots); err != nil {
		slog.Error("create session", "error", err)
	}

	result := InitializeResult{
		ProtocolVersion: "2025-03-26",
		Capabilities: ServerCapability{
			Tools: &ToolCapability{ListChanged: true},
		},
		ServerInfo: ServerInfo{Name: "mcplexer", Version: "0.1.6"},
	}

	data, err := json.Marshal(result)
	if err != nil {
		return nil, &RPCError{Code: CodeInternalError, Message: err.Error()}
	}
	return data, nil
}

// InvalidateAndNotifyToolsChanged clears the tools/list cache and sends
// a tools/list_changed notification to the connected client. Called by
// the downstream manager when a server emits notifications/tools/list_changed.
func (h *handler) InvalidateAndNotifyToolsChanged() {
	h.toolsListCache.Flush()
	h.sendToolsListChanged()
}

func mapRouteError(err error) *RPCError {
	switch {
	case errors.Is(err, routing.ErrNoRoute):
		return &RPCError{Code: CodeRouteNotFound, Message: "no matching route"}
	case errors.Is(err, routing.ErrDenied):
		return &RPCError{Code: CodeRouteNotFound, Message: "route denied by policy"}
	default:
		return &RPCError{Code: CodeInternalError, Message: err.Error()}
	}
}
