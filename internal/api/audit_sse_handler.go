package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/revitteth/mcplexer/internal/audit"
)

type auditSSEHandler struct {
	bus *audit.Bus
}

func (h *auditSSEHandler) stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Read optional filters from query params.
	qWorkspace := r.URL.Query().Get("workspace_id")
	qTool := r.URL.Query().Get("tool_name")
	qStatus := r.URL.Query().Get("status")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()

	ch := h.bus.Subscribe()
	defer h.bus.Unsubscribe(ch)

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case rec, ok := <-ch:
			if !ok {
				return
			}
			if !matchFilter(rec.WorkspaceID, qWorkspace) ||
				!matchFilter(rec.ToolName, qTool) ||
				!matchFilter(rec.Status, qStatus) {
				continue
			}
			data, err := json.Marshal(rec)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprint(w, ":\n\n")
			flusher.Flush()
		}
	}
}

// matchFilter returns true if the filter is empty or matches the value.
func matchFilter(value, filter string) bool {
	return filter == "" || value == filter
}
