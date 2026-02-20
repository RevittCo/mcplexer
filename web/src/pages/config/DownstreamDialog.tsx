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
import { Checkbox } from '@/components/ui/checkbox'
import { ChevronDown, ChevronRight } from 'lucide-react'
import type { ServerCacheConfig } from '@/api/types'

export interface DownstreamFormData {
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
  cache_config?: ServerCacheConfig
}

export const emptyDownstreamForm: DownstreamFormData = {
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

export function DownstreamDialog({
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
  form: DownstreamFormData
  setForm: React.Dispatch<React.SetStateAction<DownstreamFormData>>
  onSave: () => void
  saving: boolean
  editing: boolean
  saveError: string | null
}) {
  const [showCaching, setShowCaching] = useState(false)

  const cacheEnabled = form.cache_config?.enabled ?? true
  const cacheTTL = form.cache_config?.read_ttl_sec ?? 1800
  const cacheMaxEntries = form.cache_config?.max_entries ?? 1000

  function updateCache(patch: Partial<ServerCacheConfig>) {
    setForm((f) => ({
      ...f,
      cache_config: { ...f.cache_config, ...patch },
    }))
  }

  function formatTTLHint(sec: number): string {
    if (sec === 0) return 'indefinite'
    if (sec < 60) return `${sec}s`
    if (sec < 3600) return `${Math.round(sec / 60)}m`
    return `${(sec / 3600).toFixed(1)}h`
  }

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

          <button
            type="button"
            className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors"
            onClick={() => setShowCaching(!showCaching)}
          >
            {showCaching ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
            Caching
            {!showCaching && (
              <span className="text-muted-foreground/60">
                — {cacheEnabled ? `${formatTTLHint(cacheTTL)} TTL` : 'disabled'}
              </span>
            )}
          </button>

          {showCaching && (
            <div className="space-y-3 rounded-md border border-border/50 p-3">
              <label className="flex items-center gap-2 cursor-pointer">
                <Checkbox
                  checked={cacheEnabled}
                  onCheckedChange={(checked) => updateCache({ enabled: checked === true })}
                />
                <span className="text-sm">Enable caching</span>
              </label>

              {cacheEnabled && (
                <>
                  <div className="space-y-2">
                    <Label className="text-xs text-muted-foreground">TTL (seconds)</Label>
                    <Input
                      type="number"
                      min={0}
                      value={cacheTTL}
                      onChange={(e) => updateCache({ read_ttl_sec: Number(e.target.value) })}
                      className="w-32"
                    />
                    <p className="text-xs text-muted-foreground/60">
                      0 = indefinite. Default is 1800 (30 min).
                    </p>
                  </div>
                  <div className="space-y-2">
                    <Label className="text-xs text-muted-foreground">Max Entries</Label>
                    <Input
                      type="number"
                      min={1}
                      value={cacheMaxEntries}
                      onChange={(e) => updateCache({ max_entries: Number(e.target.value) })}
                      className="w-32"
                    />
                  </div>
                </>
              )}
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
          <Button onClick={onSave} disabled={saving || !form.name}>
            {saving ? 'Saving...' : 'Save'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
