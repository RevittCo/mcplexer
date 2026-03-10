import { useCallback, useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Badge } from '@/components/ui/badge'
import { deleteSecret, listSecretKeys, putSecret } from '@/api/client'
import { Eye, EyeOff, Plus, Trash2, Lock } from 'lucide-react'
import { toast } from 'sonner'

interface Props {
  open: boolean
  onClose: () => void
  authScopeId: string
  authScopeName: string
  serverName: string
}

export function ApiKeyDialog({ open, onClose, authScopeId, authScopeName, serverName }: Props) {
  const [keys, setKeys] = useState<string[]>([])
  const [loading, setLoading] = useState(false)
  const [newKey, setNewKey] = useState('')
  const [newValue, setNewValue] = useState('')
  const [showValue, setShowValue] = useState(false)
  const [saving, setSaving] = useState(false)

  const fetchKeys = useCallback(async () => {
    if (!authScopeId) return
    setLoading(true)
    try {
      const res = await listSecretKeys(authScopeId)
      setKeys(res.keys)
    } catch {
      // scope may not have secrets yet
      setKeys([])
    } finally {
      setLoading(false)
    }
  }, [authScopeId])

  useEffect(() => {
    if (open) {
      fetchKeys()
      setNewKey('')
      setNewValue('')
      setShowValue(false)
    }
  }, [open, fetchKeys])

  async function handleAdd() {
    if (!newKey.trim() || !newValue.trim()) return
    setSaving(true)
    try {
      await putSecret(authScopeId, newKey.trim(), newValue.trim())
      toast.success(`Secret "${newKey.trim()}" saved`)
      setNewKey('')
      setNewValue('')
      setShowValue(false)
      fetchKeys()
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : 'Failed to save secret')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(key: string) {
    try {
      await deleteSecret(authScopeId, key)
      toast.success(`Secret "${key}" removed`)
      fetchKeys()
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : 'Failed to delete secret')
    }
  }

  return (
    <Dialog open={open} onOpenChange={() => onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Lock className="h-4 w-4" />
            API Keys — {serverName}
          </DialogTitle>
        </DialogHeader>
        <p className="text-xs text-muted-foreground">
          Environment variables injected into <span className="font-mono">{serverName}</span> via
          auth scope <span className="font-mono">{authScopeName}</span>. Values are encrypted at rest.
        </p>

        {loading ? (
          <div className="flex items-center gap-2 text-muted-foreground text-sm py-4">
            <div className="h-2 w-2 animate-pulse rounded-full bg-primary" />
            Loading...
          </div>
        ) : (
          <div className="space-y-3">
            {keys.length > 0 && (
              <div className="space-y-1.5">
                {keys.map((key) => (
                  <div
                    key={key}
                    className="flex items-center justify-between rounded-md border border-border/50 px-3 py-2"
                  >
                    <div className="flex items-center gap-2">
                      <span className="font-mono text-sm">{key}</span>
                      <Badge variant="outline" className="text-[10px] text-emerald-500 border-emerald-500/30">
                        configured
                      </Badge>
                    </div>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-6 w-6 p-0 hover:bg-destructive/10 hover:text-destructive"
                      onClick={() => handleDelete(key)}
                    >
                      <Trash2 className="h-3 w-3" />
                    </Button>
                  </div>
                ))}
              </div>
            )}

            <div className="space-y-2 rounded-md border border-border/50 p-3">
              <div className="flex items-center gap-2 text-xs text-muted-foreground">
                <Plus className="h-3 w-3" />
                Add environment variable
              </div>
              <div className="space-y-2">
                <Label className="text-xs text-muted-foreground">Variable name</Label>
                <Input
                  className="font-mono text-sm"
                  value={newKey}
                  onChange={(e) => setNewKey(e.target.value.toUpperCase().replace(/[^A-Z0-9_]/g, ''))}
                  placeholder="AIKIDO_API_KEY"
                />
              </div>
              <div className="space-y-2">
                <Label className="text-xs text-muted-foreground">Value</Label>
                <div className="relative">
                  <Input
                    className="font-mono text-sm pr-8"
                    type={showValue ? 'text' : 'password'}
                    value={newValue}
                    onChange={(e) => setNewValue(e.target.value)}
                    placeholder="sk-..."
                    onKeyDown={(e) => e.key === 'Enter' && handleAdd()}
                  />
                  <button
                    type="button"
                    className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                    onClick={() => setShowValue(!showValue)}
                  >
                    {showValue ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
                  </button>
                </div>
              </div>
              <Button
                size="sm"
                onClick={handleAdd}
                disabled={saving || !newKey.trim() || !newValue.trim()}
              >
                {saving ? 'Saving...' : 'Save'}
              </Button>
            </div>
          </div>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Done
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
