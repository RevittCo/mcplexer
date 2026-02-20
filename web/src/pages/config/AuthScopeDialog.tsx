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
import { Link } from 'react-router-dom'

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
  saveError: string | null
  providers: { id: string; name: string }[]
}) {
  const [hintInput, setHintInput] = useState('')

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
