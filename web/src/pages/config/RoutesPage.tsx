import { useCallback, useMemo, useState } from 'react'
import { Button } from '@/components/ui/button'
import { useApi } from '@/hooks/use-api'
import {
  deleteRoute,
  listAuthScopes,
  listDownstreams,
  listRoutes,
  listWorkspaces,
  createRoute,
  updateRoute,
} from '@/api/client'
import type { RouteRule, Workspace } from '@/api/types'
import { Plus } from 'lucide-react'
import { toast } from 'sonner'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { RouteDialog, emptyForm } from './RouteDialog'
import type { RouteFormData } from './RouteDialog'
import { BulkEnableDialog } from './BulkEnableDialog'
import { RouteWorkspaceGroup } from './RouteWorkspaceGroup'

interface WorkspaceGroup {
  workspace: Workspace
  rules: RouteRule[]
  enabledDownstreamIds: Set<string>
}

export function RoutesPage() {
  const fetcher = useCallback(() => listRoutes(), [])
  const { data: routes, loading, error, refetch } = useApi(fetcher)

  const workspacesFetcher = useCallback(() => listWorkspaces(), [])
  const { data: workspaces } = useApi(workspacesFetcher)

  const downstreamsFetcher = useCallback(() => listDownstreams(), [])
  const { data: downstreams } = useApi(downstreamsFetcher)

  const authScopesFetcher = useCallback(() => listAuthScopes(), [])
  const { data: authScopes } = useApi(authScopesFetcher)

  // Dialog state
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<RouteRule | null>(null)
  const [form, setForm] = useState<RouteFormData>(emptyForm)
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<RouteRule | null>(null)

  // Bulk enable state
  const [bulkWorkspace, setBulkWorkspace] = useState<Workspace | null>(null)

  // Expanded workspaces
  const [expandedWs, setExpandedWs] = useState<Set<string>>(new Set())

  // Group routes by workspace
  const groups = useMemo((): WorkspaceGroup[] => {
    if (!workspaces || !routes) return []

    const rulesByWs = new Map<string, RouteRule[]>()
    for (const r of routes) {
      const existing = rulesByWs.get(r.workspace_id) ?? []
      existing.push(r)
      rulesByWs.set(r.workspace_id, existing)
    }

    const result: WorkspaceGroup[] = workspaces.map((ws) => {
      const wsRules = rulesByWs.get(ws.id) ?? []
      const enabledIds = new Set(wsRules.map((r) => r.downstream_server_id).filter(Boolean))
      return { workspace: ws, rules: wsRules, enabledDownstreamIds: enabledIds }
    })

    // Sort: workspaces with rules first, then alphabetical
    result.sort((a, b) => {
      if (a.rules.length > 0 && b.rules.length === 0) return -1
      if (a.rules.length === 0 && b.rules.length > 0) return 1
      return a.workspace.name.localeCompare(b.workspace.name)
    })

    return result
  }, [workspaces, routes])

  function toggleExpand(wsId: string) {
    setExpandedWs((prev) => {
      const next = new Set(prev)
      if (next.has(wsId)) next.delete(wsId)
      else next.add(wsId)
      return next
    })
  }

  function openCreate(prefillWorkspaceId?: string) {
    setEditing(null)
    setForm({ ...emptyForm, workspace_id: prefillWorkspaceId ?? '' })
    setSaveError(null)
    setDialogOpen(true)
  }

  function openEdit(r: RouteRule) {
    setEditing(r)
    const tm = Array.isArray(r.tool_match) ? (r.tool_match as string[]) : []
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

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Route Rules</h1>
        <Button onClick={() => openCreate()}>
          <Plus className="mr-2 h-4 w-4" />
          Add Route
        </Button>
      </div>

      {loading && !routes && (
        <div className="flex items-center gap-2 text-muted-foreground">
          <div className="h-2 w-2 animate-pulse rounded-full bg-primary" />
          Loading...
        </div>
      )}
      {error && <p className="text-destructive">Error: {error}</p>}

      {groups.length > 0 && (
        <div className="space-y-3">
          {groups.map((g) => (
            <RouteWorkspaceGroup
              key={g.workspace.id}
              workspace={g.workspace}
              rules={g.rules}
              expanded={expandedWs.has(g.workspace.id)}
              onToggle={() => toggleExpand(g.workspace.id)}
              onEnableServers={() => setBulkWorkspace(g.workspace)}
              onAddRule={() => openCreate(g.workspace.id)}
              onEditRule={openEdit}
              onDeleteRule={setDeleteTarget}
              downstreams={downstreams ?? []}
              authScopes={authScopes ?? []}
            />
          ))}
        </div>
      )}

      {workspaces && routes && groups.length === 0 && (
        <div className="text-center py-12 text-muted-foreground">
          <p className="text-sm">No workspaces configured yet.</p>
          <p className="text-xs mt-1">Create a workspace first, then add route rules.</p>
        </div>
      )}

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

      {bulkWorkspace && (
        <BulkEnableDialog
          open={!!bulkWorkspace}
          onClose={() => setBulkWorkspace(null)}
          workspace={bulkWorkspace}
          downstreams={downstreams ?? []}
          enabledDownstreamIds={
            groups.find((g) => g.workspace.id === bulkWorkspace.id)?.enabledDownstreamIds ?? new Set()
          }
          onSuccess={refetch}
        />
      )}

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title="Delete route rule"
        description="Are you sure you want to delete this route rule?"
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={confirmDelete}
      />
    </div>
  )
}
