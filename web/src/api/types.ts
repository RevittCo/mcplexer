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
  setup_url: string
  help_text: string
  callback_url: string
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
}

export interface DownstreamOAuthStatusResponse {
  entries: DownstreamOAuthStatusEntry[]
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
  idle_timeout_sec: number
  max_instances: number
  restart_policy: string
  disabled: boolean
  created_at: string
  updated_at: string
}

export interface RouteRule {
  id: string
  priority: number
  workspace_id: string
  path_glob: string
  tool_match: string[]
  downstream_server_id: string
  auth_scope_id: string
  policy: 'allow' | 'deny'
  log_level: string
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
  subpath: string
  tool_name: string
  params_redacted: Record<string, unknown>
  route_rule_id: string
  downstream_server_id: string
  downstream_instance_id: string
  auth_scope_id: string
  status: 'success' | 'error'
  error_code: string
  error_message: string
  latency_ms: number
  response_size: number
}

export interface AuditFilter {
  workspace_id?: string
  tool_name?: string
  status?: 'success' | 'error'
  after?: string
  before?: string
  limit?: number
  offset?: number
}

export interface AuditStats {
  total_requests: number
  success_count: number
  error_count: number
  avg_latency_ms: number
  p95_latency_ms: number
}

export interface TimeSeriesPoint {
  bucket: string
  sessions: number
  servers: number
  total: number
  errors: number
}

export interface DashboardData {
  active_sessions: number
  active_downstreams: DownstreamStatus[]
  recent_errors: AuditRecord[]
  recent_calls: AuditRecord[]
  stats: AuditStats | null
  timeseries: TimeSeriesPoint[]
}

export interface DownstreamStatus {
  server_id: string
  server_name: string
  instance_count: number
  state: string
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

export interface ApiError {
  error: string
  code?: string
}
