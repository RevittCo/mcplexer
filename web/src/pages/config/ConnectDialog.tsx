import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Badge } from '@/components/ui/badge'
import { AlertCircle, ExternalLink, Loader2, RotateCcw, Zap } from 'lucide-react'
import { useApi } from '@/hooks/use-api'
import { connectDownstream, getOAuthCapabilities, listWorkspaces } from '@/api/client'
import type { DownstreamServer, OAuthCapabilities } from '@/api/types'
import { toast } from 'sonner'
import { CopyButton } from '@/components/ui/copy-button'
import { redirectToOAuth } from '@/lib/safe-redirect'

interface ConnectDialogProps {
  open: boolean
  onClose: () => void
  server: DownstreamServer | null
  onConnected: () => void
}

export function ConnectDialog({ open, onClose, server, onConnected }: ConnectDialogProps) {
  const workspacesFetcher = useCallback(() => listWorkspaces(), [])
  const { data: workspaces } = useApi(workspacesFetcher)

  const [caps, setCaps] = useState<OAuthCapabilities | null>(null)
  const [capsLoading, setCapsLoading] = useState(false)
  const [capsError, setCapsError] = useState<string | null>(null)
  const [clientId, setClientId] = useState('')
  const [clientSecret, setClientSecret] = useState('')
  const [accountLabel, setAccountLabel] = useState('')
  const [workspaceId, setWorkspaceId] = useState('global')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  function fetchCapabilities() {
    if (!server) return
    setCapsLoading(true)
    setCaps(null)
    setCapsError(null)
    getOAuthCapabilities(server.id)
      .then(setCaps)
      .catch(() => setCapsError('Failed to check server capabilities. The server may be offline.'))
      .finally(() => setCapsLoading(false))
  }

  useEffect(() => {
    if (open && server) fetchCapabilities()
  }, [open, server?.id])

  useEffect(() => {
    if (!open) return
    setClientId('')
    setClientSecret('')
    setAccountLabel('')
    setWorkspaceId('global')
    setSaving(false)
    setError(null)
  }, [open, server?.id])

  useEffect(() => {
    if (!open || !workspaces || workspaces.length === 0) return
    if (!workspaces.some((workspace) => workspace.id === workspaceId)) {
      setWorkspaceId(workspaces[0].id)
    }
  }, [open, workspaceId, workspaces])

  async function handleSubmit() {
    if (!server) return
    setSaving(true)
    setError(null)
    try {
      const resp = await connectDownstream(server.id, {
        workspace_id: workspaceId,
        client_id: clientId || undefined,
        client_secret: clientSecret || undefined,
        account_label: accountLabel || undefined,
      })
      toast.success(`${server.name} connected`)
      onConnected()
      onClose()
      if (resp.authorize_url) {
        redirectToOAuth(resp.authorize_url)
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to connect')
    } finally {
      setSaving(false)
    }
  }

  if (!server) return null

  const isAutoDiscovery = caps?.supports_auto_discovery && !caps.needs_credentials
  const template = caps?.template ?? null
  const selectedWorkspace = workspaces?.find((workspace) => workspace.id === workspaceId)
  const authModeLabel = useMemo(() => {
    if (capsLoading) return 'Checking capabilities'
    if (isAutoDiscovery) return 'Automatic OAuth discovery'
    if (template) return `${template.name} OAuth app`
    if (capsError) return 'Unavailable'
    return 'Manual OAuth'
  }, [capsError, capsLoading, isAutoDiscovery, template])
  const canSubmit =
    !saving &&
    !capsLoading &&
    !capsError &&
    !!workspaceId &&
    (caps?.needs_credentials !== true || !!clientId.trim())

  return (
    <Dialog open={open} onOpenChange={() => onClose()}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Connect {server.name}</DialogTitle>
          <DialogDescription>
            Configure OAuth for this downstream server. MCPlexer will create or reuse a credential
            scope and route rule for the workspace you choose.
          </DialogDescription>
        </DialogHeader>

        <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_18rem]">
          <div className="space-y-4">
            {capsLoading && (
              <div className="flex items-center gap-2 rounded-md border border-border/50 bg-muted/30 px-3 py-4 text-sm text-muted-foreground">
                <Loader2 className="h-4 w-4 animate-spin" />
                Checking server capabilities...
              </div>
            )}

            {!capsLoading && capsError && (
              <div className="flex items-start gap-2 rounded-md border border-destructive/30 bg-destructive/5 p-3">
                <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-destructive" />
                <div className="flex-1 text-sm text-destructive">{capsError}</div>
                <Button variant="ghost" size="sm" className="h-7 text-xs" onClick={fetchCapabilities}>
                  <RotateCcw className="mr-1 h-3 w-3" />
                  Retry
                </Button>
              </div>
            )}

            {!capsLoading && !capsError && (
              <div className="rounded-md border border-border/50 p-4">
                <div className="flex flex-wrap items-center gap-2">
                  <Badge variant="outline" className="font-mono text-xs">
                    {server.tool_namespace}
                  </Badge>
                  {isAutoDiscovery && (
                    <Badge className="border-0 bg-emerald-500/15 text-emerald-600">
                      <Zap className="mr-1 h-3 w-3" />
                      Automatic setup
                    </Badge>
                  )}
                </div>

                <div className="mt-3 space-y-2">
                  {isAutoDiscovery && (
                    <p className="text-sm text-muted-foreground">
                      This integration can discover its OAuth server automatically. Click Connect,
                      then authenticate in the browser.
                    </p>
                  )}

                  {!isAutoDiscovery && template && (
                    <TemplateForm
                      template={template}
                      clientId={clientId}
                      setClientId={setClientId}
                      clientSecret={clientSecret}
                      setClientSecret={setClientSecret}
                    />
                  )}

                  {!isAutoDiscovery && !template && (
                    <p className="text-sm text-muted-foreground">
                      This server exposes OAuth, but it does not have a saved template. MCPlexer
                      will try discovery or use credentials you provide.
                    </p>
                  )}
                </div>
              </div>
            )}

            <div className="grid gap-4 md:grid-cols-2">
              <div className="space-y-2">
                <Label className="text-xs text-muted-foreground">Workspace</Label>
                <Select value={workspaceId} onValueChange={setWorkspaceId}>
                  <SelectTrigger>
                    <SelectValue placeholder="Select workspace..." />
                  </SelectTrigger>
                  <SelectContent>
                    {(workspaces ?? []).map((workspace) => (
                      <SelectItem key={workspace.id} value={workspace.id}>
                        {workspace.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <p className="text-xs text-muted-foreground/70">
                  MCPlexer creates or reuses a route rule in this workspace.
                </p>
              </div>

              <div className="space-y-2">
                <Label className="text-xs text-muted-foreground">Account Label (optional)</Label>
                <Input
                  value={accountLabel}
                  onChange={(e) => setAccountLabel(e.target.value)}
                  placeholder="e.g. Personal, Work, Client X"
                />
                <p className="text-xs text-muted-foreground/70">
                  Use a label when you need multiple accounts for the same integration.
                </p>
              </div>
            </div>
          </div>

          <div className="space-y-3 rounded-md border border-border/50 bg-muted/20 p-4">
            <div className="space-y-1">
              <p className="text-xs uppercase tracking-wide text-muted-foreground">Summary</p>
              <p className="text-sm font-medium">{server.name}</p>
            </div>

            <div className="space-y-2 text-sm">
              <SummaryRow
                label="Workspace"
                value={selectedWorkspace?.name ?? workspaceId ?? 'Select one'}
              />
              <SummaryRow label="Auth mode" value={authModeLabel} />
              <SummaryRow label="Credential" value={accountLabel || `${server.tool_namespace}_oauth`} />
            </div>

            <div className="rounded-md border border-border/50 bg-background/40 px-3 py-2 text-xs text-muted-foreground">
              Connect will save the route, create or reuse the OAuth scope, then redirect you to
              authenticate.
            </div>
          </div>
        </div>

        {error && (
          <div className="flex items-start gap-2 rounded-md border border-destructive/30 bg-destructive/5 p-3">
            <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-destructive" />
            <div className="flex-1 text-sm text-destructive">{error}</div>
            <Button variant="ghost" size="sm" className="h-7 text-xs" onClick={handleSubmit}>
              <RotateCcw className="mr-1 h-3 w-3" />
              Retry
            </Button>
          </div>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={handleSubmit} disabled={!canSubmit}>
            {saving ? 'Connecting...' : 'Connect'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function TemplateForm({
  template,
  clientId,
  setClientId,
  clientSecret,
  setClientSecret,
}: {
  template: NonNullable<OAuthCapabilities['template']>
  clientId: string
  setClientId: (value: string) => void
  clientSecret: string
  setClientSecret: (value: string) => void
}) {
  return (
    <div className="space-y-3 rounded-md border border-border/50 bg-muted/20 p-3">
      <p className="text-xs text-muted-foreground">{template.help_text}</p>

      {template.setup_url && (
        <a
          href={template.setup_url}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-1 text-xs text-primary hover:underline"
        >
          <ExternalLink className="h-3 w-3" />
          Open {template.name} developer settings
        </a>
      )}

      {template.callback_url && (
        <div className="space-y-1">
          <Label className="text-xs text-muted-foreground">Callback URL</Label>
          <div className="flex items-center gap-2">
            <code className="flex-1 truncate rounded-md border border-border bg-background/60 px-2 py-1.5 font-mono text-xs">
              {template.callback_url}
            </code>
            <CopyButton value={template.callback_url} />
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

      {template.needs_secret && (
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

      {template.scopes.length > 0 && (
        <div className="flex flex-wrap gap-1">
          <span className="mr-1 text-xs text-muted-foreground">Scopes:</span>
          {template.scopes.map((scope) => (
            <Badge key={scope} variant="secondary" className="font-mono text-xs">
              {scope}
            </Badge>
          ))}
        </div>
      )}
    </div>
  )
}

function SummaryRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-start justify-between gap-3">
      <span className="text-muted-foreground">{label}</span>
      <span className="max-w-[12rem] text-right">{value}</span>
    </div>
  )
}
