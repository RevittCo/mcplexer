package api

import (
	"context"
	"net/http"
	"slices"
	"time"

	"github.com/revittco/mcplexer/internal/cache"
	"github.com/revittco/mcplexer/internal/downstream"
	"github.com/revittco/mcplexer/internal/routing"
	"github.com/revittco/mcplexer/internal/store"
)

type dashboardHandler struct {
	sessionStore    store.SessionStore
	auditStore      store.AuditStore
	downstreamStore store.DownstreamServerStore
	approvalStore   store.ToolApprovalStore
	manager         *downstream.Manager // optional
	toolCache       *cache.ToolCache    // optional
	engine          *routing.Engine     // optional
}

type downstreamStatus struct {
	ServerID      string `json:"server_id"`
	ServerName    string `json:"server_name"`
	InstanceCount int    `json:"instance_count"`
	State         string `json:"state"`
	Disabled      bool   `json:"disabled"`
}

type dashboardResponse struct {
	ActiveSessions    int                          `json:"active_sessions"`
	ActiveSessionList []store.Session              `json:"active_session_list"`
	ActiveDownstreams []downstreamStatus           `json:"active_downstreams"`
	RecentErrors      []store.AuditRecord          `json:"recent_errors"`
	RecentCalls       []store.AuditRecord          `json:"recent_calls"`
	Stats             *store.AuditStats            `json:"stats,omitempty"`
	TimeSeries        []store.TimeSeriesPoint      `json:"timeseries"`
	ToolLeaderboard   []store.ToolLeaderboardEntry `json:"tool_leaderboard"`
	ServerHealth      []store.ServerHealthEntry    `json:"server_health"`
	ErrorBreakdown    []store.ErrorBreakdownEntry  `json:"error_breakdown"`
	RouteHitMap       []store.RouteHitEntry        `json:"route_hit_map"`
	ApprovalMetrics   *store.ApprovalMetrics       `json:"approval_metrics,omitempty"`
	CacheStats        *cacheStatsResponse          `json:"cache_stats,omitempty"`
}

// rangeConfig holds computed parameters for a dashboard time range.
type rangeConfig struct {
	statsWindow time.Duration
	bucketSec   int
	dataPoints  int
	callsLimit  int
	errorsLimit int
}

var rangeConfigs = map[string]rangeConfig{
	"1h":  {statsWindow: 1 * time.Hour, bucketSec: 60, dataPoints: 60, callsLimit: 20, errorsLimit: 10},
	"6h":  {statsWindow: 6 * time.Hour, bucketSec: 300, dataPoints: 72, callsLimit: 50, errorsLimit: 20},
	"24h": {statsWindow: 24 * time.Hour, bucketSec: 900, dataPoints: 96, callsLimit: 50, errorsLimit: 20},
	"7d":  {statsWindow: 7 * 24 * time.Hour, bucketSec: 3600, dataPoints: 168, callsLimit: 50, errorsLimit: 20},
}

func (h *dashboardHandler) get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rangeParam := r.URL.Query().Get("range")
	rc, ok := rangeConfigs[rangeParam]
	if !ok {
		rc = rangeConfigs["1h"]
	}

	now := time.Now().UTC()
	after := now.Add(-rc.statsWindow)

	// Clean up sessions that connected more than 1 hour ago and have no
	// recent audit activity — they are likely dead stdio clients.
	staleThreshold := now.Add(-1 * time.Hour)
	h.sessionStore.CleanupStaleSessions(ctx, staleThreshold) //nolint:errcheck

	sessions, err := h.sessionStore.ListActiveSessions(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list sessions")
		return
	}

	// Recent calls (all statuses)
	recentCalls, _, err := h.auditStore.QueryAuditRecords(ctx, store.AuditFilter{
		After: &after,
		Limit: rc.callsLimit,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query recent calls")
		return
	}

	// Recent errors + blocked
	errStatus := "error"
	errorRecords, _, err := h.auditStore.QueryAuditRecords(ctx, store.AuditFilter{
		Status: &errStatus,
		After:  &after,
		Limit:  rc.errorsLimit,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query recent errors")
		return
	}
	blockedStatus := "blocked"
	blockedRecords, _, err := h.auditStore.QueryAuditRecords(ctx, store.AuditFilter{
		Status: &blockedStatus,
		After:  &after,
		Limit:  rc.errorsLimit,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query recent blocked")
		return
	}
	errorRecords = append(errorRecords, blockedRecords...)
	slices.SortFunc(errorRecords, func(a, b store.AuditRecord) int {
		return b.Timestamp.Compare(a.Timestamp)
	})
	if len(errorRecords) > rc.errorsLimit {
		errorRecords = errorRecords[:rc.errorsLimit]
	}

	stats, err := h.auditStore.GetAuditStats(ctx, "", after, now)
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

	rawTS, err := h.auditStore.GetDashboardTimeSeriesBucketed(ctx, after, now, rc.bucketSec)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get time series")
		return
	}
	timeseries := fillTimeSeriesBucketed(rawTS, after, rc.bucketSec, rc.dataPoints)

	activeDownstreams := h.buildDownstreamStatus(ctx)

	toolLeaderboard, err := h.auditStore.GetToolLeaderboard(ctx, after, now, 10)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get tool leaderboard")
		return
	}
	if toolLeaderboard == nil {
		toolLeaderboard = []store.ToolLeaderboardEntry{}
	}

	serverHealth, err := h.auditStore.GetServerHealth(ctx, after, now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get server health")
		return
	}
	if serverHealth == nil {
		serverHealth = []store.ServerHealthEntry{}
	}

	errorBreakdown, err := h.auditStore.GetErrorBreakdown(ctx, after, now, 10)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get error breakdown")
		return
	}
	if errorBreakdown == nil {
		errorBreakdown = []store.ErrorBreakdownEntry{}
	}

	routeHitMap, err := h.auditStore.GetRouteHitMap(ctx, after, now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get route hit map")
		return
	}
	if routeHitMap == nil {
		routeHitMap = []store.RouteHitEntry{}
	}

	var approvalMetrics *store.ApprovalMetrics
	if h.approvalStore != nil {
		approvalMetrics, err = h.approvalStore.GetApprovalMetrics(ctx, after, now)
		if err != nil {
			approvalMetrics = nil // non-critical, degrade gracefully
		}
	}

	var cacheStats *cacheStatsResponse
	if h.engine != nil {
		cs := &cacheStatsResponse{
			RouteResolution: h.engine.RouteStats(),
		}
		// Compute tool call cache stats from audit records (DB-backed,
		// works across all stdio instances rather than just this process).
		auditCache, cacheErr := h.auditStore.GetAuditCacheStats(ctx, after, now)
		if cacheErr == nil && auditCache != nil {
			cs.ToolCall = cache.Stats{
				Hits:    int64(auditCache.Hits),
				Misses:  int64(auditCache.Misses),
				HitRate: auditCache.HitRate,
			}
		}
		cacheStats = cs
	}

	writeJSON(w, http.StatusOK, dashboardResponse{
		ActiveSessions:    len(sessions),
		ActiveSessionList: sessions,
		ActiveDownstreams: activeDownstreams,
		RecentErrors:      errorRecords,
		RecentCalls:       recentCalls,
		Stats:             stats,
		TimeSeries:        timeseries,
		ToolLeaderboard:   toolLeaderboard,
		ServerHealth:      serverHealth,
		ErrorBreakdown:    errorBreakdown,
		RouteHitMap:       routeHitMap,
		ApprovalMetrics:   approvalMetrics,
		CacheStats:        cacheStats,
	})
}

// fillTimeSeriesBucketed zero-fills missing buckets so the frontend always
// gets exactly `count` data points at the given bucket interval.
func fillTimeSeriesBucketed(raw []store.TimeSeriesPoint, start time.Time, bucketSec, count int) []store.TimeSeriesPoint {
	interval := time.Duration(bucketSec) * time.Second
	start = start.Truncate(interval)

	idx := make(map[int64]store.TimeSeriesPoint, len(raw))
	for _, p := range raw {
		idx[p.Bucket.Truncate(interval).Unix()] = p
	}

	out := make([]store.TimeSeriesPoint, count)
	for i := range count {
		bucket := start.Add(time.Duration(i) * interval)
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
			Disabled:   srv.Disabled,
		}
		if srv.Disabled {
			ds.State = "disabled"
		} else if agg, ok := running[srv.ID]; ok {
			ds.InstanceCount = agg.count
			ds.State = agg.state
		} else if srv.Transport == "http" {
			ds.State = "external"
		} else {
			ds.State = "stopped"
		}
		result = append(result, ds)
	}
	return result
}
