import { useCallback, useState } from 'react'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { useApi } from '@/hooks/use-api'
import {
  createAuthScope,
  deleteAuthScope,
  getOAuthAuthorizeURL,
  getOAuthStatus,
  listAuthScopes,
  listOAuthProviders,
  revokeOAuthToken,
  updateAuthScope,
} from '@/api/client'
import type { AuthScope, OAuthStatus } from '@/api/types'
import { Copy, ExternalLink, Lock, Pencil, Plus, Trash2, Unplug } from 'lucide-react'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { toast } from 'sonner'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { AuthScopeDialog, emptyAuthScopeForm } from './AuthScopeDialog'
import type { AuthScopeFormData } from './AuthScopeDialog'

export function AuthScopesPage() {
  const fetcher = useCallback(() => listAuthScopes(), [])
  const { data, loading, error, refetch } = useApi(fetcher)

  const providersFetcher = useCallback(() => listOAuthProviders(), [])
  const { data: providers } = useApi(providersFetcher)

  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<AuthScope | null>(null)
  const [form, setForm] = useState<AuthScopeFormData>(emptyAuthScopeForm)
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<AuthScope | null>(null)

  function openCreate() {
    setEditing(null)
    setForm(emptyAuthScopeForm)
    setSaveError(null)
    setDialogOpen(true)
  }

  function openEdit(scope: AuthScope) {
    setEditing(scope)
    setForm({
      name: scope.name,
      type: scope.type,
      oauth_provider_id: scope.oauth_provider_id ?? '',
      redaction_hints: [...(scope.redaction_hints ?? [])],
    })
    setSaveError(null)
    setDialogOpen(true)
  }

  async function handleSave() {
    setSaving(true)
    setSaveError(null)
    try {
      if (editing) {
        await updateAuthScope(editing.id, form)
      } else {
        await createAuthScope(form)
      }
      setDialogOpen(false)
      toast.success(editing ? 'Credential updated' : 'Credential created')
      refetch()
    } catch (err: unknown) {
      setSaveError(err instanceof Error ? err.message : 'Failed to save credential')
    } finally {
      setSaving(false)
    }
  }

  async function confirmDelete() {
    if (!deleteTarget) return
    try {
      await deleteAuthScope(deleteTarget.id)
      setDeleteTarget(null)
      toast.success('Credential deleted')
      refetch()
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : 'Failed to delete credential')
    }
  }

  function handleDuplicate(scope: AuthScope) {
    setEditing(null)
    setForm({
      name: `${scope.name} (copy)`,
      type: scope.type,
      oauth_provider_id: scope.oauth_provider_id ?? '',
      redaction_hints: [...(scope.redaction_hints ?? [])],
    })
    setDialogOpen(true)
  }

  async function handleAuthenticate(scopeId: string) {
    try {
      const { authorize_url } = await getOAuthAuthorizeURL(scopeId)
      window.location.href = authorize_url
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : 'Failed to start authentication')
    }
  }

  async function handleRevoke(scopeId: string) {
    try {
      await revokeOAuthToken(scopeId)
      toast.success('Token revoked')
      refetch()
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : 'Failed to revoke token')
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Credentials</h1>
        <Button onClick={openCreate}>
          <Plus className="mr-2 h-4 w-4" />
          Add Credential
        </Button>
      </div>

      <Card>
        <CardContent className="pt-6">
          {loading && !data && (
            <div className="flex items-center gap-2 text-muted-foreground">
              <div className="h-2 w-2 animate-pulse rounded-full bg-primary" />
              Loading...
            </div>
          )}
          {error && <p className="text-destructive">Error: {error}</p>}
          {data && (
            <Table>
              <TableHeader>
                <TableRow className="border-border/50 hover:bg-transparent">
                  <TableHead>Name</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead className="hidden sm:table-cell">OAuth Status</TableHead>
                  <TableHead className="hidden md:table-cell">Redaction Hints</TableHead>
                  <TableHead>Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={5} className="h-32">
                      <div className="flex flex-col items-center justify-center text-muted-foreground">
                        <Lock className="mb-2 h-8 w-8 text-muted-foreground/50" />
                        <p className="text-sm">No credentials configured</p>
                        <button onClick={openCreate} className="text-xs text-primary hover:underline">
                          Add a credential to get started
                        </button>
                      </div>
                    </TableCell>
                  </TableRow>
                ) : (
                  data.map((scope) => (
                    <TableRow key={scope.id} className="border-border/30 hover:bg-muted/30">
                      <TableCell className="font-medium">{scope.name}</TableCell>
                      <TableCell>
                        <Badge variant="outline" className="font-mono text-xs">
                          {scope.type}
                        </Badge>
                      </TableCell>
                      <TableCell className="hidden sm:table-cell">
                        {scope.type === 'oauth2' ? (
                          <OAuthStatusBadge scopeId={scope.id} />
                        ) : (
                          <span className="text-muted-foreground">-</span>
                        )}
                      </TableCell>
                      <TableCell className="hidden md:table-cell">
                        {(scope.redaction_hints ?? []).length > 0 ? (
                          <div className="flex max-w-[12rem] flex-wrap gap-1 overflow-hidden max-h-12">
                            {(scope.redaction_hints ?? []).map((hint) => (
                              <Badge
                                key={hint}
                                variant="secondary"
                                className="font-mono text-xs"
                              >
                                {hint}
                              </Badge>
                            ))}
                          </div>
                        ) : (
                          <span className="text-muted-foreground">-</span>
                        )}
                      </TableCell>
                      <TableCell>
                        <div className="flex gap-0.5">
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Button
                                variant="ghost"
                                size="sm"
                                className="h-7 w-7 p-0"
                                onClick={() => handleDuplicate(scope)}
                              >
                                <Copy className="h-3.5 w-3.5" />
                              </Button>
                            </TooltipTrigger>
                            <TooltipContent>Duplicate</TooltipContent>
                          </Tooltip>
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Button
                                variant="ghost"
                                size="sm"
                                className="h-7 w-7 p-0"
                                onClick={() => openEdit(scope)}
                              >
                                <Pencil className="h-3.5 w-3.5" />
                              </Button>
                            </TooltipTrigger>
                            <TooltipContent>Edit</TooltipContent>
                          </Tooltip>
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Button
                                variant="ghost"
                                size="sm"
                                className="h-7 w-7 p-0 hover:bg-destructive/10 hover:text-destructive"
                                onClick={() => setDeleteTarget(scope)}
                              >
                                <Trash2 className="h-3.5 w-3.5" />
                              </Button>
                            </TooltipTrigger>
                            <TooltipContent>Delete</TooltipContent>
                          </Tooltip>
                          {scope.type === 'oauth2' && (
                            <>
                              <Tooltip>
                                <TooltipTrigger asChild>
                                  <Button
                                    variant="ghost"
                                    size="sm"
                                    className="h-7 w-7 p-0 text-primary hover:bg-primary/10"
                                    onClick={() => handleAuthenticate(scope.id)}
                                  >
                                    <ExternalLink className="h-3.5 w-3.5" />
                                  </Button>
                                </TooltipTrigger>
                                <TooltipContent>Authenticate</TooltipContent>
                              </Tooltip>
                              <Tooltip>
                                <TooltipTrigger asChild>
                                  <Button
                                    variant="ghost"
                                    size="sm"
                                    className="h-7 w-7 p-0 hover:bg-amber-500/10 hover:text-amber-600"
                                    onClick={() => handleRevoke(scope.id)}
                                  >
                                    <Unplug className="h-3.5 w-3.5" />
                                  </Button>
                                </TooltipTrigger>
                                <TooltipContent>Revoke Token</TooltipContent>
                              </Tooltip>
                            </>
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <AuthScopeDialog
        open={dialogOpen}
        onClose={() => setDialogOpen(false)}
        form={form}
        setForm={setForm}
        onSave={handleSave}
        saving={saving}
        editing={!!editing}
        providers={providers ?? []}
        saveError={saveError}
      />

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title="Delete credential"
        description={`Are you sure you want to delete "${deleteTarget?.name}"?`}
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={confirmDelete}
      />
    </div>
  )
}

function formatRelativeTime(isoDate: string): string {
  const diff = new Date(isoDate).getTime() - Date.now()
  const days = Math.floor(diff / (1000 * 60 * 60 * 24))
  if (days > 1) return `${days} days`
  const hours = Math.floor(diff / (1000 * 60 * 60))
  if (hours > 0) return `${hours} hours`
  return 'soon'
}

function OAuthStatusBadge({ scopeId }: { scopeId: string }) {
  const fetcher = useCallback(() => getOAuthStatus(scopeId), [scopeId])
  const { data: status } = useApi<OAuthStatus>(fetcher)

  if (!status) return <span className="text-muted-foreground">...</span>

  const colors: Record<string, string> = {
    valid: 'bg-emerald-500/10 text-emerald-600 border-emerald-500/20',
    expired: 'bg-yellow-500/10 text-yellow-600 border-yellow-500/20',
    refresh_needed: 'bg-yellow-500/10 text-yellow-600 border-yellow-500/20',
    not_configured: 'bg-muted text-muted-foreground border-border',
  }

  let label = ''
  switch (status.status) {
    case 'valid':
      label = status.expires_at
        ? `Connected \u2014 ${formatRelativeTime(status.expires_at)} left`
        : 'Connected'
      break
    case 'expired':
      label = 'Expired'
      break
    case 'refresh_needed':
      label = 'Needs Refresh'
      break
    default:
      label = 'Not Connected'
  }

  return (
    <Badge variant="outline" className={`text-xs ${colors[status.status] ?? colors.not_configured}`}>
      {label}
    </Badge>
  )
}
