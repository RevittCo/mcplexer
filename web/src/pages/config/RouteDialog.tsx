import { useState } from 'react'
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
  requires_approval: boolean
  approval_timeout: number
}

export const emptyForm: RouteFormData = {
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
  const [prevForm, setPrevForm] = useState(form)

  if (form !== prevForm) {
    setPrevForm(form)
    setChipInput('')
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
              onValueChange={(v) => setForm((f) => ({ ...f, downstream_server_id: v }))}
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
              onValueChange={(v) => setForm((f) => ({ ...f, auth_scope_id: v === 'none' ? '' : v }))}
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
                    setForm((f) => ({ ...f, policy, downstream_server_id: '', auth_scope_id: '' }))
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
                onChange={(e) => setForm((f) => ({ ...f, priority: Number(e.target.value) }))}
              />
            </div>
          </div>

          {form.policy === 'allow' && (
            <div className="space-y-3 rounded-md border border-border/50 p-3">
              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  checked={form.requires_approval}
                  onChange={(e) => setForm((f) => ({ ...f, requires_approval: e.target.checked }))}
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
                    onChange={(e) => setForm((f) => ({ ...f, approval_timeout: Number(e.target.value) }))}
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
            {showAdvanced ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
            Advanced
            {!showAdvanced && !hasNonDefaultAdvanced && (
              <span className="text-muted-foreground/60"> — all paths, all tools</span>
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
                    <Badge key={i} variant="outline" className="gap-1 font-mono text-xs">
                      {pattern}
                      <button type="button" className="ml-0.5 hover:text-destructive" onClick={() => removeChip(i)}>
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
        {saveError && <p className="text-sm text-destructive">{saveError}</p>}
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
