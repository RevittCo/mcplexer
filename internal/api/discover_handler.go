package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/revittco/mcplexer/internal/addon"
	"github.com/revittco/mcplexer/internal/downstream"
	"github.com/revittco/mcplexer/internal/store"
)

type discoverHandler struct {
	manager  *downstream.Manager
	store    store.Store
	addonReg *addon.Registry // nil = no addons
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

	// Merge addon tools into the cache so they appear in the UI.
	raw = h.mergeAddonTools(ctx, id, raw)

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

// mergeAddonTools appends addon tools for this server's namespace into the
// capabilities cache so the UI can display them alongside native MCP tools.
func (h *discoverHandler) mergeAddonTools(
	ctx context.Context, serverID string, raw json.RawMessage,
) json.RawMessage {
	if h.addonReg == nil {
		return raw
	}

	srv, err := h.store.GetDownstreamServer(ctx, serverID)
	if err != nil {
		return raw
	}

	addonTools := h.addonReg.ToolsForNamespace(srv.ToolNamespace)
	if len(addonTools) == 0 {
		return raw
	}

	var cache struct {
		Tools []json.RawMessage `json:"tools"`
	}
	if err := json.Unmarshal(raw, &cache); err != nil {
		return raw
	}

	for _, at := range addonTools {
		entry := map[string]any{
			"name":        at.Name,
			"description": at.Description + " [addon]",
		}
		if at.InputSchema != nil {
			entry["inputSchema"] = at.InputSchema
		}
		data, err := json.Marshal(entry)
		if err != nil {
			continue
		}
		cache.Tools = append(cache.Tools, data)
	}

	merged, err := json.Marshal(cache)
	if err != nil {
		return raw
	}
	return merged
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
