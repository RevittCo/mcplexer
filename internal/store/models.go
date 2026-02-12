package store

import (
	"encoding/json"
	"time"
)

// Workspace represents a workspace context for routing.
type Workspace struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	RootPath      string          `json:"root_path"`
	Tags          json.RawMessage `json:"tags,omitempty"`
	DefaultPolicy string          `json:"default_policy"`
	Source        string          `json:"source"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// AuthScope represents a credential scope for downstream server authentication.
type AuthScope struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Type            string          `json:"type"`
	EncryptedData   []byte          `json:"-"`
	RedactionHints  json.RawMessage `json:"redaction_hints,omitempty"`
	OAuthProviderID string          `json:"oauth_provider_id"`
	OAuthTokenData  []byte          `json:"-"`
	Source          string          `json:"source"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// OAuthProvider stores OAuth2 app configuration.
type OAuthProvider struct {
	ID                    string          `json:"id"`
	Name                  string          `json:"name"`
	TemplateID            string          `json:"template_id"`
	AuthorizeURL          string          `json:"authorize_url"`
	TokenURL              string          `json:"token_url"`
	ClientID              string          `json:"client_id"`
	EncryptedClientSecret []byte          `json:"-"`
	Scopes                json.RawMessage `json:"scopes,omitempty"`
	UsePKCE               bool            `json:"use_pkce"`
	Source                string          `json:"source"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
}

// OAuthTokenData holds decrypted OAuth2 token information.
type OAuthTokenData struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	TokenType    string   `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
	Scopes       []string `json:"scopes,omitempty"`
}

// DownstreamServer represents a downstream MCP server configuration.
type DownstreamServer struct {
	ID                string          `json:"id"`
	Name              string          `json:"name"`
	Transport         string          `json:"transport"`
	Command           string          `json:"command"`
	Args              json.RawMessage `json:"args,omitempty"`
	URL               *string         `json:"url,omitempty"`
	ToolNamespace     string          `json:"tool_namespace"`
	Discovery         string          `json:"discovery"` // "static" or "dynamic"
	CapabilitiesCache json.RawMessage `json:"capabilities_cache,omitempty"`
	IdleTimeoutSec    int             `json:"idle_timeout_sec"`
	MaxInstances      int             `json:"max_instances"`
	RestartPolicy     string          `json:"restart_policy"`
	Disabled          bool            `json:"disabled"`
	Source            string          `json:"source"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

// RouteRule represents a routing rule for matching tool calls to downstream servers.
type RouteRule struct {
	ID                 string          `json:"id"`
	Priority           int             `json:"priority"`
	WorkspaceID        string          `json:"workspace_id"`
	PathGlob           string          `json:"path_glob"`
	ToolMatch          json.RawMessage `json:"tool_match,omitempty"`
	DownstreamServerID string          `json:"downstream_server_id"`
	AuthScopeID        string          `json:"auth_scope_id"`
	Policy             string          `json:"policy"`
	LogLevel           string          `json:"log_level"`
	Source             string          `json:"source"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// Session represents an active or past MCP client session.
type Session struct {
	ID             string     `json:"id"`
	ClientType     string     `json:"client_type"`
	ClientPID      *int       `json:"client_pid,omitempty"`
	ConnectedAt    time.Time  `json:"connected_at"`
	DisconnectedAt *time.Time `json:"disconnected_at,omitempty"`
	WorkspaceID    *string    `json:"workspace_id,omitempty"`
	ModelHint      string     `json:"model_hint"`
}

// AuditRecord represents a single audit log entry.
type AuditRecord struct {
	ID                   string          `json:"id"`
	Timestamp            time.Time       `json:"timestamp"`
	SessionID            string          `json:"session_id"`
	ClientType           string          `json:"client_type"`
	Model                string          `json:"model"`
	WorkspaceID          string          `json:"workspace_id"`
	Subpath              string          `json:"subpath"`
	ToolName             string          `json:"tool_name"`
	ParamsRedacted       json.RawMessage `json:"params_redacted,omitempty"`
	RouteRuleID          string          `json:"route_rule_id"`
	DownstreamServerID   string          `json:"downstream_server_id"`
	DownstreamInstanceID string          `json:"downstream_instance_id"`
	AuthScopeID          string          `json:"auth_scope_id"`
	Status               string          `json:"status"`
	ErrorCode            string          `json:"error_code,omitempty"`
	ErrorMessage         string          `json:"error_message,omitempty"`
	LatencyMs            int             `json:"latency_ms"`
	ResponseSize         int             `json:"response_size"`
	CreatedAt            time.Time       `json:"created_at"`
}

// AuditFilter specifies query parameters for listing audit records.
type AuditFilter struct {
	SessionID   *string    `json:"session_id,omitempty"`
	WorkspaceID *string    `json:"workspace_id,omitempty"`
	ToolName    *string    `json:"tool_name,omitempty"`
	Status      *string    `json:"status,omitempty"`
	After       *time.Time `json:"after,omitempty"`
	Before      *time.Time `json:"before,omitempty"`
	Limit       int        `json:"limit"`
	Offset      int        `json:"offset"`
}

// TimeSeriesPoint holds minute-bucketed aggregate metrics.
type TimeSeriesPoint struct {
	Bucket   time.Time `json:"bucket"`
	Sessions int       `json:"sessions"`
	Servers  int       `json:"servers"`
	Total    int       `json:"total"`
	Errors   int       `json:"errors"`
}

// AuditStats holds aggregate statistics for audit records.
type AuditStats struct {
	TotalRequests int     `json:"total_requests"`
	SuccessCount  int     `json:"success_count"`
	ErrorCount    int     `json:"error_count"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	P95LatencyMs  int     `json:"p95_latency_ms"`
}
