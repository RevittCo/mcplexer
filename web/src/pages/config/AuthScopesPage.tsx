import { useCallback, useState } from 'react'
import { Card, CardContent } from '@/components/ui/card'
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Badge } from '@/components/ui/badge'
import { useApi } from '@/hooks/use-api'
import {
  createAuthScope,
  deleteAuthScope,
  getOAuthAuthorizeURL,
  getOAuthStatus,
  listAuthScopes,
  listOAuthProviders,
  listOAuthTemplates,
  oauthQuickSetup,
  revokeOAuthToken,
  updateAuthScope,
} from '@/api/client'
import type { AuthScope, OAuthStatus, OAuthTemplate } from '@/api/types'
import { Copy, ExternalLink, Lock, Pencil, Plus, Trash2, Unplug } from 'lucide-react'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

interface FormData {
  name: string
  type: 'env' | 'header' | 'oauth2'
  oauth_provider_id: string
  redaction_hints: string[]
}

const emptyForm: FormData = {
  name: '',
  type: 'env',
  oauth_provider_id: '',
  redaction_hints: [],
}

export function AuthScopesPage() {
  const fetcher = useCallback(() => listAuthScopes(), [])
  const { data, loading, error, refetch } = useApi(fetcher)

  const providersFetcher = useCallback(() => listOAuthProviders(), [])
  const { data: providers } = useApi(providersFetcher)

  const templatesFetcher = useCallback(() => listOAuthTemplates(), [])
  const { data: templates } = useApi(templatesFetcher)

  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<AuthScope | null>(null)
  const [form, setForm] = useState<FormData>(emptyForm)
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)

  function openCreate() {
    setEditing(null)
    setForm(emptyForm)
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
      refetch()
    } catch (err: unknown) {
      setSaveError(err instanceof Error ? err.message : 'Failed to save auth scope')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(id: string) {
    try {
      await deleteAuthScope(id)
      refetch()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to delete auth scope'
      alert(msg)
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
      const msg = err instanceof Error ? err.message : 'Failed to start authentication'
      alert(msg)
    }
  }

  async function handleRevoke(scopeId: string) {
    try {
      await revokeOAuthToken(scopeId)
      refetch()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to revoke token'
      alert(msg)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Auth Scopes</h1>
        <Button onClick={openCreate}>
          <Plus className="mr-2 h-4 w-4" />
          Add Auth Scope
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
                        <p className="text-sm">No auth scopes configured</p>
                        <p className="text-xs text-muted-foreground/60">
                          Add an auth scope for credential injection
                        </p>
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
                          {scope.type === 'oauth2' && (
                            <>
                              <Tooltip>
                                <TooltipTrigger asChild>
                                  <Button
                                    variant="ghost"
                                    size="sm"
                                    className="h-7 w-7 p-0"
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
                                onClick={() => handleDelete(scope.id)}
                              >
                                <Trash2 className="h-3.5 w-3.5" />
                              </Button>
                            </TooltipTrigger>
                            <TooltipContent>Delete</TooltipContent>
                          </Tooltip>
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
        templates={templates ?? []}
        saveError={saveError}
        onQuickSetup={async (name, templateId, clientId, clientSecret) => {
          setSaving(true)
          setSaveError(null)
          try {
            const resp = await oauthQuickSetup({
              name,
              template_id: templateId,
              client_id: clientId,
              client_secret: clientSecret,
            })
            setDialogOpen(false)
            refetch()
            if (resp.authorize_url) {
              window.location.href = resp.authorize_url
            }
          } catch (err: unknown) {
            setSaveError(err instanceof Error ? err.message : 'Failed to set up OAuth')
          } finally {
            setSaving(false)
          }
        }}
      />
    </div>
  )
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

  const labels: Record<string, string> = {
    valid: 'Connected',
    expired: 'Expired',
    refresh_needed: 'Needs Refresh',
    not_configured: 'Not Connected',
  }

  return (
    <Badge variant="outline" className={`text-xs ${colors[status.status] ?? colors.not_configured}`}>
      {labels[status.status] ?? 'Not Connected'}
    </Badge>
  )
}

function AuthScopeDialog({
  open,
  onClose,
  form,
  setForm,
  onSave,
  saving,
  editing,
  providers,
  templates,
  saveError,
  onQuickSetup,
}: {
  open: boolean
  onClose: () => void
  form: FormData
  setForm: React.Dispatch<React.SetStateAction<FormData>>
  onSave: () => void
  saving: boolean
  editing: boolean
  saveError: string | null
  providers: { id: string; name: string }[]
  templates: OAuthTemplate[]
  onQuickSetup: (name: string, templateId: string, clientId: string, clientSecret: string) => void
}) {
  const [hintInput, setHintInput] = useState('')
  const [oauthMode, setOauthMode] = useState<'template' | 'existing'>('template')
  const [selectedTemplate, setSelectedTemplate] = useState<OAuthTemplate | null>(null)
  const [clientId, setClientId] = useState('')
  const [clientSecret, setClientSecret] = useState('')

  function addHint() {
    if (!hintInput) return
    setForm((f) => ({
      ...f,
      redaction_hints: [...f.redaction_hints, hintInput],
    }))
    setHintInput('')
  }

  function removeHint(index: number) {
    setForm((f) => ({
      ...f,
      redaction_hints: (f.redaction_hints ?? []).filter((_, i) => i !== index),
    }))
  }

  function resetOAuthState() {
    setSelectedTemplate(null)
    setClientId('')
    setClientSecret('')
    setOauthMode('template')
  }

  const isQuickSetup = !editing && form.type === 'oauth2' && oauthMode === 'template' && selectedTemplate

  return (
    <Dialog open={open} onOpenChange={() => { onClose(); resetOAuthState() }}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{editing ? 'Edit Auth Scope' : 'Add Auth Scope'}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label className="text-xs text-muted-foreground">Name</Label>
            <Input
              value={form.name}
              onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
            />
          </div>
          <div className="space-y-2">
            <Label className="text-xs text-muted-foreground">Type</Label>
            <Select
              value={form.type}
              onValueChange={(v) => {
                setForm((f) => ({ ...f, type: v as 'env' | 'header' | 'oauth2' }))
                resetOAuthState()
              }}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="env">Environment Variable</SelectItem>
                <SelectItem value="header">HTTP Header</SelectItem>
                <SelectItem value="oauth2">OAuth 2.0</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {form.type === 'oauth2' && !editing && (
            <div className="space-y-3">
              <div className="flex gap-2">
                <Button
                  type="button"
                  variant={oauthMode === 'template' ? 'default' : 'outline'}
                  size="sm"
                  onClick={() => setOauthMode('template')}
                >
                  Use Template
                </Button>
                <Button
                  type="button"
                  variant={oauthMode === 'existing' ? 'default' : 'outline'}
                  size="sm"
                  onClick={() => { setOauthMode('existing'); setSelectedTemplate(null) }}
                >
                  Use Existing Provider
                </Button>
              </div>

              {oauthMode === 'template' && (
                <>
                  {!selectedTemplate ? (
                    <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
                      {templates.map((tpl) => (
                        <button
                          key={tpl.id}
                          type="button"
                          className="flex flex-col items-center gap-1 rounded-md border border-border p-3 text-sm hover:border-primary hover:bg-muted/50 transition-colors"
                          onClick={() => setSelectedTemplate(tpl)}
                        >
                          <span className="font-medium">{tpl.name}</span>
                        </button>
                      ))}
                    </div>
                  ) : (
                    <div className="space-y-3 rounded-md border border-border p-3">
                      <div className="flex items-center justify-between">
                        <span className="text-sm font-medium">{selectedTemplate.name}</span>
                        <Button
                          type="button"
                          variant="ghost"
                          size="sm"
                          className="h-6 text-xs"
                          onClick={() => setSelectedTemplate(null)}
                        >
                          Change
                        </Button>
                      </div>
                      <p className="text-xs text-muted-foreground">{selectedTemplate.help_text}</p>
                      {selectedTemplate.setup_url && (
                        <a
                          href={selectedTemplate.setup_url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="inline-flex items-center gap-1 text-xs text-primary hover:underline"
                        >
                          <ExternalLink className="h-3 w-3" />
                          Open {selectedTemplate.name} developer settings
                        </a>
                      )}
                      {selectedTemplate.callback_url && (
                        <div className="space-y-1">
                          <Label className="text-xs text-muted-foreground">Callback URL (copy this)</Label>
                          <Input
                            readOnly
                            value={selectedTemplate.callback_url}
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
                      {selectedTemplate.needs_secret && (
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
                      {selectedTemplate.scopes.length > 0 && (
                        <div className="flex flex-wrap gap-1">
                          <span className="text-xs text-muted-foreground mr-1">Scopes:</span>
                          {selectedTemplate.scopes.map((s) => (
                            <Badge key={s} variant="secondary" className="font-mono text-xs">{s}</Badge>
                          ))}
                        </div>
                      )}
                    </div>
                  )}
                </>
              )}

              {oauthMode === 'existing' && (
                <div className="space-y-2">
                  <Label className="text-xs text-muted-foreground">OAuth Provider</Label>
                  <Select
                    value={form.oauth_provider_id}
                    onValueChange={(v) => setForm((f) => ({ ...f, oauth_provider_id: v }))}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder="Select a provider..." />
                    </SelectTrigger>
                    <SelectContent>
                      {providers.map((p) => (
                        <SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              )}
            </div>
          )}

          {form.type === 'oauth2' && editing && (
            <div className="space-y-2">
              <Label className="text-xs text-muted-foreground">OAuth Provider</Label>
              <Select
                value={form.oauth_provider_id}
                onValueChange={(v) => setForm((f) => ({ ...f, oauth_provider_id: v }))}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select a provider..." />
                </SelectTrigger>
                <SelectContent>
                  {providers.map((p) => (
                    <SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}

          <div className="space-y-2">
            <Label className="text-xs text-muted-foreground">Redaction Hints</Label>
            <div className="flex flex-wrap gap-1 mb-2">
              {(form.redaction_hints ?? []).map((hint, i) => (
                <Badge
                  key={i}
                  variant="outline"
                  className="cursor-pointer font-mono text-xs hover:bg-destructive/10 hover:text-destructive"
                  onClick={() => removeHint(i)}
                >
                  {hint} x
                </Badge>
              ))}
            </div>
            <div className="flex gap-2">
              <Input
                className="font-mono text-sm"
                placeholder="e.g. *token*"
                value={hintInput}
                onChange={(e) => setHintInput(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault()
                    addHint()
                  }
                }}
              />
              <Button type="button" variant="outline" size="sm" onClick={addHint}>
                Add
              </Button>
            </div>
          </div>
        </div>
        {saveError && (
          <p className="text-sm text-destructive">{saveError}</p>
        )}
        <DialogFooter>
          <Button variant="outline" onClick={() => { onClose(); resetOAuthState() }}>
            Cancel
          </Button>
          {isQuickSetup ? (
            <Button
              onClick={() => onQuickSetup(form.name, selectedTemplate.id, clientId, clientSecret)}
              disabled={saving || !form.name || !clientId}
            >
              {saving ? 'Setting up...' : 'Save & Authenticate'}
            </Button>
          ) : (
            <Button onClick={onSave} disabled={saving || !form.name}>
              {saving ? 'Saving...' : 'Save'}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
