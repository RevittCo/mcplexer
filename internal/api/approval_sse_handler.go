package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/revittco/mcplexer/internal/approval"
)

type approvalSSEHandler struct {
	bus *approval.Bus
}

func (h *approvalSSEHandler) stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

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
		case evt, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(evt)
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
