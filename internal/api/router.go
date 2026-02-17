package api

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/revittco/mcplexer/internal/approval"
	"github.com/revittco/mcplexer/internal/audit"
	"github.com/revittco/mcplexer/internal/config"
	"github.com/revittco/mcplexer/internal/downstream"
	"github.com/revittco/mcplexer/internal/oauth"
	"github.com/revittco/mcplexer/internal/routing"
	"github.com/revittco/mcplexer/internal/secrets"
	"github.com/revittco/mcplexer/internal/store"
	"github.com/revittco/mcplexer/internal/web"
)

// RouterDeps holds the dependencies needed by the HTTP API router.
type RouterDeps struct {
	Store           store.Store
	ConfigSvc       *config.Service
	Engine          *routing.Engine
	Manager         *downstream.Manager    // optional; enables tool discovery
	FlowManager     *oauth.FlowManager     // optional; enables OAuth flows
	Encryptor       *secrets.AgeEncryptor  // optional; enables secret encryption
	AuditBus        *audit.Bus             // optional; enables SSE audit stream
	ApprovalManager *approval.Manager      // optional; enables approval system
	ApprovalBus     *approval.Bus          // optional; enables approval SSE stream
}

// NewRouter creates an http.Handler with all API routes and SPA fallback.
func NewRouter(deps RouterDeps) http.Handler {
	mux := http.NewServeMux()

	ws := &workspaceHandler{svc: deps.ConfigSvc, store: deps.Store}
	mux.HandleFunc("GET /api/v1/workspaces", ws.list)
	mux.HandleFunc("POST /api/v1/workspaces", ws.create)
	mux.HandleFunc("GET /api/v1/workspaces/{id}", ws.get)
	mux.HandleFunc("PUT /api/v1/workspaces/{id}", ws.update)
	mux.HandleFunc("DELETE /api/v1/workspaces/{id}", ws.delete)

	ds := &downstreamHandler{svc: deps.ConfigSvc, store: deps.Store}
	mux.HandleFunc("GET /api/v1/downstreams", ds.list)
	mux.HandleFunc("POST /api/v1/downstreams", ds.create)
	mux.HandleFunc("GET /api/v1/downstreams/{id}", ds.get)
	mux.HandleFunc("PUT /api/v1/downstreams/{id}", ds.update)
	mux.HandleFunc("DELETE /api/v1/downstreams/{id}", ds.delete)

	rt := &routeHandler{svc: deps.ConfigSvc, store: deps.Store}
	mux.HandleFunc("GET /api/v1/routes", rt.list)
	mux.HandleFunc("POST /api/v1/routes", rt.create)
	mux.HandleFunc("GET /api/v1/routes/{id}", rt.get)
	mux.HandleFunc("PUT /api/v1/routes/{id}", rt.update)
	mux.HandleFunc("DELETE /api/v1/routes/{id}", rt.delete)

	auth := &authHandler{svc: deps.ConfigSvc, store: deps.Store}
	mux.HandleFunc("GET /api/v1/auth-scopes", auth.list)
	mux.HandleFunc("POST /api/v1/auth-scopes", auth.create)
	mux.HandleFunc("GET /api/v1/auth-scopes/{id}", auth.get)
	mux.HandleFunc("PUT /api/v1/auth-scopes/{id}", auth.update)
	mux.HandleFunc("DELETE /api/v1/auth-scopes/{id}", auth.delete)

	auditH := &auditHandler{store: deps.Store}
	mux.HandleFunc("GET /api/v1/audit", auditH.query)

	if deps.AuditBus != nil {
		sse := &auditSSEHandler{bus: deps.AuditBus}
		mux.HandleFunc("GET /api/v1/audit/stream", sse.stream)
	}

	if deps.ApprovalManager != nil {
		ah := &approvalHandler{manager: deps.ApprovalManager, store: deps.Store}
		mux.HandleFunc("GET /api/v1/approvals", ah.list)
		mux.HandleFunc("GET /api/v1/approvals/{id}", ah.get)
		mux.HandleFunc("POST /api/v1/approvals/{id}/resolve", ah.resolve)
	}

	if deps.ApprovalBus != nil {
		asse := &approvalSSEHandler{bus: deps.ApprovalBus}
		mux.HandleFunc("GET /api/v1/approvals/stream", asse.stream)
	}

	mux.HandleFunc("GET /api/v1/health", healthCheck)

	dash := &dashboardHandler{
		sessionStore:    deps.Store,
		auditStore:      deps.Store,
		downstreamStore: deps.Store,
		manager:         deps.Manager,
	}
	mux.HandleFunc("GET /api/v1/dashboard", dash.get)

	dr := &dryRunHandler{
		engine:          deps.Engine,
		routeStore:      deps.Store,
		workspaceStore:  deps.Store,
		downstreamStore: deps.Store,
		authScopeStore:  deps.Store,
		flowManager:     deps.FlowManager,
	}
	mux.HandleFunc("POST /api/v1/dry-run", dr.run)

	disc := &discoverHandler{manager: deps.Manager, store: deps.Store}
	mux.HandleFunc("POST /api/v1/downstreams/{id}/discover", disc.discover)

	if deps.FlowManager != nil {
		dOAuth := &downstreamOAuthHandler{
			store:       deps.Store,
			flowManager: deps.FlowManager,
			callbackURL: deps.FlowManager.CallbackURL(),
		}
		mux.HandleFunc("POST /api/v1/downstreams/{id}/oauth-setup", dOAuth.setup)
		mux.HandleFunc("GET /api/v1/downstreams/{id}/oauth-status", dOAuth.status)

		dc := &downstreamConnectHandler{
			store:       deps.Store,
			flowManager: deps.FlowManager,
			encryptor:   deps.Encryptor,
		}
		mux.HandleFunc("POST /api/v1/downstreams/{id}/connect", dc.connect)
		mux.HandleFunc("GET /api/v1/downstreams/{id}/oauth-capabilities", dc.capabilities)
	}

	op := &oauthProviderHandler{svc: deps.ConfigSvc, store: deps.Store, encryptor: deps.Encryptor}
	mux.HandleFunc("GET /api/v1/oauth-providers", op.list)
	mux.HandleFunc("POST /api/v1/oauth-providers", op.create)
	mux.HandleFunc("GET /api/v1/oauth-providers/{id}", op.get)
	mux.HandleFunc("PUT /api/v1/oauth-providers/{id}", op.update)
	mux.HandleFunc("DELETE /api/v1/oauth-providers/{id}", op.delete)
	mux.HandleFunc("GET /api/v1/oauth-templates", op.listTemplates)

	oidc := &oidcDiscoverHandler{}
	mux.HandleFunc("POST /api/v1/oauth-providers/discover", oidc.discover)

	if deps.FlowManager != nil {
		of := &oauthFlowHandler{
			flow:      deps.FlowManager,
			store:     deps.Store,
			opStore:   deps.Store,
			encryptor: deps.Encryptor,
		}
		mux.HandleFunc("GET /api/v1/auth-scopes/{id}/oauth/authorize", of.authorize)
		mux.HandleFunc("GET /api/v1/oauth/callback", of.callback)
		mux.HandleFunc("GET /api/v1/auth-scopes/{id}/oauth/status", of.status)
		mux.HandleFunc("POST /api/v1/auth-scopes/{id}/oauth/revoke", of.revoke)
		mux.HandleFunc("POST /api/v1/auth-scopes/oauth-quick-setup", of.quickSetup)
	}

	// SPA fallback: serve embedded static files
	distFS, err := fs.Sub(web.StaticFiles, "dist")
	if err == nil {
		spaHandler := spaFallback(distFS, http.FileServerFS(distFS))
		mux.Handle("/", spaHandler)
	}

	// Apply middleware chain: CORS -> RequestID -> Logging -> mux
	var handler http.Handler = mux
	handler = loggingMiddleware(handler)
	handler = requestIDMiddleware(handler)
	handler = corsMiddleware(handler)

	return handler
}

// spaFallback serves static files from the embedded FS, falling back to
// index.html for any path that doesn't match a real file (SPA client-side routing).
func spaFallback(staticFS fs.FS, fileServer http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clean the path and check if the file exists in the embedded FS.
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(staticFS, p); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		// File doesn't exist — serve index.html for SPA routing.
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
