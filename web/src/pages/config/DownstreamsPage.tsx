import { useCallback, useEffect, useMemo, useState } from 'react'
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
  createDownstream,
  deleteDownstream,
  getDownstreamOAuthStatus,
  listDownstreams,
  updateDownstream,
} from '@/api/client'
import type { DownstreamOAuthStatusEntry, DownstreamServer } from '@/api/types'
import { ConnectDialog } from './ConnectDialog'
import { DownstreamDialog, emptyDownstreamForm } from './DownstreamDialog'
import type { DownstreamFormData } from './DownstreamDialog'
import { Copy, Link, Pause, Pencil, Play, Plus, Server, Trash2 } from 'lucide-react'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { toast } from 'sonner'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { getOAuthBadges } from './DownstreamOAuthBadges'

export function DownstreamsPage() {
  const fetcher = useCallback(() => listDownstreams(), [])
  const { data, loading, error, refetch } = useApi(fetcher)

  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<DownstreamServer | null>(null)
  const [form, setForm] = useState<DownstreamFormData>(emptyDownstreamForm)
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [connectDialogOpen, setConnectDialogOpen] = useState(false)
  const [connectServer, setConnectServer] = useState<DownstreamServer | null>(null)
  const [oauthStatuses, setOauthStatuses] = useState<Record<string, DownstreamOAuthStatusEntry[]>>({})
  const [statusErrors, setStatusErrors] = useState<Record<string, boolean>>({})
  const [deleteTarget, setDeleteTarget] = useState<DownstreamServer | null>(null)

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

  function openCreate() {
    setEditing(null)
    setForm(emptyDownstreamForm)
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
      cache_config: ds.cache_config ? { ...ds.cache_config } : undefined,
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
      cache_config: ds.cache_config ? { ...ds.cache_config } : undefined,
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
                        {getOAuthBadges(ds, oauthStatuses, statusErrors, openConnect)}
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
