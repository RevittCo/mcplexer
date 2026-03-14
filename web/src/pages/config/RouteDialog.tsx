import { useEffect, useState } from 'react'
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
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Badge } from '@/components/ui/badge'
import { ChevronDown, ChevronRight, X } from 'lucide-react'

export interface RouteFormData {
  name: string
  priority: number
  workspace_id: string
  path_glob: string
  tool_match: string[]
  downstream_server_id: string
  auth_scope_id: string
  policy: 'allow' | 'deny'
  log_level: string
  approval_mode: 'none' | 'write' | 'all'
  approval_timeout: number
}

export const emptyForm: RouteFormData = {
  name: '',
  priority: 100,
  workspace_id: '',
  path_glob: '**',
  tool_match: [],
  downstream_server_id: '',
  auth_scope_id: '',
  policy: 'allow',
  log_level: 'info',
  approval_mode: 'none',
  approval_timeout: 300,
}

interface RouteDialogProps {
  open: boolean
  onClose: () => void
  form: RouteFormData
  setForm: React.Dispatch<React.SetStateAction<RouteFormData>>
  onSave: () => void
  saving: boolean
  editing: boolean
  saveError: string | null
  workspaces: { id: string; name: string }[]
  downstreams: { id: string; name: string }[]
  authScopes: { id: string; name: string }[]
}

export function RouteDialog({
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
}: RouteDialogProps) {
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [chipInput, setChipInput] = useState('')

  const hasNonDefaultAdvanced = form.path_glob !== '**' || form.tool_match.length > 0

  useEffect(() => {
    if (!open) return
    setChipInput('')
    setShowAdvanced(form.path_glob !== '**' || form.tool_match.length > 0)
  }, [editing, open])

  function addChip() {
    const value = chipInput.trim()
    if (!value) return
    setForm((current) => ({ ...current, tool_match: [...current.tool_match, value] }))
    setChipInput('')
  }

  function removeChip(index: number) {
    setForm((current) => ({
      ...current,
      tool_match: current.tool_match.filter((_, currentIndex) => currentIndex !== index),
    }))
  }

  return (
    <Dialog open={open} onOpenChange={() => onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{editing ? 'Edit Route' : 'Add Route'}</DialogTitle>
          <DialogDescription>
            Routes match a workspace path and tool pattern, then attach the chosen downstream
            server and optional credential.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="space-y-2">
            <Label className="text-xs text-muted-foreground">Name (optional)</Label>
            <Input
              value={form.name}
              onChange={(e) => setForm((current) => ({ ...current, name: e.target.value }))}
              placeholder="e.g. GitHub allow-all"
            />
          </div>

          <div className="space-y-2">
            <Label className="text-xs text-muted-foreground">Workspace</Label>
            <Select
              value={form.workspace_id}
              onValueChange={(value) => setForm((current) => ({ ...current, workspace_id: value }))}
            >
              <SelectTrigger>
                <SelectValue placeholder="Select workspace..." />
              </SelectTrigger>
              <SelectContent>
                {workspaces.map((workspace) => (
                  <SelectItem key={workspace.id} value={workspace.id}>
                    {workspace.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label
              className={`text-xs text-muted-foreground ${form.policy === 'deny' ? 'opacity-50' : ''}`}
            >
              Downstream Server
            </Label>
            <Select
              value={form.downstream_server_id}
              onValueChange={(value) =>
                setForm((current) => ({ ...current, downstream_server_id: value }))
              }
              disabled={form.policy === 'deny'}
            >
              <SelectTrigger>
                <SelectValue
                  placeholder={form.policy === 'deny' ? 'N/A for deny rules' : 'Select server...'}
                />
              </SelectTrigger>
              <SelectContent>
                {downstreams.map((downstream) => (
                  <SelectItem key={downstream.id} value={downstream.id}>
                    {downstream.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label
              className={`text-xs text-muted-foreground ${form.policy === 'deny' ? 'opacity-50' : ''}`}
            >
              Auth Scope
            </Label>
            <Select
              value={form.auth_scope_id || 'none'}
              onValueChange={(value) =>
                setForm((current) => ({ ...current, auth_scope_id: value === 'none' ? '' : value }))
              }
              disabled={form.policy === 'deny'}
            >
              <SelectTrigger>
                <SelectValue placeholder={form.policy === 'deny' ? 'N/A for deny rules' : 'None'} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="none">None</SelectItem>
                {authScopes.map((scope) => (
                  <SelectItem key={scope.id} value={scope.id}>
                    {scope.name}
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
                onValueChange={(value) => {
                  const policy = value as 'allow' | 'deny'
                  if (policy === 'deny') {
                    setForm((current) => ({
                      ...current,
                      policy,
                      downstream_server_id: '',
                      auth_scope_id: '',
                    }))
                    return
                  }
                  setForm((current) => ({ ...current, policy }))
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
                  setForm((current) => ({ ...current, priority: Number(e.target.value) }))
                }
              />
            </div>
          </div>

          {form.policy === 'allow' && (
            <div className="space-y-3 rounded-md border border-border/50 p-3">
              <div className="space-y-2">
                <Label className="text-xs text-muted-foreground">Approval</Label>
                <Select
                  value={form.approval_mode}
                  onValueChange={(value) =>
                    setForm((current) => ({
                      ...current,
                      approval_mode: value as 'none' | 'write' | 'all',
                    }))
                  }
                >
                  <SelectTrigger className="w-48">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="none">None</SelectItem>
                    <SelectItem value="write">Write only (destructive)</SelectItem>
                    <SelectItem value="all">All tool calls</SelectItem>
                  </SelectContent>
                </Select>
                <p className="text-xs text-muted-foreground/60">
                  {form.approval_mode === 'none' && 'Tool calls execute without approval.'}
                  {form.approval_mode === 'write' &&
                    'Read-only tools execute freely; write or destructive tools require approval.'}
                  {form.approval_mode === 'all' &&
                    'Every tool call requires approval before execution.'}
                </p>
              </div>

              {form.approval_mode !== 'none' && (
                <div className="space-y-2">
                  <Label className="text-xs text-muted-foreground">Timeout (seconds)</Label>
                  <Input
                    type="number"
                    min={10}
                    value={form.approval_timeout}
                    onChange={(e) =>
                      setForm((current) => ({
                        ...current,
                        approval_timeout: Number(e.target.value),
                      }))
                    }
                    className="w-32"
                  />
                  <p className="text-xs text-muted-foreground/60">
                    How long to wait for approval before auto-denying the call.
                  </p>
                </div>
              )}
            </div>
          )}

          <button
            type="button"
            className="flex items-center gap-1.5 text-xs text-muted-foreground transition-colors hover:text-foreground"
            onClick={() => setShowAdvanced((current) => !current)}
          >
            {showAdvanced ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
            Advanced matching
            {!showAdvanced && !hasNonDefaultAdvanced && (
              <span className="text-muted-foreground/70">all paths, all tools</span>
            )}
          </button>

          {showAdvanced && (
            <div className="space-y-4 rounded-md border border-border/50 p-3">
              <div className="space-y-2">
                <Label className="text-xs text-muted-foreground">Path Glob</Label>
                <Input
                  className="font-mono text-sm"
                  value={form.path_glob}
                  onChange={(e) =>
                    setForm((current) => ({ ...current, path_glob: e.target.value }))
                  }
                  placeholder="**"
                />
                <p className="text-xs text-muted-foreground/60">
                  Matches the workspace subpath. Default <code className="font-mono">**</code>{' '}
                  matches everything.
                </p>
              </div>

              <div className="space-y-2">
                <Label className="text-xs text-muted-foreground">Tool Match</Label>
                <div className="mb-2 flex flex-wrap gap-1">
                  {form.tool_match.map((pattern, index) => (
                    <Badge key={`${pattern}-${index}`} variant="outline" className="gap-1 font-mono text-xs">
                      {pattern}
                      <button
                        type="button"
                        className="ml-0.5 hover:text-destructive"
                        onClick={() => removeChip(index)}
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
                  Leave blank to match every tool in the workspace.
                </p>
              </div>
            </div>
          )}
        </div>

        {saveError && <p className="text-sm text-destructive">{saveError}</p>}

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button
            onClick={onSave}
            disabled={
              saving ||
              !form.workspace_id ||
              (form.policy === 'allow' && !form.downstream_server_id)
            }
          >
            {saving ? 'Saving...' : 'Save'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
