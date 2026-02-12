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
import { ChevronDown, ChevronRight, GitBranch, Pencil, Plus, Trash2 } from 'lucide-react'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

interface FormData {
  priority: number
  workspace_id: string
  path_glob: string
  tool_match: string[]
  downstream_server_id: string
  auth_scope_id: string
  policy: 'allow' | 'deny'
  log_level: string
}

const emptyForm: FormData = {
  priority: 100,
  workspace_id: '',
  path_glob: '**',
  tool_match: ['*'],
  downstream_server_id: '',
  auth_scope_id: '',
  policy: 'allow',
  log_level: 'info',
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
      priority: r.priority,
      workspace_id: r.workspace_id,
      path_glob: r.path_glob || '**',
      tool_match: tm,
      downstream_server_id: r.downstream_server_id,
      auth_scope_id: r.auth_scope_id,
      policy: r.policy,
      log_level: r.log_level,
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
      refetch()
    } catch (err: unknown) {
      setSaveError(err instanceof Error ? err.message : 'Failed to save route rule')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(id: string) {
    try {
      await deleteRoute(id)
      refetch()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to delete route rule'
      alert(msg)
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
                  <TableHead>Workspace</TableHead>
                  <TableHead className="hidden md:table-cell">Path Glob</TableHead>
                  <TableHead className="hidden lg:table-cell">Downstream</TableHead>
                  <TableHead className="hidden lg:table-cell">Auth Scope</TableHead>
                  <TableHead>Policy</TableHead>
                  <TableHead className="w-24">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={7} className="h-32">
                      <div className="flex flex-col items-center justify-center text-muted-foreground">
                        <GitBranch className="mb-2 h-8 w-8 text-muted-foreground/50" />
                        <p className="text-sm">No route rules configured</p>
                        <p className="text-xs text-muted-foreground/60">
                          Add routing rules to control tool call dispatch
                        </p>
                      </div>
                    </TableCell>
                  </TableRow>
                ) : (
                  data.map((r) => (
                    <TableRow key={r.id} className="border-border/30 hover:bg-muted/30">
                      <TableCell className="hidden sm:table-cell font-mono text-sm text-muted-foreground">
                        {r.priority}
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
                                onClick={() => handleDelete(r.id)}
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
  const [toolMatchStr, setToolMatchStr] = useState('')
  const [prevForm, setPrevForm] = useState(form)

  // Sync toolMatchStr when form changes (e.g. opening edit dialog).
  if (form !== prevForm) {
    setPrevForm(form)
    const val = form.tool_match.length > 0 ? JSON.stringify(form.tool_match) : ''
    setToolMatchStr(val)
    // Show advanced section when editing a rule with non-default values.
    if (form.path_glob !== '**' || form.tool_match.length > 0) {
      setShowAdvanced(true)
    }
  }

  const hasNonDefaultAdvanced = form.path_glob !== '**' || form.tool_match.length > 0

  return (
    <Dialog open={open} onOpenChange={() => onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{editing ? 'Edit Route' : 'Add Route'}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
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
            <Label className="text-xs text-muted-foreground">Downstream Server</Label>
            <Select
              value={form.downstream_server_id}
              onValueChange={(v) =>
                setForm((f) => ({ ...f, downstream_server_id: v }))
              }
            >
              <SelectTrigger>
                <SelectValue placeholder="Select server..." />
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
            <Label className="text-xs text-muted-foreground">Auth Scope</Label>
            <Select
              value={form.auth_scope_id || 'none'}
              onValueChange={(v) =>
                setForm((f) => ({ ...f, auth_scope_id: v === 'none' ? '' : v }))
              }
            >
              <SelectTrigger>
                <SelectValue placeholder="None" />
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
                onValueChange={(v) =>
                  setForm((f) => ({ ...f, policy: v as 'allow' | 'deny' }))
                }
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
                <Input
                  value={toolMatchStr}
                  onChange={(e) => {
                    setToolMatchStr(e.target.value)
                    if (e.target.value.trim() === '') {
                      setForm((f) => ({ ...f, tool_match: [] }))
                      return
                    }
                    try {
                      const parsed: unknown = JSON.parse(e.target.value)
                      if (
                        Array.isArray(parsed) &&
                        parsed.every((v) => typeof v === 'string')
                      ) {
                        setForm((f) => ({ ...f, tool_match: parsed as string[] }))
                      }
                    } catch {
                      // Wait for valid JSON
                    }
                  }}
                  className="font-mono text-sm"
                  placeholder='["github__*", "slack__*"]'
                />
                <p className="text-xs text-muted-foreground/60">
                  JSON array of tool globs. Leave empty to match all tools.
                  Example: <code className="font-mono">["github__*", "slack__post_message"]</code>
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
