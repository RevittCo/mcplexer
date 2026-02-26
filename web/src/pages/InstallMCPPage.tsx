import { useCallback, useState } from 'react'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { useApi } from '@/hooks/use-api'
import {
  getMCPInstallStatus,
  installMCP,
  previewMCPInstall,
  uninstallMCP,
} from '@/api/client'
import type { MCPClient, MCPInstallPreview } from '@/api/types'
import { toast } from 'sonner'
import {
  ClipboardCopy,
  Download,
  Eye,
  Loader2,
  Trash2,
} from 'lucide-react'
import { CopyButton } from '@/components/ui/copy-button'
import { cn } from '@/lib/utils'

const CLIENT_ICONS: Record<string, string> = {
  claude_desktop: '🖥️',
  claude_code: '⌨️',
  cursor: '▲',
  windsurf: '🏄',
  codex: '📦',
  opencode: '🔓',
  gemini_cli: '✦',
}

export function InstallMCPPage() {
  const statusFetcher = useCallback(() => getMCPInstallStatus(), [])
  const { data: status, loading, refetch } = useApi(statusFetcher)

  const [dialogClient, setDialogClient] = useState<MCPClient | null>(null)
  const [preview, setPreview] = useState<MCPInstallPreview | null>(null)
  const [previewLoading, setPreviewLoading] = useState(false)
  const [installing, setInstalling] = useState<string | null>(null)

  async function handleInstallClick(client: MCPClient) {
    setDialogClient(client)
    setPreview(null)
    setPreviewLoading(true)
    try {
      const p = await previewMCPInstall(client.id)
      setPreview(p)
    } catch {
      setPreview(null)
    } finally {
      setPreviewLoading(false)
    }
  }

  async function handleConfirmInstall() {
    if (!dialogClient) return
    setInstalling(dialogClient.id)
    try {
      await installMCP(dialogClient.id)
      toast.success(`Installed. Restart ${dialogClient.name} to pick up changes.`)
      refetch()
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Install failed'
      toast.error(message)
    } finally {
      setInstalling(null)
      setDialogClient(null)
    }
  }

  async function handleUninstall(client: MCPClient) {
    setInstalling(client.id)
    try {
      await uninstallMCP(client.id)
      toast.success(`Removed from ${client.name}.`)
      refetch()
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Uninstall failed'
      toast.error(message)
    } finally {
      setInstalling(null)
    }
  }

  const serverEntryJSON = status
    ? JSON.stringify({ mcpServers: { mx: status.server_entry } }, null, 2)
    : ''

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Install MCP</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Configure MCPlexer as an MCP server in your AI tools.
        </p>
      </div>

      {loading ? (
        <div className="flex items-center justify-center py-16">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </div>
      ) : (
        <>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {status?.clients.map((client) => (
              <ToolCard
                key={client.id}
                client={client}
                installing={installing === client.id}
                onInstall={() => handleInstallClick(client)}
                onUninstall={() => handleUninstall(client)}
              />
            ))}

            {/* Other Tools — raw JSON card */}
            <Card>
              <CardContent className="flex flex-col gap-3 pt-5">
                <div className="flex items-start justify-between">
                  <div className="flex items-center gap-2.5">
                    <ClipboardCopy className="h-[18px] w-[18px] text-muted-foreground" />
                    <div>
                      <p className="text-sm font-medium leading-none">Other</p>
                      <p className="mt-1 text-[11px] text-muted-foreground">
                        Copy into your MCP client&apos;s config
                      </p>
                    </div>
                  </div>
                  <CopyButton value={serverEntryJSON} />
                </div>
                <div className="relative rounded-md border border-border bg-muted/30">
                  <pre className="overflow-x-auto p-3 font-mono text-[10px] leading-relaxed text-foreground max-h-48">
                    {serverEntryJSON}
                  </pre>
                </div>
              </CardContent>
            </Card>
          </div>
        </>
      )}

      {/* Install confirmation dialog */}
      <Dialog open={!!dialogClient} onOpenChange={(open) => !open && setDialogClient(null)}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>Install to {dialogClient?.name}</DialogTitle>
            <DialogDescription>
              This will add the MCPlexer server entry to{' '}
              <code className="rounded bg-muted px-1 py-0.5 text-xs font-mono">
                {dialogClient?.config_path}
              </code>
            </DialogDescription>
          </DialogHeader>

          <div className="max-h-72 overflow-auto rounded-md border border-border bg-muted/30">
            {previewLoading ? (
              <div className="flex items-center justify-center py-8">
                <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
              </div>
            ) : preview ? (
              <pre className="p-4 font-mono text-xs leading-relaxed text-foreground">
                {preview.content}
              </pre>
            ) : (
              <p className="p-4 text-sm text-muted-foreground">
                Failed to load preview.
              </p>
            )}
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogClient(null)}>
              Cancel
            </Button>
            <Button
              onClick={handleConfirmInstall}
              disabled={installing !== null}
            >
              {installing ? (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              ) : (
                <Download className="mr-2 h-4 w-4" />
              )}
              Install
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function ToolCard({
  client,
  installing,
  onInstall,
  onUninstall,
}: {
  client: MCPClient
  installing: boolean
  onInstall: () => void
  onUninstall: () => void
}) {
  const icon = CLIENT_ICONS[client.id] ?? '🔌'

  return (
    <Card
      className={cn(
        'transition-colors',
        !client.detected && 'opacity-50',
      )}
    >
      <CardContent className="flex flex-col gap-3 pt-5">
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-2.5">
            <span className="text-lg">{icon}</span>
            <div>
              <p className="text-sm font-medium leading-none">{client.name}</p>
              <p className="mt-1 font-mono text-[11px] text-muted-foreground truncate max-w-[200px]">
                {client.config_path || 'N/A'}
              </p>
            </div>
          </div>
          <StatusBadge client={client} />
        </div>

        <div className="flex gap-2">
          {!client.detected ? (
            <Button variant="outline" size="sm" disabled className="w-full text-xs">
              Not Found
            </Button>
          ) : client.configured ? (
            <Button
              variant="outline"
              size="sm"
              className="w-full text-xs"
              disabled={installing}
              onClick={onUninstall}
            >
              {installing ? (
                <Loader2 className="mr-1.5 h-3 w-3 animate-spin" />
              ) : (
                <Trash2 className="mr-1.5 h-3 w-3" />
              )}
              Uninstall
            </Button>
          ) : (
            <Button
              size="sm"
              className="w-full text-xs"
              disabled={installing}
              onClick={onInstall}
            >
              {installing ? (
                <Loader2 className="mr-1.5 h-3 w-3 animate-spin" />
              ) : (
                <Eye className="mr-1.5 h-3 w-3" />
              )}
              Install
            </Button>
          )}
        </div>
      </CardContent>
    </Card>
  )
}

function StatusBadge({ client }: { client: MCPClient }) {
  if (client.configured) {
    return (
      <Badge className="border-0 bg-emerald-500/15 text-emerald-600 text-[10px]">
        Installed
      </Badge>
    )
  }
  if (client.detected) {
    return (
      <Badge variant="outline" className="text-[10px]">
        Available
      </Badge>
    )
  }
  return (
    <Badge variant="outline" className="text-[10px] text-muted-foreground border-muted">
      Not Found
    </Badge>
  )
}
