package api

import (
	"context"
	"net/http"

	"github.com/revittco/mcplexer/internal/downstream"
	"github.com/revittco/mcplexer/internal/store"
)

type discoverHandler struct {
	manager *downstream.Manager
	store   store.Store
}

func (h *discoverHandler) discover(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if h.manager == nil {
		writeError(w, http.StatusServiceUnavailable, "downstream manager not available")
		return
	}

	ctx := r.Context()

	// Find an auth scope linked to this downstream via route rules.
	authScopeID := h.findAuthScope(ctx, id)

	raw, err := h.manager.ListTools(ctx, id, authScopeID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to discover tools: "+err.Error())
		return
	}

	if err := h.store.UpdateCapabilitiesCache(ctx, id, raw); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update cache")
		return
	}

	srv, err := h.store.GetDownstreamServer(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read updated server")
		return
	}

	writeJSON(w, http.StatusOK, srv)
}

// findAuthScope looks up an auth scope linked to a downstream server via route rules.
func (h *discoverHandler) findAuthScope(ctx context.Context, serverID string) string {
	rules, err := h.store.ListRouteRules(ctx, "")
	if err != nil {
		return ""
	}
	for _, rule := range rules {
		if rule.DownstreamServerID == serverID && rule.AuthScopeID != "" {
			return rule.AuthScopeID
		}
	}
	return ""
}
