package store

import (
	"context"
	"encoding/json"
	"time"
)

// Store is the composite interface for all data access.
type Store interface {
	WorkspaceStore
	AuthScopeStore
	OAuthProviderStore
	DownstreamServerStore
	RouteRuleStore
	SessionStore
	AuditStore
	ToolApprovalStore
	Tx(ctx context.Context, fn func(Store) error) error
	Ping(ctx context.Context) error
	Close() error
}

// WorkspaceStore manages workspace records.
type WorkspaceStore interface {
	CreateWorkspace(ctx context.Context, w *Workspace) error
	GetWorkspace(ctx context.Context, id string) (*Workspace, error)
	GetWorkspaceByName(ctx context.Context, name string) (*Workspace, error)
	ListWorkspaces(ctx context.Context) ([]Workspace, error)
	UpdateWorkspace(ctx context.Context, w *Workspace) error
	DeleteWorkspace(ctx context.Context, id string) error
}

// AuthScopeStore manages auth scope records.
type AuthScopeStore interface {
	CreateAuthScope(ctx context.Context, a *AuthScope) error
	GetAuthScope(ctx context.Context, id string) (*AuthScope, error)
	GetAuthScopeByName(ctx context.Context, name string) (*AuthScope, error)
	ListAuthScopes(ctx context.Context) ([]AuthScope, error)
	UpdateAuthScope(ctx context.Context, a *AuthScope) error
	DeleteAuthScope(ctx context.Context, id string) error
	UpdateAuthScopeTokenData(ctx context.Context, id string, data []byte) error
}

// OAuthProviderStore manages OAuth provider records.
type OAuthProviderStore interface {
	CreateOAuthProvider(ctx context.Context, p *OAuthProvider) error
	GetOAuthProvider(ctx context.Context, id string) (*OAuthProvider, error)
	GetOAuthProviderByName(ctx context.Context, name string) (*OAuthProvider, error)
	ListOAuthProviders(ctx context.Context) ([]OAuthProvider, error)
	UpdateOAuthProvider(ctx context.Context, p *OAuthProvider) error
	DeleteOAuthProvider(ctx context.Context, id string) error
}

// DownstreamServerStore manages downstream server records.
type DownstreamServerStore interface {
	CreateDownstreamServer(ctx context.Context, d *DownstreamServer) error
	GetDownstreamServer(ctx context.Context, id string) (*DownstreamServer, error)
	GetDownstreamServerByName(ctx context.Context, name string) (*DownstreamServer, error)
	ListDownstreamServers(ctx context.Context) ([]DownstreamServer, error)
	UpdateDownstreamServer(ctx context.Context, d *DownstreamServer) error
	DeleteDownstreamServer(ctx context.Context, id string) error
	UpdateCapabilitiesCache(ctx context.Context, id string, cache json.RawMessage) error
}

// RouteRuleStore manages route rule records.
type RouteRuleStore interface {
	CreateRouteRule(ctx context.Context, r *RouteRule) error
	GetRouteRule(ctx context.Context, id string) (*RouteRule, error)
	ListRouteRules(ctx context.Context, workspaceID string) ([]RouteRule, error)
	UpdateRouteRule(ctx context.Context, r *RouteRule) error
	DeleteRouteRule(ctx context.Context, id string) error
}

// SessionStore manages session records.
type SessionStore interface {
	CreateSession(ctx context.Context, s *Session) error
	GetSession(ctx context.Context, id string) (*Session, error)
	DisconnectSession(ctx context.Context, id string) error
	ListActiveSessions(ctx context.Context) ([]Session, error)
	CleanupStaleSessions(ctx context.Context, before time.Time) (int, error)
}

// AuditStore manages audit log records.
type AuditStore interface {
	InsertAuditRecord(ctx context.Context, r *AuditRecord) error
	QueryAuditRecords(ctx context.Context, f AuditFilter) ([]AuditRecord, int, error)
	GetAuditStats(ctx context.Context, workspaceID string, after, before time.Time) (*AuditStats, error)
	GetDashboardTimeSeries(ctx context.Context, after, before time.Time) ([]TimeSeriesPoint, error)
	GetDashboardTimeSeriesBucketed(ctx context.Context, after, before time.Time, bucketSec int) ([]TimeSeriesPoint, error)
	GetToolLeaderboard(ctx context.Context, after, before time.Time, limit int) ([]ToolLeaderboardEntry, error)
	GetServerHealth(ctx context.Context, after, before time.Time) ([]ServerHealthEntry, error)
	GetErrorBreakdown(ctx context.Context, after, before time.Time, limit int) ([]ErrorBreakdownEntry, error)
	GetRouteHitMap(ctx context.Context, after, before time.Time) ([]RouteHitEntry, error)
	GetAuditCacheStats(ctx context.Context, after, before time.Time) (*AuditCacheStats, error)
}

// ToolApprovalStore manages tool call approval records.
type ToolApprovalStore interface {
	CreateToolApproval(ctx context.Context, a *ToolApproval) error
	GetToolApproval(ctx context.Context, id string) (*ToolApproval, error)
	ListPendingApprovals(ctx context.Context) ([]ToolApproval, error)
	ResolveToolApproval(ctx context.Context, id, status, approverSessionID, approverType, resolution string) error
	ExpirePendingApprovals(ctx context.Context, before time.Time) (int, error)
	GetApprovalMetrics(ctx context.Context, after, before time.Time) (*ApprovalMetrics, error)
}
