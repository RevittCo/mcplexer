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
import { ExternalLink } from 'lucide-react'
import { useApi } from '@/hooks/use-api'
import { connectDownstream, listOAuthTemplates, listWorkspaces } from '@/api/client'
import type { DownstreamServer, OAuthTemplate } from '@/api/types'

interface ConnectDialogProps {
  open: boolean
  onClose: () => void
  server: DownstreamServer | null
  onConnected: () => void
}

export function ConnectDialog({ open, onClose, server, onConnected }: ConnectDialogProps) {
  const templatesFetcher = useCallback(() => listOAuthTemplates(), [])
  const { data: templates } = useApi(templatesFetcher)

  const workspacesFetcher = useCallback(() => listWorkspaces(), [])
  const { data: workspaces } = useApi(workspacesFetcher)

  const [clientId, setClientId] = useState('')
  const [clientSecret, setClientSecret] = useState('')
  const [workspaceId, setWorkspaceId] = useState('global')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Find matching template for this server.
  const template: OAuthTemplate | null =
    templates?.find((t) => t.id === server?.id) ?? null

  // Reset form when dialog opens with a new server.
  useEffect(() => {
    if (open) {
      setClientId('')
      setClientSecret('')
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
      })
      onConnected()
      onClose()
      if (resp.authorize_url) {
        window.location.href = resp.authorize_url
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to connect')
    } finally {
      setSaving(false)
    }
  }

  if (!server) return null

  return (
    <Dialog open={open} onOpenChange={() => onClose()}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Connect {server.name}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          {template ? (
            <TemplateForm
              template={template}
              clientId={clientId}
              setClientId={setClientId}
              clientSecret={clientSecret}
              setClientSecret={setClientSecret}
            />
          ) : (
            <div className="rounded-md border border-border p-3 space-y-2">
              <p className="text-sm text-muted-foreground">
                This server supports automatic OAuth setup via MCP discovery.
                Click Connect to start the authentication flow.
              </p>
            </div>
          )}

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

        {error && <p className="text-sm text-destructive">{error}</p>}

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button
            onClick={handleSubmit}
            disabled={saving || (template !== null && !clientId)}
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
  template: OAuthTemplate
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
          <Input
            readOnly
            value={template.callback_url}
            className="font-mono text-xs"
            onClick={(e) => (e.target as HTMLInputElement).select()}
          />
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
