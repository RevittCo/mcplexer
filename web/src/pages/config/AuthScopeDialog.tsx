import { useCallback, useEffect, useState } from 'react'
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
import { Link } from 'react-router-dom'
import { deleteSecret, listSecretKeys, putSecret } from '@/api/client'
import { Eye, EyeOff, Trash2 } from 'lucide-react'
import { toast } from 'sonner'

export interface AuthScopeFormData {
  name: string
  type: 'env' | 'header' | 'oauth2'
  oauth_provider_id: string
  redaction_hints: string[]
}

export const emptyAuthScopeForm: AuthScopeFormData = {
  name: '',
  type: 'env',
  oauth_provider_id: '',
  redaction_hints: [],
}

export function AuthScopeDialog({
  open,
  onClose,
  form,
  setForm,
  onSave,
  saving,
  editing,
  editingId,
  envFields,
  providers,
  saveError,
}: {
  open: boolean
  onClose: () => void
  form: AuthScopeFormData
  setForm: React.Dispatch<React.SetStateAction<AuthScopeFormData>>
  onSave: () => void
  saving: boolean
  editing: boolean
  editingId?: string
  envFields?: { key: string; label: string; secret: boolean }[]
  saveError: string | null
  providers: { id: string; name: string }[]
}) {
  const [hintInput, setHintInput] = useState('')

  // Secrets management for env-type scopes
  const [secretKeys, setSecretKeys] = useState<string[]>([])
  // For labeled fields: track values by key
  const [fieldValues, setFieldValues] = useState<Record<string, string>>({})
  const [fieldVisibility, setFieldVisibility] = useState<Record<string, boolean>>({})
  // For generic fallback
  const [newKey, setNewKey] = useState('')
  const [newValue, setNewValue] = useState('')
  const [showValue, setShowValue] = useState(false)
  const [savingSecret, setSavingSecret] = useState(false)

  const hasLabeledFields = editing && form.type === 'env' && envFields && envFields.length > 0

  const fetchSecretKeys = useCallback(async () => {
    if (!editingId || (form.type !== 'env' && form.type !== 'header')) return
    try {
      const res = await listSecretKeys(editingId)
      setSecretKeys(res.keys)
    } catch {
      setSecretKeys([])
    }
  }, [editingId, form.type])

  useEffect(() => {
    if (open && editing) {
      fetchSecretKeys()
    }
    if (open) {
      setFieldValues({})
      setFieldVisibility({})
      setNewKey('')
      setNewValue('')
      setShowValue(false)
    }
  }, [open, editing, fetchSecretKeys])

  async function handleSaveField(key: string) {
    const value = fieldValues[key]
    if (!editingId || !value?.trim()) return
    setSavingSecret(true)
    try {
      await putSecret(editingId, key, value.trim())
      toast.success(`Saved`)
      setFieldValues((v) => ({ ...v, [key]: '' }))
      fetchSecretKeys()
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : 'Failed to save')
    } finally {
      setSavingSecret(false)
    }
  }

  async function handleSaveAllFields() {
    if (!editingId || !envFields) return
    const toSave = envFields.filter((f) => fieldValues[f.key]?.trim())
    if (toSave.length === 0) return
    setSavingSecret(true)
    try {
      for (const f of toSave) {
        await putSecret(editingId, f.key, fieldValues[f.key].trim())
      }
      toast.success('Credentials saved')
      setFieldValues({})
      fetchSecretKeys()
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : 'Failed to save')
    } finally {
      setSavingSecret(false)
    }
  }

  async function handleAddSecret() {
    if (!editingId || !newKey.trim() || !newValue.trim()) return
    setSavingSecret(true)
    try {
      await putSecret(editingId, newKey.trim(), newValue.trim())
      toast.success(`Secret "${newKey.trim()}" saved`)
      setNewKey('')
      setNewValue('')
      setShowValue(false)
      fetchSecretKeys()
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : 'Failed to save secret')
    } finally {
      setSavingSecret(false)
    }
  }

  async function handleDeleteSecret(key: string) {
    if (!editingId) return
    try {
      await deleteSecret(editingId, key)
      toast.success(`Secret "${key}" removed`)
      fetchSecretKeys()
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : 'Failed to delete secret')
    }
  }

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

  return (
    <Dialog open={open} onOpenChange={() => onClose()}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{editing ? 'Edit Credential' : 'Add Credential'}</DialogTitle>
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
              onValueChange={(v) =>
                setForm((f) => ({ ...f, type: v as 'env' | 'header' | 'oauth2' }))
              }
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

          {editing && form.type === 'env' && editingId && hasLabeledFields && (
            <div className="space-y-3 rounded-md border border-border/50 p-3">
              <Label className="text-xs font-semibold">Credentials</Label>
              {envFields.map((field) => {
                const isSet = secretKeys.includes(field.key)
                const isSecret = field.secret
                const visible = fieldVisibility[field.key] ?? false
                return (
                  <div key={field.key} className="space-y-1">
                    <div className="flex items-center gap-2">
                      <Label className="text-xs text-muted-foreground">{field.label}</Label>
                      {isSet && (
                        <Badge variant="outline" className="text-[10px] text-emerald-500 border-emerald-500/30">
                          set
                        </Badge>
                      )}
                    </div>
                    <div className="relative">
                      <Input
                        className="font-mono text-sm h-8 pr-7"
                        type={isSecret && !visible ? 'password' : 'text'}
                        value={fieldValues[field.key] ?? ''}
                        onChange={(e) => setFieldValues((v) => ({ ...v, [field.key]: e.target.value }))}
                        placeholder={isSet ? '••••••••  (enter new value to replace)' : `Enter ${field.label.toLowerCase()}`}
                        onKeyDown={(e) => e.key === 'Enter' && handleSaveField(field.key)}
                      />
                      {isSecret && (
                        <button
                          type="button"
                          className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                          onClick={() => setFieldVisibility((v) => ({ ...v, [field.key]: !visible }))}
                        >
                          {visible ? <EyeOff className="h-3 w-3" /> : <Eye className="h-3 w-3" />}
                        </button>
                      )}
                    </div>
                  </div>
                )
              })}
              <Button
                size="sm"
                onClick={handleSaveAllFields}
                disabled={savingSecret || !envFields.some((f) => fieldValues[f.key]?.trim())}
              >
                {savingSecret ? 'Saving...' : 'Save Credentials'}
              </Button>
              <p className="text-[10px] text-muted-foreground">Values are encrypted at rest.</p>
            </div>
          )}

          {editing && form.type === 'env' && editingId && !hasLabeledFields && (
            <div className="space-y-3 rounded-md border border-border/50 p-3">
              <Label className="text-xs font-semibold">Environment Variables (encrypted)</Label>
              {secretKeys.length > 0 && (
                <div className="space-y-1.5">
                  {secretKeys.map((key) => (
                    <div key={key} className="flex items-center justify-between rounded-md border border-border/50 px-3 py-2">
                      <div className="flex items-center gap-2">
                        <span className="font-mono text-sm">{key}</span>
                        <Badge variant="outline" className="text-[10px] text-emerald-500 border-emerald-500/30">set</Badge>
                      </div>
                      <Button variant="ghost" size="sm" className="h-6 w-6 p-0 hover:bg-destructive/10 hover:text-destructive" onClick={() => handleDeleteSecret(key)}>
                        <Trash2 className="h-3 w-3" />
                      </Button>
                    </div>
                  ))}
                </div>
              )}
              <div className="space-y-2">
                <div className="grid grid-cols-2 gap-2">
                  <div className="space-y-1">
                    <Label className="text-[10px] text-muted-foreground">Variable name</Label>
                    <Input className="font-mono text-sm h-8" value={newKey} onChange={(e) => setNewKey(e.target.value.toUpperCase().replace(/[^A-Z0-9_]/g, ''))} placeholder="API_KEY" />
                  </div>
                  <div className="space-y-1">
                    <Label className="text-[10px] text-muted-foreground">Value</Label>
                    <div className="relative">
                      <Input className="font-mono text-sm h-8 pr-7" type={showValue ? 'text' : 'password'} value={newValue} onChange={(e) => setNewValue(e.target.value)} placeholder="sk-..." onKeyDown={(e) => e.key === 'Enter' && handleAddSecret()} />
                      <button type="button" className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground" onClick={() => setShowValue(!showValue)}>
                        {showValue ? <EyeOff className="h-3 w-3" /> : <Eye className="h-3 w-3" />}
                      </button>
                    </div>
                  </div>
                </div>
                <Button size="sm" variant="outline" className="h-7 text-xs" onClick={handleAddSecret} disabled={savingSecret || !newKey.trim() || !newValue.trim()}>
                  {savingSecret ? 'Saving...' : 'Add Secret'}
                </Button>
              </div>
            </div>
          )}

          {editing && form.type === 'header' && editingId && (
            <div className="space-y-3 rounded-md border border-border/50 p-3">
              <Label className="text-xs font-semibold">HTTP Headers (encrypted)</Label>
              {secretKeys.length > 0 && (
                <div className="space-y-1.5">
                  {secretKeys.map((key) => (
                    <div key={key} className="flex items-center justify-between rounded-md border border-border/50 px-3 py-2">
                      <div className="flex items-center gap-2">
                        <span className="font-mono text-sm">{key}</span>
                        <Badge variant="outline" className="text-[10px] text-emerald-500 border-emerald-500/30">set</Badge>
                      </div>
                      <Button variant="ghost" size="sm" className="h-6 w-6 p-0 hover:bg-destructive/10 hover:text-destructive" onClick={() => handleDeleteSecret(key)}>
                        <Trash2 className="h-3 w-3" />
                      </Button>
                    </div>
                  ))}
                </div>
              )}
              <div className="space-y-2">
                <div className="grid grid-cols-2 gap-2">
                  <div className="space-y-1">
                    <Label className="text-[10px] text-muted-foreground">Header name</Label>
                    <Input className="font-mono text-sm h-8" value={newKey} onChange={(e) => setNewKey(e.target.value)} placeholder="Authorization" />
                  </div>
                  <div className="space-y-1">
                    <Label className="text-[10px] text-muted-foreground">Header value</Label>
                    <div className="relative">
                      <Input className="font-mono text-sm h-8 pr-7" type={showValue ? 'text' : 'password'} value={newValue} onChange={(e) => setNewValue(e.target.value)} placeholder="Bearer sk-..." onKeyDown={(e) => e.key === 'Enter' && handleAddSecret()} />
                      <button type="button" className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground" onClick={() => setShowValue(!showValue)}>
                        {showValue ? <EyeOff className="h-3 w-3" /> : <Eye className="h-3 w-3" />}
                      </button>
                    </div>
                  </div>
                </div>
                <Button size="sm" variant="outline" className="h-7 text-xs" onClick={handleAddSecret} disabled={savingSecret || !newKey.trim() || !newValue.trim()}>
                  {savingSecret ? 'Saving...' : 'Add Header'}
                </Button>
                <p className="text-[10px] text-muted-foreground">Headers are encrypted at rest and injected into HTTP requests to the downstream server.</p>
              </div>
            </div>
          )}

          {form.type === 'oauth2' && (
            <div className="space-y-3">
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
              {!editing && (
                <p className="text-xs text-muted-foreground">
                  For easy setup with templates, use{' '}
                  <Link to="/setup" className="text-primary hover:underline" onClick={onClose}>
                    Quick Setup &rarr;
                  </Link>
                </p>
              )}
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
