import { useCallback, useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { CopyButton } from '@/components/ui/copy-button'
import { StepIndicator, type StepDef } from '@/components/ui/step-indicator'
import { useApi } from '@/hooks/use-api'
import {
  connectDownstream,
  getDownstreamOAuthStatus,
  getOAuthCapabilities,
  listDownstreams,
  listWorkspaces,
} from '@/api/client'
import type { DownstreamOAuthStatusEntry, DownstreamServer, OAuthCapabilities } from '@/api/types'
import {
  AlertCircle,
  ArrowLeft,
  ArrowRight,
  CheckCircle2,
  Clock,
  ExternalLink,
  Loader2,
  Plus,
  RotateCcw,
  Zap,
} from 'lucide-react'
import { redirectToOAuth } from '@/lib/safe-redirect'

type Step = 'pick' | 'configure' | 'workspace' | 'review' | 'connecting' | 'success'

function formatRelativeTime(isoDate: string): string {
  const diff = new Date(isoDate).getTime() - Date.now()
  if (diff < 0) return 'expired'
  const days = Math.floor(diff / (1000 * 60 * 60 * 24))
  if (days > 1) return `${days}d`
  const hours = Math.floor(diff / (1000 * 60 * 60))
  if (hours > 0) return `${hours}h`
  return 'soon'
}

function buildSteps(skipsConfig: boolean): StepDef[] {
  const steps: StepDef[] = [{ id: 'pick', label: 'Integration' }]
  if (!skipsConfig) steps.push({ id: 'configure', label: 'Credentials' })
  steps.push({ id: 'workspace', label: 'Workspace' })
  steps.push({ id: 'review', label: 'Review' })
  return steps
}

export function QuickSetupPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const [stepStack, setStepStack] = useState<Step[]>(['pick'])
  const [selectedDs, setSelectedDs] = useState<DownstreamServer | null>(null)
  const [caps, setCaps] = useState<OAuthCapabilities | null>(null)
  const [clientId, setClientId] = useState('')
  const [clientSecret, setClientSecret] = useState('')
  const [workspaceId, setWorkspaceId] = useState('global')
  const [accountLabel, setAccountLabel] = useState('')
  const [connectError, setConnectError] = useState<string | null>(null)
  const [oauthStatuses, setOauthStatuses] = useState<Record<string, DownstreamOAuthStatusEntry[]>>({})
  const [capsCache, setCapsCache] = useState<Record<string, OAuthCapabilities>>({})
  const [statusErrors, setStatusErrors] = useState<Record<string, boolean>>({})

  const step = stepStack[stepStack.length - 1]
  function pushStep(s: Step) { setStepStack((prev) => [...prev, s]) }
  function goBack() { setStepStack((prev) => prev.length > 1 ? prev.slice(0, -1) : prev) }

  const dsFetcher = useCallback(() => listDownstreams(), [])
  const { data: downstreams } = useApi(dsFetcher)
  const wsFetcher = useCallback(() => listWorkspaces(), [])
  const { data: workspaces } = useApi(wsFetcher)

  const httpDownstreams = (downstreams ?? []).filter((d) => d.transport === 'http')

  // Handle OAuth redirect back
  useEffect(() => {
    const oauthResult = searchParams.get('oauth')
    if (oauthResult === 'success') {
      setStepStack(['success'])
      setSearchParams({}, { replace: true })
    } else if (oauthResult === 'error') {
      setConnectError(searchParams.get('message') ?? 'Authentication failed')
      setStepStack(['pick'])
      setSearchParams({}, { replace: true })
    }
  }, [searchParams, setSearchParams])

  // Fetch status & capabilities for all HTTP downstreams
  useEffect(() => {
    if (!downstreams) return
    let active = true
    for (const ds of downstreams) {
      if (ds.transport !== 'http') continue
      getDownstreamOAuthStatus(ds.id)
        .then((res) => { if (active) setOauthStatuses((prev) => ({ ...prev, [ds.id]: res.entries })) })
        .catch(() => { if (active) setStatusErrors((prev) => ({ ...prev, [ds.id]: true })) })
      getOAuthCapabilities(ds.id)
        .then((res) => { if (active) setCapsCache((prev) => ({ ...prev, [ds.id]: res })) })
        .catch(() => {})
    }
    return () => { active = false }
  }, [downstreams])

  const skipsConfig = !!(caps?.supports_auto_discovery && !caps.needs_credentials)
  const completedSteps = stepStack.slice(0, -1)
  const wsName = workspaces?.find((w) => w.id === workspaceId)?.name ?? workspaceId

  function getStatusInfo(dsId: string) {
    const entries = oauthStatuses[dsId]
    const connected = entries?.some((e) => e.status === 'authenticated')
    const expired = entries?.some((e) => e.status === 'expired')
    const expiresAt = entries?.find((e) => e.status === 'authenticated')?.expires_at
    return { connected, expired, expiresAt }
  }

  function pickIntegration(ds: DownstreamServer) {
    setSelectedDs(ds)
    const dsCaps = capsCache[ds.id] ?? null
    setCaps(dsCaps)
    setClientId('')
    setClientSecret('')
    setAccountLabel('')
    setWorkspaceId('global')
    setConnectError(null)
    if (dsCaps?.supports_auto_discovery && !dsCaps.needs_credentials) {
      setStepStack(['pick', 'workspace'])
    } else {
      setStepStack(['pick', 'configure'])
    }
  }

  async function handleConnect() {
    if (!selectedDs) return
    pushStep('connecting')
    setConnectError(null)
    try {
      const resp = await connectDownstream(selectedDs.id, {
        workspace_id: workspaceId,
        client_id: clientId || undefined,
        client_secret: clientSecret || undefined,
        account_label: accountLabel || undefined,
      })
      if (resp.authorize_url) {
        redirectToOAuth(resp.authorize_url)
      }
    } catch (err: unknown) {
      setConnectError(err instanceof Error ? err.message : 'Failed to connect')
      setStepStack((prev) => prev.filter((s) => s !== 'connecting'))
    }
  }

  const STEP_HELP: Record<Step, string> = {
    pick: 'Select an integration to connect to your MCP gateway.',
    configure: 'Enter your OAuth app credentials. We\'ll handle the rest.',
    workspace: 'Choose which workspace this integration will be available in.',
    review: 'Review your setup before connecting.',
    connecting: 'Redirecting to authenticate...',
    success: '',
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center gap-3">
        {step !== 'pick' && step !== 'success' && step !== 'connecting' && (
          <Button variant="ghost" size="sm" className="h-8 w-8 p-0" onClick={goBack}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
        )}
        <h1 className="text-2xl font-bold">Quick Setup</h1>
      </div>

      {/* Step indicator */}
      {step !== 'pick' && step !== 'success' && step !== 'connecting' && (
        <StepIndicator
          steps={buildSteps(skipsConfig)}
          currentStep={step}
          completedSteps={completedSteps}
        />
      )}

      {/* Help text */}
      {STEP_HELP[step] && (
        <p className="text-sm text-muted-foreground">{STEP_HELP[step]}</p>
      )}

      {/* Global error banner */}
      {connectError && step !== 'connecting' && (
        <div className="flex items-start gap-2 rounded-md border border-destructive/30 bg-destructive/5 p-3">
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-destructive" />
          <div className="flex-1 text-sm text-destructive">{connectError}</div>
          <Button variant="ghost" size="sm" className="h-7 text-xs" onClick={() => setConnectError(null)}>
            Dismiss
          </Button>
        </div>
      )}

      {/* Step: Pick Integration */}
      {step === 'pick' && (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {httpDownstreams.map((ds) => {
            const { connected, expired, expiresAt } = getStatusInfo(ds.id)
            const autoDisc = capsCache[ds.id]?.supports_auto_discovery ?? false
            const hasError = statusErrors[ds.id]
            const capsLoaded = !!capsCache[ds.id]
            return (
              <button
                key={ds.id}
                type="button"
                className="relative flex flex-col gap-2 rounded-lg border border-border p-5 text-left transition-all hover:border-primary hover:bg-muted/50 hover:shadow-sm"
                onClick={() => pickIntegration(ds)}
              >
                <div className="absolute right-3 top-3 flex gap-1.5">
                  {hasError && (
                    <Badge variant="outline" className="text-destructive border-destructive/30">
                      <AlertCircle className="mr-1 h-3 w-3" /> Error
                    </Badge>
                  )}
                  {!hasError && connected && (
                    <Badge className="bg-emerald-500/15 text-emerald-600 border-0">
                      <CheckCircle2 className="mr-1 h-3 w-3" />
                      Connected
                      {expiresAt && (
                        <span className="ml-1 opacity-70">
                          <Clock className="mr-0.5 inline h-2.5 w-2.5" />
                          {formatRelativeTime(expiresAt)}
                        </span>
                      )}
                    </Badge>
                  )}
                  {!hasError && expired && (
                    <Badge variant="outline" className="text-amber-600 border-amber-300">
                      Expired — Reconnect
                    </Badge>
                  )}
                  {!hasError && !connected && !expired && autoDisc && capsLoaded && (
                    <Badge className="bg-emerald-500/15 text-emerald-600 border-0">
                      <Zap className="mr-1 h-3 w-3" /> 1-Click
                    </Badge>
                  )}
                  {!hasError && !connected && !expired && !autoDisc && capsLoaded && (
                    <Badge variant="outline" className="text-amber-600 border-amber-300">
                      Credentials Required
                    </Badge>
                  )}
                  {!hasError && !capsLoaded && (
                    <Loader2 className="h-3.5 w-3.5 animate-spin text-muted-foreground/50" />
                  )}
                </div>
                <span className="text-base font-semibold">{ds.name}</span>
                <span className="text-xs text-muted-foreground">{ds.tool_namespace}</span>
                {oauthStatuses[ds.id]?.filter((e) => e.status === 'authenticated').length > 0 && (
                  <div className="flex flex-wrap gap-1 mt-1">
                    {oauthStatuses[ds.id]
                      .filter((e) => e.status === 'authenticated')
                      .map((e) => (
                        <Badge key={e.auth_scope_id} variant="secondary" className="text-xs font-normal">
                          {e.auth_scope_name.replace(/_oauth_?/, ' ').replace(/_/g, ' ').trim() || 'Default'}
                        </Badge>
                      ))}
                  </div>
                )}
              </button>
            )
          })}
          <Link
            to="/config/downstreams"
            className="flex flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-border p-5 text-muted-foreground transition-colors hover:border-primary hover:text-foreground"
          >
            <Plus className="h-5 w-5" />
            <span className="text-sm font-medium">Add Server</span>
          </Link>
        </div>
      )}

      {/* Step: Configure Credentials */}
      {step === 'configure' && selectedDs && caps && (
        <div className="mx-auto max-w-md space-y-4">
          <h2 className="text-lg font-semibold">{selectedDs.name}</h2>
          {caps.has_template && caps.template ? (
            <div className="space-y-3 rounded-md border border-border p-4">
              <p className="text-xs text-muted-foreground">{caps.template.help_text}</p>
              {caps.template.setup_url && (
                <a
                  href={caps.template.setup_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1 text-xs text-primary hover:underline"
                >
                  <ExternalLink className="h-3 w-3" />
                  Open {caps.template.name} developer settings
                </a>
              )}
              {caps.template.callback_url && (
                <div className="space-y-1">
                  <Label className="text-xs text-muted-foreground">Callback URL</Label>
                  <div className="flex items-center gap-2">
                    <code className="flex-1 truncate rounded-md border border-border bg-muted/50 px-2 py-1.5 font-mono text-xs">
                      {caps.template.callback_url}
                    </code>
                    <CopyButton value={caps.template.callback_url} />
                  </div>
                </div>
              )}
              <div className="space-y-1">
                <Label className="text-xs text-muted-foreground">Client ID</Label>
                <Input
                  value={clientId}
                  onChange={(e) => setClientId(e.target.value)}
                  placeholder="Paste your client ID"
                />
              </div>
              {caps.template.needs_secret && (
                <div className="space-y-1">
                  <Label className="text-xs text-muted-foreground">Client Secret</Label>
                  <Input
                    type="password"
                    value={clientSecret}
                    onChange={(e) => setClientSecret(e.target.value)}
                    placeholder="Paste your client secret"
                  />
                </div>
              )}
              {caps.template.scopes.length > 0 && (
                <div className="flex flex-wrap gap-1">
                  <span className="text-xs text-muted-foreground mr-1">Scopes:</span>
                  {caps.template.scopes.map((s) => (
                    <Badge key={s} variant="secondary" className="font-mono text-xs">{s}</Badge>
                  ))}
                </div>
              )}
            </div>
          ) : (
            <div className="rounded-md border border-border p-4">
              <p className="text-sm text-muted-foreground">
                This server supports automatic OAuth setup. Click Next to pick a workspace and connect.
              </p>
            </div>
          )}
          <div className="flex justify-end">
            <Button onClick={() => pushStep('workspace')} disabled={caps.needs_credentials && !clientId}>
              Next <ArrowRight className="ml-2 h-4 w-4" />
            </Button>
          </div>
        </div>
      )}

      {/* Step: Pick Workspace */}
      {step === 'workspace' && selectedDs && (
        <div className="mx-auto max-w-md space-y-4">
          <div className="space-y-2">
            <Label className="text-xs text-muted-foreground">Workspace</Label>
            <Select value={workspaceId} onValueChange={setWorkspaceId}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(workspaces ?? []).map((ws) => (
                  <SelectItem key={ws.id} value={ws.id}>{ws.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className="text-xs text-muted-foreground/60">
              A route rule will be created for this workspace.
            </p>
          </div>
          <div className="space-y-2">
            <Label className="text-xs text-muted-foreground">Account Label (optional)</Label>
            <Input
              value={accountLabel}
              onChange={(e) => setAccountLabel(e.target.value)}
              placeholder="e.g., Personal, Work, Client X"
            />
            <p className="text-xs text-muted-foreground/60">
              Label this account to connect multiple accounts for the same service.
            </p>
          </div>
          <div className="flex justify-end">
            <Button onClick={() => pushStep('review')}>
              Next <ArrowRight className="ml-2 h-4 w-4" />
            </Button>
          </div>
        </div>
      )}

      {/* Step: Review */}
      {step === 'review' && selectedDs && (
        <div className="mx-auto max-w-md space-y-4">
          <div className="rounded-md border border-border divide-y divide-border">
            <div className="flex justify-between p-3">
              <span className="text-xs text-muted-foreground">Integration</span>
              <span className="text-sm font-medium">{selectedDs.name}</span>
            </div>
            <div className="flex justify-between p-3">
              <span className="text-xs text-muted-foreground">Workspace</span>
              <span className="text-sm font-medium">{wsName}</span>
            </div>
            <div className="flex justify-between p-3">
              <span className="text-xs text-muted-foreground">Auth Method</span>
              <span className="text-sm font-medium">
                {skipsConfig ? 'Auto-discovery' : caps?.has_template ? 'OAuth Template' : 'OAuth'}
              </span>
            </div>
            {accountLabel && (
              <div className="flex justify-between p-3">
                <span className="text-xs text-muted-foreground">Account</span>
                <span className="text-sm font-medium">{accountLabel}</span>
              </div>
            )}
            {clientId && (
              <div className="flex justify-between p-3">
                <span className="text-xs text-muted-foreground">Client ID</span>
                <span className="truncate ml-4 max-w-[180px] font-mono text-xs text-muted-foreground">
                  {clientId}
                </span>
              </div>
            )}
          </div>
          <p className="text-xs text-muted-foreground/60">
            This will create a credential set and route rule, then redirect you to authenticate.
          </p>
          <div className="flex justify-end gap-2">
            <Button variant="outline" onClick={goBack}>Back</Button>
            <Button onClick={handleConnect}>
              <Zap className="mr-2 h-4 w-4" /> Connect
            </Button>
          </div>
        </div>
      )}

      {/* Step: Connecting */}
      {step === 'connecting' && (
        <div className="flex flex-col items-center justify-center py-16 text-muted-foreground">
          <Loader2 className="mb-4 h-8 w-8 animate-spin text-primary" />
          <p className="text-sm">Connecting to {selectedDs?.name}...</p>
          <p className="mt-1 text-xs text-muted-foreground/60">You will be redirected to authenticate.</p>
        </div>
      )}

      {/* Step: Success */}
      {step === 'success' && (
        <div className="mx-auto flex max-w-md flex-col items-center py-12 text-center">
          <div className="mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-emerald-500/10">
            <CheckCircle2 className="h-8 w-8 text-emerald-600" />
          </div>
          <h2 className="text-xl font-semibold">Connected!</h2>
          <p className="mt-2 text-sm text-muted-foreground">
            Your integration has been authenticated and is ready to use.
          </p>
          <div className="mt-6 flex gap-3">
            <Button variant="outline" asChild>
              <Link to="/">Back to Dashboard</Link>
            </Button>
            <Button onClick={() => {
              setStepStack(['pick'])
              setSelectedDs(null)
              setCaps(null)
              setConnectError(null)
            }}>
              <RotateCcw className="mr-2 h-4 w-4" /> Connect Another
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
