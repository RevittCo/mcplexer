package api

import (
	"errors"
	"net/http"

	"github.com/revitteth/mcplexer/internal/config"
	"github.com/revitteth/mcplexer/internal/store"
)

type authHandler struct {
	svc   *config.Service
	store store.AuthScopeStore
}

// authScopeResponse masks EncryptedData from API responses.
type authScopeResponse struct {
	store.AuthScope
	HasSecrets bool `json:"has_secrets"`
}

func newAuthScopeResponse(a *store.AuthScope) authScopeResponse {
	return authScopeResponse{
		AuthScope:  *a,
		HasSecrets: len(a.EncryptedData) > 0,
	}
}

func (h *authHandler) list(w http.ResponseWriter, r *http.Request) {
	scopes, err := h.store.ListAuthScopes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list auth scopes")
		return
	}
	resp := make([]authScopeResponse, len(scopes))
	for i := range scopes {
		resp[i] = newAuthScopeResponse(&scopes[i])
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *authHandler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	scope, err := h.store.GetAuthScope(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "auth scope not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get auth scope")
		return
	}
	writeJSON(w, http.StatusOK, newAuthScopeResponse(scope))
}

func (h *authHandler) create(w http.ResponseWriter, r *http.Request) {
	var a store.AuthScope
	if err := decodeJSON(r, &a); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.CreateAuthScope(r.Context(), &a); err != nil {
		if errors.Is(err, store.ErrAlreadyExists) {
			writeError(w, http.StatusConflict, "auth scope already exists")
			return
		}
		writeErrorDetail(w, http.StatusBadRequest, "failed to create auth scope", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, newAuthScopeResponse(&a))
}

func (h *authHandler) update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var a store.AuthScope
	if err := decodeJSON(r, &a); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	a.ID = id
	if err := h.svc.UpdateAuthScope(r.Context(), &a); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "auth scope not found")
			return
		}
		writeErrorDetail(w, http.StatusBadRequest, "failed to update auth scope", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, newAuthScopeResponse(&a))
}

func (h *authHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteAuthScope(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "auth scope not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete auth scope")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
