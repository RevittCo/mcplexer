import { useCallback, useEffect, useState } from 'react'
import {
  Dialog,
  DialogContent,
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

  // Fetch capabilities when dialog opens.
  useEffect(() => {
    if (open && server) fetchCapabilities()
  }, [open, server?.id])

  // Reset form when dialog opens with a new server.
  useEffect(() => {
    if (open) {
      setClientId('')
      setClientSecret('')
      setAccountLabel('')
      setWorkspaceId('global')
      setSaving(false)
      setError(null)
    }
  }, [open, server?.id])

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

  return (
    <Dialog open={open} onOpenChange={() => onClose()}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Connect {server.name}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          {capsLoading && (
            <div className="flex items-center gap-2 py-4 text-sm text-muted-foreground">
              <Loader2 className="h-4 w-4 animate-spin" />
              Checking server capabilities...
            </div>
          )}

          {!capsLoading && capsError && (
            <div className="flex items-start gap-2 rounded-md border border-destructive/30 bg-destructive/5 p-3">
              <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-destructive" />
              <div className="flex-1 text-sm text-destructive">{capsError}</div>
              <Button variant="ghost" size="sm" className="h-7 text-xs" onClick={fetchCapabilities}>
                <RotateCcw className="mr-1 h-3 w-3" /> Retry
              </Button>
            </div>
          )}

          {!capsLoading && !capsError && isAutoDiscovery && (
            <div className="rounded-md border border-border p-3 space-y-2">
              <div className="flex items-center gap-2">
                <Zap className="h-4 w-4 text-emerald-600" />
                <span className="text-sm font-medium">Automatic Setup</span>
              </div>
              <p className="text-sm text-muted-foreground">
                This integration connects automatically. Click Connect and authenticate.
              </p>
            </div>
          )}

          {!capsLoading && !capsError && !isAutoDiscovery && template && (
            <TemplateForm
              template={template}
              clientId={clientId}
              setClientId={setClientId}
              clientSecret={clientSecret}
              setClientSecret={setClientSecret}
            />
          )}

          {!capsLoading && !capsError && !isAutoDiscovery && !template && (
            <div className="rounded-md border border-border p-3 space-y-2">
              <p className="text-sm text-muted-foreground">
                This server supports automatic OAuth setup via MCP discovery.
                Click Connect to start the authentication flow.
              </p>
            </div>
          )}

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

          <div className="space-y-2">
            <Label className="text-xs text-muted-foreground">Workspace</Label>
            <Select value={workspaceId} onValueChange={setWorkspaceId}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(workspaces ?? []).map((ws) => (
                  <SelectItem key={ws.id} value={ws.id}>
                    {ws.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>

        {error && (
          <div className="flex items-start gap-2 rounded-md border border-destructive/30 bg-destructive/5 p-3">
            <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-destructive" />
            <div className="flex-1 text-sm text-destructive">{error}</div>
            <Button variant="ghost" size="sm" className="h-7 text-xs" onClick={handleSubmit}>
              <RotateCcw className="mr-1 h-3 w-3" /> Retry
            </Button>
          </div>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button
            onClick={handleSubmit}
            disabled={saving || capsLoading || !!capsError || (caps?.needs_credentials === true && !clientId)}
          >
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
  setClientId: (v: string) => void
  clientSecret: string
  setClientSecret: (v: string) => void
}) {
  return (
    <div className="space-y-3 rounded-md border border-border p-3">
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
          <Label className="text-xs text-muted-foreground">
            Callback URL (copy this)
          </Label>
          <div className="flex items-center gap-2">
            <code className="flex-1 truncate rounded-md border border-border bg-muted/50 px-2 py-1.5 font-mono text-xs">
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
          <span className="text-xs text-muted-foreground mr-1">Scopes:</span>
          {template.scopes.map((s) => (
            <Badge key={s} variant="secondary" className="font-mono text-xs">
              {s}
            </Badge>
          ))}
        </div>
      )}
    </div>
  )
}
