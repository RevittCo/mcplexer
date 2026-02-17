package api

import (
	"errors"
	"net/http"

	"github.com/revittco/mcplexer/internal/approval"
	"github.com/revittco/mcplexer/internal/store"
)

type approvalHandler struct {
	manager *approval.Manager
	store   store.ToolApprovalStore
}

func (h *approvalHandler) list(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")

	if status == "pending" || status == "" {
		// Return in-memory pending approvals for realtime accuracy.
		pending := h.manager.ListPending("")
		if pending == nil {
			pending = []*store.ToolApproval{}
		}
		writeJSON(w, http.StatusOK, pending)
		return
	}

	// For resolved statuses, query the DB.
	approvals, err := h.store.ListPendingApprovals(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list approvals")
		return
	}
	if approvals == nil {
		approvals = []store.ToolApproval{}
	}
	writeJSON(w, http.StatusOK, approvals)
}

func (h *approvalHandler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a, err := h.store.GetToolApproval(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "approval not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get approval")
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (h *approvalHandler) resolve(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body struct {
		Approved bool   `json:"approved"`
		Reason   string `json:"reason"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := h.manager.Resolve(id, "", "dashboard", body.Reason, body.Approved)
	if err != nil {
		if errors.Is(err, approval.ErrAlreadyResolved) {
			writeError(w, http.StatusConflict, "approval already resolved")
			return
		}
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "approval not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	action := "denied"
	if body.Approved {
		action = "approved"
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": action})
}
