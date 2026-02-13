import { useCallback, useEffect, useMemo, useState } from 'react'
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
  createDownstream,
  deleteDownstream,
  getDownstreamOAuthStatus,
  listDownstreams,
  updateDownstream,
} from '@/api/client'
import type { DownstreamOAuthStatusEntry, DownstreamServer } from '@/api/types'
import { ConnectDialog } from './ConnectDialog'
import { AlertCircle, Clock, Copy, Link, Pause, Pencil, Play, Plus, Server, Trash2 } from 'lucide-react'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { toast } from 'sonner'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'

interface FormData {
  name: string
  transport: 'stdio' | 'http'
  command: string
  args: string[]
  url: string | null
  tool_namespace: string
  idle_timeout_sec: number
  max_instances: number
  restart_policy: string
  disabled: boolean
}

const emptyForm: FormData = {
  name: '',
  transport: 'stdio',
  command: '',
  args: [],
  url: null,
  tool_namespace: '',
  idle_timeout_sec: 300,
  max_instances: 1,
  restart_policy: 'on-failure',
  disabled: false,
}

export function DownstreamsPage() {
  const fetcher = useCallback(() => listDownstreams(), [])
  const { data, loading, error, refetch } = useApi(fetcher)

  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<DownstreamServer | null>(null)
  const [form, setForm] = useState<FormData>(emptyForm)
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [connectDialogOpen, setConnectDialogOpen] = useState(false)
  const [connectServer, setConnectServer] = useState<DownstreamServer | null>(null)
  const [oauthStatuses, setOauthStatuses] = useState<Record<string, DownstreamOAuthStatusEntry[]>>({})
  const [statusErrors, setStatusErrors] = useState<Record<string, boolean>>({})
  const [deleteTarget, setDeleteTarget] = useState<DownstreamServer | null>(null)

  // Fetch OAuth status for all HTTP downstreams.
  useEffect(() => {
    if (!data) return
    let active = true
    for (const ds of data) {
      if (ds.transport === 'http') {
        getDownstreamOAuthStatus(ds.id)
          .then((res) => {
            if (!active) return
            setOauthStatuses((prev) => ({ ...prev, [ds.id]: res.entries }))
          })
          .catch(() => {
            if (!active) return
            setStatusErrors((prev) => ({ ...prev, [ds.id]: true }))
          })
      }
    }
    return () => { active = false }
  }, [data])

  function formatRelativeTime(isoDate: string): string {
    const diff = new Date(isoDate).getTime() - Date.now()
    if (diff < 0) return 'expired'
    const days = Math.floor(diff / (1000 * 60 * 60 * 24))
    if (days > 1) return `${days}d`
    const hours = Math.floor(diff / (1000 * 60 * 60))
    if (hours > 0) return `${hours}h`
    return 'soon'
  }

  function getOAuthBadges(ds: DownstreamServer) {
    if (ds.transport !== 'http') return null
    if (statusErrors[ds.id]) {
      return (
        <Badge variant="outline" className="text-xs text-destructive border-destructive/30">
          <AlertCircle className="mr-1 h-3 w-3" /> Error
        </Badge>
      )
    }
    const entries = oauthStatuses[ds.id]
    if (!entries || entries.length === 0) {
      return (
        <Badge variant="outline" className="text-xs text-muted-foreground cursor-pointer hover:text-primary hover:border-primary"
          onClick={(e) => { e.stopPropagation(); openConnect(ds) }}>
          Not Connected
        </Badge>
      )
    }
    return (
      <div className="flex flex-col gap-1">
        {entries.map((entry) => {
          const expiring = entry.expires_at && (new Date(entry.expires_at).getTime() - Date.now()) < 7 * 24 * 60 * 60 * 1000
          if (entry.status === 'authenticated') {
            return (
              <Badge key={entry.auth_scope_id} className={`text-xs border-0 ${expiring ? 'bg-amber-500/15 text-amber-600' : 'bg-emerald-500/15 text-emerald-600'}`}>
                {entry.auth_scope_name}
                {entry.expires_at && (
                  <span className="ml-1 opacity-70">
                    <Clock className="mr-0.5 inline h-2.5 w-2.5" />
                    {formatRelativeTime(entry.expires_at)}
                  </span>
                )}
              </Badge>
            )
          }
          if (entry.status === 'expired') {
            return (
              <Badge key={entry.auth_scope_id} variant="outline" className="text-xs text-amber-600 border-amber-300">
                {entry.auth_scope_name} — Expired
              </Badge>
            )
          }
          return (
            <Badge key={entry.auth_scope_id} variant="outline" className="text-xs text-muted-foreground">
              {entry.auth_scope_name}
            </Badge>
          )
        })}
      </div>
    )
  }

  function openCreate() {
    setEditing(null)
    setForm(emptyForm)
    setSaveError(null)
    setDialogOpen(true)
  }

  function handleDuplicate(ds: DownstreamServer) {
    setEditing(null)
    setForm({
      name: `${ds.name} (copy)`,
      transport: ds.transport,
      command: ds.command,
      args: [...(ds.args || [])],
      url: ds.url,
      tool_namespace: `${ds.tool_namespace}_copy`,
      idle_timeout_sec: ds.idle_timeout_sec,
      max_instances: ds.max_instances,
      restart_policy: ds.restart_policy,
      disabled: false,
    })
    setSaveError(null)
    setDialogOpen(true)
  }

  function openEdit(ds: DownstreamServer) {
    setEditing(ds)
    setForm({
      name: ds.name,
      transport: ds.transport,
      command: ds.command,
      args: [...(ds.args || [])],
      url: ds.url,
      tool_namespace: ds.tool_namespace,
      idle_timeout_sec: ds.idle_timeout_sec,
      max_instances: ds.max_instances,
      restart_policy: ds.restart_policy,
      disabled: ds.disabled,
    })
    setSaveError(null)
    setDialogOpen(true)
  }

  async function handleSave() {
    setSaving(true)
    setSaveError(null)
    try {
      if (editing) {
        await updateDownstream(editing.id, form)
      } else {
        await createDownstream(form)
      }
      setDialogOpen(false)
      toast.success(editing ? 'Server updated' : 'Server created')
      refetch()
    } catch (err: unknown) {
      setSaveError(err instanceof Error ? err.message : 'Failed to save downstream server')
    } finally {
      setSaving(false)
    }
  }

  async function handleToggleDisabled(ds: DownstreamServer) {
    try {
      await updateDownstream(ds.id, { disabled: !ds.disabled })
      toast.success(ds.disabled ? 'Server enabled' : 'Server disabled')
      refetch()
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : 'Failed to toggle server')
    }
  }

  function openConnect(ds: DownstreamServer) {
    setConnectServer(ds)
    setConnectDialogOpen(true)
  }

  async function confirmDelete() {
    if (!deleteTarget) return
    try {
      await deleteDownstream(deleteTarget.id)
      setDeleteTarget(null)
      toast.success('Server deleted')
      refetch()
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : 'Failed to delete downstream server')
    }
  }

  const sortedData = useMemo(() => {
    if (!data) return null
    return [...data].sort((a, b) => {
      if (a.disabled !== b.disabled) return a.disabled ? 1 : -1
      return a.name.localeCompare(b.name)
    })
  }, [data])

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Downstream Servers</h1>
        <Button onClick={openCreate}>
          <Plus className="mr-2 h-4 w-4" />
          Add Server
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
          {sortedData && (
            <Table>
              <TableHeader>
                <TableRow className="border-border/50 hover:bg-transparent">
                  <TableHead>Name</TableHead>
                  <TableHead className="hidden sm:table-cell">Transport</TableHead>
                  <TableHead className="hidden lg:table-cell">Command / URL</TableHead>
                  <TableHead className="hidden md:table-cell">Namespace</TableHead>
                  <TableHead className="hidden xl:table-cell">Max Instances</TableHead>
                  <TableHead className="hidden sm:table-cell">Auth Status</TableHead>
                  <TableHead>Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {sortedData.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={7} className="h-32">
                      <div className="flex flex-col items-center justify-center text-muted-foreground">
                        <Server className="mb-2 h-8 w-8 text-muted-foreground/50" />
                        <p className="text-sm">No downstream servers configured</p>
                        <button onClick={openCreate} className="text-xs text-primary hover:underline">
                          Add a server to start routing tool calls
                        </button>
                      </div>
                    </TableCell>
                  </TableRow>
                ) : (
                  sortedData.map((ds) => (
                    <TableRow key={ds.id} className={`border-border/30 hover:bg-muted/30 ${ds.disabled ? 'opacity-50' : ''}`}>
                      <TableCell className="font-medium">{ds.name}</TableCell>
                      <TableCell className="hidden sm:table-cell">
                        <Badge variant="outline" className="font-mono text-xs">
                          {ds.transport}
                        </Badge>
                      </TableCell>
                      <TableCell className="hidden lg:table-cell">
                        <div className="max-w-[14rem] truncate font-mono text-xs text-accent-foreground">
                          {ds.transport === 'stdio'
                            ? `${ds.command} ${(ds.args ?? []).join(' ')}`
                            : ds.url}
                        </div>
                      </TableCell>
                      <TableCell className="hidden md:table-cell">
                        <div className="max-w-[8rem] truncate font-mono text-xs text-accent-foreground">
                          {ds.tool_namespace}
                        </div>
                      </TableCell>
                      <TableCell className="hidden xl:table-cell font-mono text-sm text-muted-foreground">
                        {ds.max_instances}
                      </TableCell>
                      <TableCell className="hidden sm:table-cell">
                        {getOAuthBadges(ds)}
                      </TableCell>
                      <TableCell>
                        <div className="flex gap-0.5">
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Button
                                variant="ghost"
                                size="sm"
                                className={`h-7 w-7 p-0 ${ds.disabled ? 'text-emerald-600 hover:bg-emerald-500/10' : 'text-amber-600 hover:bg-amber-500/10'}`}
                                onClick={() => handleToggleDisabled(ds)}
                              >
                                {ds.disabled ? <Play className="h-3.5 w-3.5" /> : <Pause className="h-3.5 w-3.5" />}
                              </Button>
                            </TooltipTrigger>
                            <TooltipContent>{ds.disabled ? 'Enable' : 'Disable'}</TooltipContent>
                          </Tooltip>
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Button
                                variant="ghost"
                                size="sm"
                                className="h-7 w-7 p-0"
                                onClick={() => handleDuplicate(ds)}
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
                                onClick={() => openEdit(ds)}
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
                                onClick={() => setDeleteTarget(ds)}
                              >
                                <Trash2 className="h-3.5 w-3.5" />
                              </Button>
                            </TooltipTrigger>
                            <TooltipContent>Delete</TooltipContent>
                          </Tooltip>
                          {ds.transport === 'http' && (
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  className="h-7 w-7 p-0 text-primary hover:bg-primary/10"
                                  onClick={() => openConnect(ds)}
                                >
                                  <Link className="h-3.5 w-3.5" />
                                </Button>
                              </TooltipTrigger>
                              <TooltipContent>Connect</TooltipContent>
                            </Tooltip>
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

      <DownstreamDialog
        open={dialogOpen}
        onClose={() => setDialogOpen(false)}
        form={form}
        setForm={setForm}
        onSave={handleSave}
        saving={saving}
        editing={!!editing}
        saveError={saveError}
      />

      <ConnectDialog
        open={connectDialogOpen}
        onClose={() => setConnectDialogOpen(false)}
        server={connectServer}
        onConnected={refetch}
      />

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title="Delete downstream server"
        description={`Are you sure you want to delete "${deleteTarget?.name}"?`}
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={confirmDelete}
      />
    </div>
  )
}

function DownstreamDialog({
  open,
  onClose,
  form,
  setForm,
  onSave,
  saving,
  editing,
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
}) {
  return (
    <Dialog open={open} onOpenChange={() => onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{editing ? 'Edit Server' : 'Add Server'}</DialogTitle>
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
            <Label className="text-xs text-muted-foreground">Transport</Label>
            <Select
              value={form.transport}
              onValueChange={(v) =>
                setForm((f) => ({ ...f, transport: v as 'stdio' | 'http' }))
              }
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="stdio">stdio</SelectItem>
                <SelectItem value="http">http</SelectItem>
              </SelectContent>
            </Select>
          </div>
          {form.transport === 'stdio' ? (
            <>
              <div className="space-y-2">
                <Label className="text-xs text-muted-foreground">Command</Label>
                <Input
                  className="font-mono text-sm"
                  value={form.command}
                  onChange={(e) => setForm((f) => ({ ...f, command: e.target.value }))}
                  placeholder="npx"
                />
              </div>
              <div className="space-y-2">
                <Label className="text-xs text-muted-foreground">
                  Args (comma-separated)
                </Label>
                <Input
                  className="font-mono text-sm"
                  value={(form.args ?? []).join(', ')}
                  onChange={(e) =>
                    setForm((f) => ({
                      ...f,
                      args: e.target.value
                        .split(',')
                        .map((s) => s.trim())
                        .filter(Boolean),
                    }))
                  }
                  placeholder="-y, @modelcontextprotocol/server-github"
                />
              </div>
              {form.command && (
                <div className="rounded-md bg-muted/50 border border-border/50 px-3 py-2 font-mono text-xs text-muted-foreground">
                  <span className="text-primary">$</span> {form.command} {(form.args ?? []).join(' ')}
                </div>
              )}
            </>
          ) : (
            <div className="space-y-2">
              <Label className="text-xs text-muted-foreground">URL</Label>
              <Input
                className="font-mono text-sm"
                value={form.url ?? ''}
                onChange={(e) => setForm((f) => ({ ...f, url: e.target.value || null }))}
                placeholder="http://localhost:3000"
              />
            </div>
          )}
          <div className="space-y-2">
            <Label className="text-xs text-muted-foreground">Tool Namespace</Label>
            <Input
              className="font-mono text-sm"
              value={form.tool_namespace}
              onChange={(e) => setForm((f) => ({ ...f, tool_namespace: e.target.value }))}
              placeholder="github"
            />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label className="text-xs text-muted-foreground">Idle Timeout (sec)</Label>
              <Input
                type="number"
                value={form.idle_timeout_sec}
                onChange={(e) =>
                  setForm((f) => ({ ...f, idle_timeout_sec: Number(e.target.value) }))
                }
              />
            </div>
            <div className="space-y-2">
              <Label className="text-xs text-muted-foreground">Max Instances</Label>
              <Input
                type="number"
                value={form.max_instances}
                onChange={(e) =>
                  setForm((f) => ({ ...f, max_instances: Number(e.target.value) }))
                }
              />
            </div>
          </div>
        </div>
        {saveError && (
          <p className="text-sm text-destructive">{saveError}</p>
        )}
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={onSave} disabled={saving || !form.name}>
            {saving ? 'Saving...' : 'Save'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
