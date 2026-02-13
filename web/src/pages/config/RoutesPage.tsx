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
  createRoute,
  deleteRoute,
  listAuthScopes,
  listDownstreams,
  listRoutes,
  listWorkspaces,
  updateRoute,
} from '@/api/client'
import type { RouteRule } from '@/api/types'
import { ChevronDown, ChevronRight, GitBranch, Pencil, Plus, ShieldCheck, Trash2, X } from 'lucide-react'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { toast } from 'sonner'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'

interface FormData {
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
}

const emptyForm: FormData = {
  name: '',
  priority: 100,
  workspace_id: '',
  path_glob: '**',
  tool_match: ['*'],
  downstream_server_id: '',
  auth_scope_id: '',
  policy: 'allow',
  log_level: 'info',
  requires_approval: false,
  approval_timeout: 300,
}

export function RoutesPage() {
  const fetcher = useCallback(() => listRoutes(), [])
  const { data, loading, error, refetch } = useApi(fetcher)

  const workspacesFetcher = useCallback(() => listWorkspaces(), [])
  const { data: workspaces } = useApi(workspacesFetcher)

  const downstreamsFetcher = useCallback(() => listDownstreams(), [])
  const { data: downstreams } = useApi(downstreamsFetcher)

  const authScopesFetcher = useCallback(() => listAuthScopes(), [])
  const { data: authScopes } = useApi(authScopesFetcher)

  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<RouteRule | null>(null)
  const [form, setForm] = useState<FormData>(emptyForm)
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<RouteRule | null>(null)

  function openCreate() {
    setEditing(null)
    setForm(emptyForm)
    setSaveError(null)
    setDialogOpen(true)
  }

  function openEdit(r: RouteRule) {
    setEditing(r)
    const tm = Array.isArray(r.tool_match) ? r.tool_match as string[] : []
    setForm({
      name: r.name || '',
      priority: r.priority,
      workspace_id: r.workspace_id,
      path_glob: r.path_glob || '**',
      tool_match: tm,
      downstream_server_id: r.downstream_server_id,
      auth_scope_id: r.auth_scope_id,
      policy: r.policy,
      log_level: r.log_level,
      requires_approval: r.requires_approval ?? false,
      approval_timeout: r.approval_timeout ?? 300,
    })
    setSaveError(null)
    setDialogOpen(true)
  }

  async function handleSave() {
    setSaving(true)
    setSaveError(null)
    try {
      if (editing) {
        await updateRoute(editing.id, form)
      } else {
        await createRoute(form)
      }
      setDialogOpen(false)
      toast.success(editing ? 'Route updated' : 'Route created')
      refetch()
    } catch (err: unknown) {
      setSaveError(err instanceof Error ? err.message : 'Failed to save route rule')
    } finally {
      setSaving(false)
    }
  }

  async function confirmDelete() {
    if (!deleteTarget) return
    try {
      await deleteRoute(deleteTarget.id)
      setDeleteTarget(null)
      toast.success('Route deleted')
      refetch()
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : 'Failed to delete route rule')
    }
  }

  const wsName = (id: string) => workspaces?.find((w) => w.id === id)?.name ?? id
  const dsName = (id: string) => downstreams?.find((d) => d.id === id)?.name ?? id
  const asName = (id: string) => authScopes?.find((a) => a.id === id)?.name ?? id

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Route Rules</h1>
        <Button onClick={openCreate}>
          <Plus className="mr-2 h-4 w-4" />
          Add Route
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
                  <TableHead className="hidden sm:table-cell">Priority</TableHead>
                  <TableHead>Name</TableHead>
                  <TableHead>Workspace</TableHead>
                  <TableHead className="hidden md:table-cell">Path Glob</TableHead>
                  <TableHead className="hidden lg:table-cell">Downstream</TableHead>
                  <TableHead className="hidden lg:table-cell">Auth Scope</TableHead>
                  <TableHead className="hidden lg:table-cell">Approval</TableHead>
                  <TableHead>Policy</TableHead>
                  <TableHead className="w-24">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={9} className="h-32">
                      <div className="flex flex-col items-center justify-center text-muted-foreground">
                        <GitBranch className="mb-2 h-8 w-8 text-muted-foreground/50" />
                        <p className="text-sm">No route rules configured</p>
                        <button onClick={openCreate} className="text-xs text-primary hover:underline">
                          Add routing rules to control tool call dispatch
                        </button>
                      </div>
                    </TableCell>
                  </TableRow>
                ) : (
                  data.map((r) => (
                    <TableRow key={r.id} className="border-border/30 hover:bg-muted/30">
                      <TableCell className="hidden sm:table-cell font-mono text-sm text-muted-foreground">
                        {r.priority}
                      </TableCell>
                      <TableCell className="text-sm">
                        {r.name || <span className="text-muted-foreground/40">—</span>}
                      </TableCell>
                      <TableCell>{wsName(r.workspace_id)}</TableCell>
                      <TableCell className="hidden md:table-cell">
                        <div className="max-w-[10rem] truncate font-mono text-xs text-accent-foreground">
                          {r.path_glob}
                        </div>
                      </TableCell>
                      <TableCell className="hidden lg:table-cell">
                        <div className="max-w-[10rem] truncate">{dsName(r.downstream_server_id)}</div>
                      </TableCell>
                      <TableCell className="hidden lg:table-cell text-muted-foreground">
                        <div className="max-w-[10rem] truncate">
                          {r.auth_scope_id ? asName(r.auth_scope_id) : '-'}
                        </div>
                      </TableCell>
                      <TableCell className="hidden lg:table-cell">
                        {r.requires_approval ? (
                          <Badge variant="outline" className="gap-1 text-amber-400 border-amber-500/30">
                            <ShieldCheck className="h-3 w-3" />
                            required
                          </Badge>
                        ) : (
                          <span className="text-muted-foreground/40">-</span>
                        )}
                      </TableCell>
                      <TableCell>
                        <Badge variant={r.policy === 'allow' ? 'secondary' : 'destructive'}>
                          {r.policy}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <div className="flex gap-1">
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Button
                                variant="ghost"
                                size="sm"
                                className="h-8 w-8 p-0"
                                onClick={() => openEdit(r)}
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
                                className="h-8 w-8 p-0 hover:bg-destructive/10 hover:text-destructive"
                                onClick={() => setDeleteTarget(r)}
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

      <RouteDialog
        open={dialogOpen}
        onClose={() => setDialogOpen(false)}
        form={form}
        setForm={setForm}
        onSave={handleSave}
        saving={saving}
        editing={!!editing}
        workspaces={workspaces ?? []}
        downstreams={downstreams ?? []}
        authScopes={authScopes ?? []}
        saveError={saveError}
      />

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title="Delete route rule"
        description={`Are you sure you want to delete this route rule?`}
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={confirmDelete}
      />
    </div>
  )
}

function RouteDialog({
  open,
  onClose,
  form,
  setForm,
  onSave,
  saving,
  editing,
  workspaces,
  downstreams,
  authScopes,
  saveError,
}: {
  open: boolean
  onClose: () => void
  form: FormData
  setForm: React.Dispatch<React.SetStateAction<FormData>>
  onSave: () => void
  saving: boolean
  editing: boolean
  saveError: string | null
  workspaces: { id: string; name: string }[]
  downstreams: { id: string; name: string }[]
  authScopes: { id: string; name: string }[]
}) {
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [chipInput, setChipInput] = useState('')
  const [prevForm, setPrevForm] = useState(form)

  // Sync state when form changes (e.g. opening edit dialog).
  if (form !== prevForm) {
    setPrevForm(form)
    setChipInput('')
    // Show advanced section when editing a rule with non-default values.
    if (form.path_glob !== '**' || form.tool_match.length > 0) {
      setShowAdvanced(true)
    }
  }

  const hasNonDefaultAdvanced = form.path_glob !== '**' || form.tool_match.length > 0

  function addChip() {
    const val = chipInput.trim()
    if (!val) return
    setForm((f) => ({ ...f, tool_match: [...f.tool_match, val] }))
    setChipInput('')
  }

  function removeChip(index: number) {
    setForm((f) => ({ ...f, tool_match: f.tool_match.filter((_, i) => i !== index) }))
  }

  return (
    <Dialog open={open} onOpenChange={() => onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{editing ? 'Edit Route' : 'Add Route'}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label className="text-xs text-muted-foreground">Name (optional)</Label>
            <Input
              value={form.name}
              onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
              placeholder="e.g. GitHub allow-all"
            />
          </div>

          <div className="space-y-2">
            <Label className="text-xs text-muted-foreground">Workspace</Label>
            <Select
              value={form.workspace_id}
              onValueChange={(v) => setForm((f) => ({ ...f, workspace_id: v }))}
            >
              <SelectTrigger>
                <SelectValue placeholder="Select workspace..." />
              </SelectTrigger>
              <SelectContent>
                {workspaces.map((w) => (
                  <SelectItem key={w.id} value={w.id}>
                    {w.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label className={`text-xs text-muted-foreground ${form.policy === 'deny' ? 'opacity-50' : ''}`}>
              Downstream Server
            </Label>
            <Select
              value={form.downstream_server_id}
              onValueChange={(v) =>
                setForm((f) => ({ ...f, downstream_server_id: v }))
              }
              disabled={form.policy === 'deny'}
            >
              <SelectTrigger>
                <SelectValue placeholder={form.policy === 'deny' ? 'N/A for deny rules' : 'Select server...'} />
              </SelectTrigger>
              <SelectContent>
                {downstreams.map((d) => (
                  <SelectItem key={d.id} value={d.id}>
                    {d.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label className={`text-xs text-muted-foreground ${form.policy === 'deny' ? 'opacity-50' : ''}`}>
              Auth Scope
            </Label>
            <Select
              value={form.auth_scope_id || 'none'}
              onValueChange={(v) =>
                setForm((f) => ({ ...f, auth_scope_id: v === 'none' ? '' : v }))
              }
              disabled={form.policy === 'deny'}
            >
              <SelectTrigger>
                <SelectValue placeholder={form.policy === 'deny' ? 'N/A for deny rules' : 'None'} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="none">None</SelectItem>
                {authScopes.map((a) => (
                  <SelectItem key={a.id} value={a.id}>
                    {a.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label className="text-xs text-muted-foreground">Policy</Label>
              <Select
                value={form.policy}
                onValueChange={(v) => {
                  const policy = v as 'allow' | 'deny'
                  if (policy === 'deny') {
                    setForm((f) => ({
                      ...f,
                      policy,
                      downstream_server_id: '',
                      auth_scope_id: '',
                    }))
                  } else {
                    setForm((f) => ({ ...f, policy }))
                  }
                }}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="allow">Allow</SelectItem>
                  <SelectItem value="deny">Deny</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label className="text-xs text-muted-foreground">Priority</Label>
              <Input
                type="number"
                value={form.priority}
                onChange={(e) =>
                  setForm((f) => ({ ...f, priority: Number(e.target.value) }))
                }
              />
            </div>
          </div>

          {form.policy === 'allow' && (
            <div className="space-y-3 rounded-md border border-border/50 p-3">
              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  checked={form.requires_approval}
                  onChange={(e) =>
                    setForm((f) => ({ ...f, requires_approval: e.target.checked }))
                  }
                  className="h-4 w-4 rounded border-border accent-primary"
                />
                <span className="text-sm">Requires approval</span>
              </label>
              {form.requires_approval && (
                <div className="space-y-2 pl-6">
                  <Label className="text-xs text-muted-foreground">Timeout (seconds)</Label>
                  <Input
                    type="number"
                    min={10}
                    value={form.approval_timeout}
                    onChange={(e) =>
                      setForm((f) => ({ ...f, approval_timeout: Number(e.target.value) }))
                    }
                    className="w-32"
                  />
                  <p className="text-xs text-muted-foreground/60">
                    How long to wait for approval before auto-denying.
                  </p>
                </div>
              )}
            </div>
          )}

          <button
            type="button"
            className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors"
            onClick={() => setShowAdvanced(!showAdvanced)}
          >
            {showAdvanced ? (
              <ChevronDown className="h-3.5 w-3.5" />
            ) : (
              <ChevronRight className="h-3.5 w-3.5" />
            )}
            Advanced
            {!showAdvanced && !hasNonDefaultAdvanced && (
              <span className="text-muted-foreground/60">
                — all paths, all tools
              </span>
            )}
          </button>

          {showAdvanced && (
            <div className="space-y-4 rounded-md border border-border/50 p-3">
              <div className="space-y-2">
                <Label className="text-xs text-muted-foreground">Path Glob</Label>
                <Input
                  className="font-mono text-sm"
                  value={form.path_glob}
                  onChange={(e) => setForm((f) => ({ ...f, path_glob: e.target.value }))}
                  placeholder="**"
                />
                <p className="text-xs text-muted-foreground/60">
                  Matches workspace subpath. Default <code className="font-mono">**</code> matches everything.
                </p>
              </div>

              <div className="space-y-2">
                <Label className="text-xs text-muted-foreground">Tool Match</Label>
                <div className="flex flex-wrap gap-1 mb-2">
                  {form.tool_match.map((pattern, i) => (
                    <Badge
                      key={i}
                      variant="outline"
                      className="gap-1 font-mono text-xs"
                    >
                      {pattern}
                      <button
                        type="button"
                        className="ml-0.5 hover:text-destructive"
                        onClick={() => removeChip(i)}
                      >
                        <X className="h-3 w-3" />
                      </button>
                    </Badge>
                  ))}
                </div>
                <Input
                  value={chipInput}
                  onChange={(e) => setChipInput(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      e.preventDefault()
                      addChip()
                    }
                  }}
                  className="font-mono text-sm"
                  placeholder="github__* (press Enter to add)"
                />
                <p className="text-xs text-muted-foreground/60">
                  Tool glob patterns. Leave empty to match all tools.
                </p>
              </div>
            </div>
          )}
        </div>
        {saveError && (
          <p className="text-sm text-destructive">{saveError}</p>
        )}
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={onSave} disabled={saving || !form.workspace_id}>
            {saving ? 'Saving...' : 'Save'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
