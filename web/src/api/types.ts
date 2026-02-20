export interface Workspace {
  id: string
  name: string
  root_path: string
  tags: Record<string, string>
  default_policy: 'allow' | 'deny'
  created_at: string
  updated_at: string
}

export interface AuthScope {
  id: string
  name: string
  type: 'env' | 'header' | 'oauth2'
  oauth_provider_id: string
  has_secrets: boolean
  redaction_hints: string[]
  source: string
  created_at: string
  updated_at: string
}

export interface OAuthProvider {
  id: string
  name: string
  template_id: string
  authorize_url: string
  token_url: string
  client_id: string
  has_client_secret: boolean
  scopes: string[]
  use_pkce: boolean
  source: string
  created_at: string
  updated_at: string
}

export interface OAuthTemplate {
  id: string
  name: string
  authorize_url: string
  token_url: string
  scopes: string[]
  use_pkce: boolean
  needs_secret: boolean
  supports_auto_discovery: boolean
  setup_url: string
  help_text: string
  callback_url: string
}

export interface OAuthCapabilities {
  has_template: boolean
  template: OAuthTemplate | null
  supports_auto_discovery: boolean
  needs_credentials: boolean
}

export interface OAuthQuickSetupRequest {
  name: string
  template_id?: string
  provider_id?: string
  client_id?: string
  client_secret?: string
}

export interface OAuthQuickSetupResponse {
  auth_scope: AuthScope
  provider: OAuthProvider
  authorize_url: string
}

export interface OAuthStatus {
  status: 'valid' | 'expired' | 'refresh_needed' | 'not_configured'
  expires_at: string | null
}

export interface ConnectDownstreamRequest {
  workspace_id?: string
  client_id?: string
  client_secret?: string
  scope_name?: string
  account_label?: string
}

export interface ConnectDownstreamResponse {
  auth_scope: AuthScope
  provider: OAuthProvider
  route_rule: RouteRule
  authorize_url: string
}

export interface DownstreamOAuthSetupResponse {
  auth_scope: AuthScope
  provider: OAuthProvider
  authorize_url: string
}

export interface DownstreamOAuthStatusEntry {
  auth_scope_id: string
  auth_scope_name: string
  status: 'authenticated' | 'expired' | 'not_configured'
  expires_at: string | null
  workspace_id?: string
  route_rule_id?: string
}

export interface DownstreamOAuthStatusResponse {
  entries: DownstreamOAuthStatusEntry[]
}

export interface ServerCacheConfig {
  enabled?: boolean
  read_ttl_sec?: number
  cacheable_patterns?: string[]
  mutation_patterns?: string[]
  max_entries?: number
}

export interface DownstreamServer {
  id: string
  name: string
  transport: 'stdio' | 'http'
  command: string
  args: string[]
  url: string | null
  tool_namespace: string
  capabilities_cache: Record<string, unknown>
  cache_config?: ServerCacheConfig
  idle_timeout_sec: number
  max_instances: number
  restart_policy: string
  disabled: boolean
  created_at: string
  updated_at: string
}

export interface RouteRule {
  id: string
  name: string
  priority: number
  workspace_id: string
  path_glob: string
  tool_match: string[]
  downstream_server_id: string
  auth_scope_id: string
  policy: 'allow' | 'deny'
  log_level: string
  requires_approval: boolean
  approval_timeout: number
  created_at: string
  updated_at: string
}

export interface AuditRecord {
  id: string
  timestamp: string
  session_id: string
  client_type: string
  model: string
  workspace_id: string
  workspace_name: string
  subpath: string
  tool_name: string
  params_redacted: Record<string, unknown>
  route_rule_id: string
  downstream_server_id: string
  downstream_instance_id: string
  auth_scope_id: string
  status: 'success' | 'error' | 'blocked'
  error_code: string
  error_message: string
  latency_ms: number
  response_size: number
  cache_hit: boolean
  route_rule_summary?: string
  downstream_server_name?: string
}

export interface AuditFilter {
  workspace_id?: string
  tool_name?: string
  status?: 'success' | 'error' | 'blocked'
  after?: string
  before?: string
  limit?: number
  offset?: number
}

export interface AuditStats {
  total_requests: number
  success_count: number
  error_count: number
  blocked_count: number
  avg_latency_ms: number
  p95_latency_ms: number
}

export interface TimeSeriesPoint {
  bucket: string
  sessions: number
  servers: number
  total: number
  errors: number
  avg_latency_ms: number
}

export interface ToolLeaderboardEntry {
  tool_name: string
  server_name: string
  call_count: number
  error_count: number
  error_rate: number
  avg_latency_ms: number
  p95_latency_ms: number
}

export interface ServerHealthEntry {
  server_id: string
  server_name: string
  call_count: number
  error_count: number
  error_rate: number
  avg_latency_ms: number
  p95_latency_ms: number
}

export interface ErrorBreakdownEntry {
  group_key: string
  server_name: string
  error_type: 'error' | 'blocked'
  count: number
}

export interface RouteHitEntry {
  route_rule_id: string
  rule_name: string
  path_glob: string
  hit_count: number
  error_count: number
}

export interface ApprovalMetrics {
  pending_count: number
  approved_count: number
  denied_count: number
  timed_out_count: number
  avg_wait_ms: number
}

export interface SessionInfo {
  id: string
  client_type: string
  client_pid: number | null
  connected_at: string
  disconnected_at: string | null
  workspace_id: string | null
  model_hint: string
}

export interface CacheLayerStats {
  hits: number
  misses: number
  evictions: number
  entries: number
  hit_rate: number
}

export interface CacheStats {
  tool_call: CacheLayerStats
  route_resolution: CacheLayerStats
}

export interface DashboardData {
  active_sessions: number
  active_session_list: SessionInfo[]
  active_downstreams: DownstreamStatus[]
  recent_errors: AuditRecord[]
  recent_calls: AuditRecord[]
  stats: AuditStats | null
  timeseries: TimeSeriesPoint[]
  tool_leaderboard: ToolLeaderboardEntry[]
  server_health: ServerHealthEntry[]
  error_breakdown: ErrorBreakdownEntry[]
  route_hit_map: RouteHitEntry[]
  approval_metrics: ApprovalMetrics | null
  cache_stats: CacheStats | null
}

export interface DownstreamStatus {
  server_id: string
  server_name: string
  instance_count: number
  state: string
  disabled: boolean
}

export interface DryRunRequest {
  workspace_id: string
  subpath: string
  tool_name: string
}

export interface DryRunAuthScope {
  id: string
  name: string
  type: string
  oauth_status: string // "valid", "expired", "none", "not_applicable"
  expires_at: string | null
}

export interface DryRunResult {
  matched: boolean
  policy: string
  matched_rule: RouteRule | null
  downstream_server: DownstreamServer | null
  auth_scope_id: string
  auth_scope: DryRunAuthScope | null
  candidate_rules: RouteRule[]
}

export interface PaginatedResponse<T> {
  data: T[]
  total: number
}

export interface ToolApproval {
  id: string
  status: 'pending' | 'approved' | 'denied' | 'timeout' | 'cancelled'
  request_session_id: string
  request_client_type: string
  request_model: string
  workspace_id: string
  workspace_name: string
  tool_name: string
  arguments: string
  justification: string
  route_rule_id: string
  downstream_server_id: string
  auth_scope_id: string
  approver_session_id: string
  approver_type: string
  resolution: string
  timeout_sec: number
  created_at: string
  resolved_at: string | null
}

export interface ApprovalEvent {
  type: 'pending' | 'resolved'
  approval: ToolApproval
}

export interface ApiError {
  error: string
  code?: string
}
