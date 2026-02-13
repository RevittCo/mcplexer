package api

import (
	"errors"
	"net/http"

	"github.com/revitteth/mcplexer/internal/config"
	"github.com/revitteth/mcplexer/internal/store"
)

type routeHandler struct {
	svc   *config.Service
	store store.RouteRuleStore
}

func (h *routeHandler) list(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.URL.Query().Get("workspace_id")
	rules, err := h.store.ListRouteRules(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list route rules")
		return
	}
	if rules == nil {
		rules = []store.RouteRule{}
	}
	writeJSON(w, http.StatusOK, rules)
}

func (h *routeHandler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rule, err := h.store.GetRouteRule(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "route rule not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get route rule")
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (h *routeHandler) create(w http.ResponseWriter, r *http.Request) {
	var rr store.RouteRule
	if err := decodeJSON(r, &rr); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.CreateRouteRule(r.Context(), &rr); err != nil {
		if errors.Is(err, store.ErrAlreadyExists) {
			writeError(w, http.StatusConflict, "route rule already exists")
			return
		}
		writeErrorDetail(w, http.StatusBadRequest, "failed to create route rule", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, rr)
}

func (h *routeHandler) update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx := r.Context()

	// Load existing record so partial updates work.
	existing, err := h.store.GetRouteRule(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "route rule not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get route rule")
		return
	}

	// Decode body on top of existing values.
	rr := *existing
	if err := decodeJSON(r, &rr); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	rr.ID = id

	if err := h.svc.UpdateRouteRule(ctx, &rr); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "route rule not found")
			return
		}
		writeErrorDetail(w, http.StatusBadRequest, "failed to update route rule", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rr)
}

func (h *routeHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteRouteRule(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "route rule not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete route rule")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
