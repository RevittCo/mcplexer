import type {
  AuditFilter,
  AuditRecord,
  AuthScope,
  ConnectDownstreamRequest,
  ConnectDownstreamResponse,
  DashboardData,
  DownstreamOAuthSetupResponse,
  DownstreamOAuthStatusResponse,
  DownstreamServer,
  DryRunRequest,
  DryRunResult,
  OAuthCapabilities,
  OAuthProvider,
  OAuthQuickSetupRequest,
  OAuthQuickSetupResponse,
  OAuthStatus,
  OAuthTemplate,
  PaginatedResponse,
  RouteRule,
  ToolApproval,
  Workspace,
} from './types'

const BASE = import.meta.env.VITE_API_BASE_URL || '/api/v1'

class ApiClientError extends Error {
  status: number
  body: string

  constructor(status: number, body: string) {
    super(`API error ${status}: ${body}`)
    this.name = 'ApiClientError'
    this.status = status
    this.body = body
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
  if (!res.ok) {
    const body = await res.text()
    throw new ApiClientError(res.status, body)
  }
  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
}

// Workspaces
export function listWorkspaces(): Promise<Workspace[]> {
  return request('/workspaces')
}

export function getWorkspace(id: string): Promise<Workspace> {
  return request(`/workspaces/${id}`)
}

export function createWorkspace(
  data: Omit<Workspace, 'id' | 'created_at' | 'updated_at'>,
): Promise<Workspace> {
  return request('/workspaces', {
    method: 'POST',
    body: JSON.stringify(data),
  })
}

export function updateWorkspace(
  id: string,
  data: Partial<Omit<Workspace, 'id' | 'created_at' | 'updated_at'>>,
): Promise<Workspace> {
  return request(`/workspaces/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  })
}

export function deleteWorkspace(id: string): Promise<void> {
  return request(`/workspaces/${id}`, { method: 'DELETE' })
}

// Auth Scopes
export function listAuthScopes(): Promise<AuthScope[]> {
  return request('/auth-scopes')
}

export function getAuthScope(id: string): Promise<AuthScope> {
  return request(`/auth-scopes/${id}`)
}

export function createAuthScope(
  data: Omit<AuthScope, 'id' | 'has_secrets' | 'source' | 'created_at' | 'updated_at'>,
): Promise<AuthScope> {
  return request('/auth-scopes', {
    method: 'POST',
    body: JSON.stringify(data),
  })
}

export function updateAuthScope(
  id: string,
  data: Partial<Omit<AuthScope, 'id' | 'created_at' | 'updated_at'>>,
): Promise<AuthScope> {
  return request(`/auth-scopes/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  })
}

export function deleteAuthScope(id: string): Promise<void> {
  return request(`/auth-scopes/${id}`, { method: 'DELETE' })
}

// OAuth Providers
export function listOAuthProviders(): Promise<OAuthProvider[]> {
  return request('/oauth-providers')
}

export function getOAuthProvider(id: string): Promise<OAuthProvider> {
  return request(`/oauth-providers/${id}`)
}

export function createOAuthProvider(
  data: Omit<OAuthProvider, 'id' | 'has_client_secret' | 'source' | 'created_at' | 'updated_at'> & { client_secret?: string },
): Promise<OAuthProvider> {
  return request('/oauth-providers', {
    method: 'POST',
    body: JSON.stringify(data),
  })
}

export function updateOAuthProvider(
  id: string,
  data: Partial<Omit<OAuthProvider, 'id' | 'has_client_secret' | 'source' | 'created_at' | 'updated_at'>>,
): Promise<OAuthProvider> {
  return request(`/oauth-providers/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  })
}

export function deleteOAuthProvider(id: string): Promise<void> {
  return request(`/oauth-providers/${id}`, { method: 'DELETE' })
}

// OAuth Templates
export function listOAuthTemplates(): Promise<OAuthTemplate[]> {
  return request('/oauth-templates')
}

// OIDC Discovery
export function discoverOIDC(issuerURL: string): Promise<{
  authorize_url: string
  token_url: string
  scopes: string[]
  use_pkce: boolean
  issuer: string
}> {
  return request('/oauth-providers/discover', {
    method: 'POST',
    body: JSON.stringify({ issuer_url: issuerURL }),
  })
}

// OAuth Quick Setup
export function oauthQuickSetup(data: OAuthQuickSetupRequest): Promise<OAuthQuickSetupResponse> {
  return request('/auth-scopes/oauth-quick-setup', {
    method: 'POST',
    body: JSON.stringify(data),
  })
}

// OAuth Flow
export function getOAuthAuthorizeURL(scopeId: string): Promise<{ authorize_url: string }> {
  return request(`/auth-scopes/${scopeId}/oauth/authorize`)
}

export function getOAuthStatus(scopeId: string): Promise<OAuthStatus> {
  return request(`/auth-scopes/${scopeId}/oauth/status`)
}

export function revokeOAuthToken(scopeId: string): Promise<void> {
  return request(`/auth-scopes/${scopeId}/oauth/revoke`, { method: 'POST' })
}

// Downstream Servers
export function listDownstreams(): Promise<DownstreamServer[]> {
  return request('/downstreams')
}

export function getDownstream(id: string): Promise<DownstreamServer> {
  return request(`/downstreams/${id}`)
}

export function createDownstream(
  data: Omit<DownstreamServer, 'id' | 'created_at' | 'updated_at' | 'capabilities_cache'>,
): Promise<DownstreamServer> {
  return request('/downstreams', {
    method: 'POST',
    body: JSON.stringify(data),
  })
}

export function updateDownstream(
  id: string,
  data: Partial<Omit<DownstreamServer, 'id' | 'created_at' | 'updated_at'>>,
): Promise<DownstreamServer> {
  return request(`/downstreams/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  })
}

export function deleteDownstream(id: string): Promise<void> {
  return request(`/downstreams/${id}`, { method: 'DELETE' })
}

// Route Rules
export function listRoutes(): Promise<RouteRule[]> {
  return request('/routes')
}

export function getRoute(id: string): Promise<RouteRule> {
  return request(`/routes/${id}`)
}

export function createRoute(
  data: Omit<RouteRule, 'id' | 'created_at' | 'updated_at'>,
): Promise<RouteRule> {
  return request('/routes', {
    method: 'POST',
    body: JSON.stringify(data),
  })
}

export function updateRoute(
  id: string,
  data: Partial<Omit<RouteRule, 'id' | 'created_at' | 'updated_at'>>,
): Promise<RouteRule> {
  return request(`/routes/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  })
}

export function deleteRoute(id: string): Promise<void> {
  return request(`/routes/${id}`, { method: 'DELETE' })
}

export function bulkCreateRoutes(
  rules: Omit<RouteRule, 'id' | 'created_at' | 'updated_at'>[],
): Promise<RouteRule[]> {
  return request('/routes/bulk', {
    method: 'POST',
    body: JSON.stringify(rules),
  })
}

// Audit
export function queryAuditLogs(
  filter: AuditFilter,
): Promise<PaginatedResponse<AuditRecord>> {
  const params = new URLSearchParams()
  if (filter.workspace_id) params.set('workspace_id', filter.workspace_id)
  if (filter.tool_name) params.set('tool_name', filter.tool_name)
  if (filter.status) params.set('status', filter.status)
  if (filter.after) params.set('after', filter.after)
  if (filter.before) params.set('before', filter.before)
  if (filter.limit) params.set('limit', String(filter.limit))
  if (filter.offset) params.set('offset', String(filter.offset))
  return request(`/audit?${params.toString()}`)
}

// Dashboard
export function getDashboard(range_?: string): Promise<DashboardData> {
  const params = range_ ? `?range=${range_}` : ''
  return request(`/dashboard${params}`)
}

// Discover Tools
export function discoverTools(id: string): Promise<DownstreamServer> {
  return request(`/downstreams/${id}/discover`, { method: 'POST' })
}

// Downstream OAuth
export function setupDownstreamOAuth(
  id: string,
  scopeName?: string,
): Promise<DownstreamOAuthSetupResponse> {
  return request(`/downstreams/${id}/oauth-setup`, {
    method: 'POST',
    body: JSON.stringify(scopeName ? { auth_scope_name: scopeName } : {}),
  })
}

// Connect Downstream (unified setup)
export function connectDownstream(
  id: string,
  data: ConnectDownstreamRequest,
): Promise<ConnectDownstreamResponse> {
  return request(`/downstreams/${id}/connect`, {
    method: 'POST',
    body: JSON.stringify(data),
  })
}

export function getDownstreamOAuthStatus(
  id: string,
): Promise<DownstreamOAuthStatusResponse> {
  return request(`/downstreams/${id}/oauth-status`)
}

export function getOAuthCapabilities(
  id: string,
): Promise<OAuthCapabilities> {
  return request(`/downstreams/${id}/oauth-capabilities`)
}

// Approvals
export function listApprovals(status?: string): Promise<ToolApproval[]> {
  const params = new URLSearchParams()
  if (status) params.set('status', status)
  const qs = params.toString()
  return request(`/approvals${qs ? `?${qs}` : ''}`)
}

export function getApproval(id: string): Promise<ToolApproval> {
  return request(`/approvals/${id}`)
}

export function resolveApproval(
  id: string,
  data: { approved: boolean; reason: string },
): Promise<{ status: string }> {
  return request(`/approvals/${id}/resolve`, {
    method: 'POST',
    body: JSON.stringify(data),
  })
}

// Dry Run
export function dryRun(params: DryRunRequest): Promise<DryRunResult> {
  return request('/dry-run', {
    method: 'POST',
    body: JSON.stringify(params),
  })
}
