package api

import (
	"errors"
	"net/http"

	"github.com/revittco/mcplexer/internal/config"
	"github.com/revittco/mcplexer/internal/routing"
	"github.com/revittco/mcplexer/internal/store"
)

type downstreamHandler struct {
	svc    *config.Service
	store  store.DownstreamServerStore
	engine *routing.Engine // optional; invalidates route cache on mutations
}

func (h *downstreamHandler) list(w http.ResponseWriter, r *http.Request) {
	servers, err := h.store.ListDownstreamServers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list downstream servers")
		return
	}
	if servers == nil {
		servers = []store.DownstreamServer{}
	}
	writeJSON(w, http.StatusOK, servers)
}

func (h *downstreamHandler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	srv, err := h.store.GetDownstreamServer(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "downstream server not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get downstream server")
		return
	}
	writeJSON(w, http.StatusOK, srv)
}

func (h *downstreamHandler) create(w http.ResponseWriter, r *http.Request) {
	var ds store.DownstreamServer
	if err := decodeJSON(r, &ds); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.CreateDownstreamServer(r.Context(), &ds); err != nil {
		if errors.Is(err, store.ErrAlreadyExists) {
			writeError(w, http.StatusConflict, "downstream server already exists")
			return
		}
		writeErrorDetail(w, http.StatusBadRequest, "failed to create downstream server", err.Error())
		return
	}
	if h.engine != nil {
		h.engine.InvalidateAllRoutes()
	}
	writeJSON(w, http.StatusCreated, ds)
}

func (h *downstreamHandler) update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx := r.Context()

	// Load existing record so partial updates work.
	existing, err := h.store.GetDownstreamServer(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "downstream server not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get downstream server")
		return
	}

	// Decode body on top of existing values.
	ds := *existing
	if err := decodeJSON(r, &ds); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ds.ID = id

	if err := h.svc.UpdateDownstreamServer(ctx, &ds); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "downstream server not found")
			return
		}
		writeErrorDetail(w, http.StatusBadRequest, "failed to update downstream server", err.Error())
		return
	}
	if h.engine != nil {
		h.engine.InvalidateAllRoutes()
	}
	writeJSON(w, http.StatusOK, ds)
}

func (h *downstreamHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteDownstreamServer(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "downstream server not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete downstream server")
		return
	}
	if h.engine != nil {
		h.engine.InvalidateAllRoutes()
	}
	w.WriteHeader(http.StatusNoContent)
}
