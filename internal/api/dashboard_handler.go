package api

import (
	"context"
	"net/http"
	"time"

	"github.com/revittco/mcplexer/internal/downstream"
	"github.com/revittco/mcplexer/internal/store"
)

type dashboardHandler struct {
	sessionStore    store.SessionStore
	auditStore      store.AuditStore
	downstreamStore store.DownstreamServerStore
	manager         *downstream.Manager // optional
}

type downstreamStatus struct {
	ServerID      string `json:"server_id"`
	ServerName    string `json:"server_name"`
	InstanceCount int    `json:"instance_count"`
	State         string `json:"state"`
}

type dashboardResponse struct {
	ActiveSessions    int                    `json:"active_sessions"`
	ActiveDownstreams []downstreamStatus     `json:"active_downstreams"`
	RecentErrors      []store.AuditRecord    `json:"recent_errors"`
	RecentCalls       []store.AuditRecord    `json:"recent_calls"`
	Stats             *store.AuditStats      `json:"stats,omitempty"`
	TimeSeries        []store.TimeSeriesPoint `json:"timeseries"`
}

func (h *dashboardHandler) get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	sessions, err := h.sessionStore.ListActiveSessions(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list sessions")
		return
	}

	now := time.Now().UTC()
	thirtyMinAgo := now.Add(-30 * time.Minute)
	oneHourAgo := now.Add(-1 * time.Hour)

	// Recent calls (all statuses)
	recentCalls, _, err := h.auditStore.QueryAuditRecords(ctx, store.AuditFilter{
		After: &oneHourAgo,
		Limit: 20,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query recent calls")
		return
	}

	// Recent errors only
	errStatus := "error"
	errorRecords, _, err := h.auditStore.QueryAuditRecords(ctx, store.AuditFilter{
		Status: &errStatus,
		After:  &oneHourAgo,
		Limit:  10,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query recent errors")
		return
	}

	stats, err := h.auditStore.GetAuditStats(ctx, "", oneHourAgo, now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get audit stats")
		return
	}

	// Ensure non-nil slices so JSON encodes as [] not null
	if recentCalls == nil {
		recentCalls = []store.AuditRecord{}
	}
	if errorRecords == nil {
		errorRecords = []store.AuditRecord{}
	}

	rawTS, err := h.auditStore.GetDashboardTimeSeries(ctx, thirtyMinAgo, now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get time series")
		return
	}
	timeseries := fillTimeSeries(rawTS, thirtyMinAgo, 30)

	activeDownstreams := h.buildDownstreamStatus(ctx)

	writeJSON(w, http.StatusOK, dashboardResponse{
		ActiveSessions:    len(sessions),
		ActiveDownstreams: activeDownstreams,
		RecentErrors:      errorRecords,
		RecentCalls:       recentCalls,
		Stats:             stats,
		TimeSeries:        timeseries,
	})
}

// fillTimeSeries zero-fills missing minute buckets so the frontend always
// gets exactly `minutes` data points.
func fillTimeSeries(raw []store.TimeSeriesPoint, start time.Time, minutes int) []store.TimeSeriesPoint {
	// Truncate start to the minute boundary.
	start = start.Truncate(time.Minute)

	// Index raw points by their minute bucket.
	idx := make(map[int64]store.TimeSeriesPoint, len(raw))
	for _, p := range raw {
		idx[p.Bucket.Truncate(time.Minute).Unix()] = p
	}

	out := make([]store.TimeSeriesPoint, minutes)
	for i := range minutes {
		bucket := start.Add(time.Duration(i) * time.Minute)
		if p, ok := idx[bucket.Unix()]; ok {
			out[i] = p
		} else {
			out[i] = store.TimeSeriesPoint{Bucket: bucket}
		}
	}
	return out
}

// buildDownstreamStatus lists all configured downstream servers and overlays
// live instance state from the process manager when available.
func (h *dashboardHandler) buildDownstreamStatus(ctx context.Context) []downstreamStatus {
	servers, err := h.downstreamStore.ListDownstreamServers(ctx)
	if err != nil {
		return []downstreamStatus{}
	}

	// Build a map of serverID -> aggregated running instances from the manager.
	type instanceAgg struct {
		count int
		state string // "best" state across instances
	}
	running := make(map[string]instanceAgg)
	if h.manager != nil {
		for _, info := range h.manager.ListInstances() {
			agg := running[info.Key.ServerID]
			agg.count++
			agg.state = info.State.String()
			running[info.Key.ServerID] = agg
		}
	}

	result := make([]downstreamStatus, 0, len(servers))
	for _, srv := range servers {
		ds := downstreamStatus{
			ServerID:   srv.ID,
			ServerName: srv.Name,
		}
		if agg, ok := running[srv.ID]; ok {
			ds.InstanceCount = agg.count
			ds.State = agg.state
		} else {
			ds.State = "stopped"
		}
		result = append(result, ds)
	}
	return result
}
