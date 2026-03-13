import { useCallback, useEffect, useState } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useApi } from '@/hooks/use-api'
import { getSettings, updateSettings } from '@/api/client'
import type { Settings } from '@/api/types'
import { Loader2, RotateCcw, Save } from 'lucide-react'
import { toast } from 'sonner'

export function SettingsPage() {
  const fetcher = useCallback(() => getSettings(), [])
  const { data, loading } = useApi(fetcher)

  const [settings, setSettings] = useState<Settings | null>(null)
  const [saving, setSaving] = useState(false)
  const [dirty, setDirty] = useState(false)

  useEffect(() => {
    if (data) {
      setSettings({
        ...data.settings,
        tool_description_overrides: data.settings.tool_description_overrides ?? {},
      })
      setDirty(false)
    }
  }, [data])

  function patch(partial: Partial<Settings>) {
    setSettings((prev) => (prev ? { ...prev, ...partial } : prev))
    setDirty(true)
  }

  function patchOverride(toolName: string, description: string) {
    setSettings((prev) => {
      if (!prev) return prev
      const overrides = { ...prev.tool_description_overrides }
      if (description === '') {
        delete overrides[toolName]
      } else {
        overrides[toolName] = description
      }
      return { ...prev, tool_description_overrides: overrides }
    })
    setDirty(true)
  }

  function resetOverride(toolName: string) {
    patchOverride(toolName, '')
  }

  async function handleSave() {
    if (!settings) return
    setSaving(true)
    try {
      await updateSettings(settings)
      setDirty(false)
      toast.success('Settings saved')
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to save'
      toast.error(msg)
    } finally {
      setSaving(false)
    }
  }

  if (loading || !settings || !data) {
    return (
      <div className="flex items-center justify-center py-24">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  const builtinDefaults = data.builtin_tool_defaults
  const builtinNames = Object.keys(builtinDefaults).sort()

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Settings</h1>
        <Button onClick={handleSave} disabled={saving || !dirty}>
          {saving ? (
            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
          ) : (
            <Save className="mr-2 h-4 w-4" />
          )}
          Save
        </Button>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <div className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-sm font-medium uppercase tracking-wider text-muted-foreground">
                Tool Display
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex items-center justify-between">
                <div className="space-y-1">
                  <Label>Minify tool schemas (slim tools)</Label>
                  <p className="text-xs text-muted-foreground">
                    Strips property descriptions from schemas to save context window
                  </p>
                </div>
                <button
                  type="button"
                  role="switch"
                  aria-checked={settings.slim_tools}
                  onClick={() => patch({ slim_tools: !settings.slim_tools })}
                  className={`relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                    settings.slim_tools ? 'bg-primary' : 'bg-muted'
                  }`}
                >
                  <span
                    className={`pointer-events-none block h-5 w-5 rounded-full bg-background shadow-lg ring-0 transition-transform ${
                      settings.slim_tools ? 'translate-x-5' : 'translate-x-0'
                    }`}
                  />
                </button>
              </div>

              <div className="flex items-center justify-between border-t pt-4">
                <div className="space-y-1">
                  <Label>Codex dynamic tools compatibility mode</Label>
                  <p className="text-xs text-muted-foreground">
                    Auto-includes dynamic downstream tools in tools/list for Codex sessions.
                  </p>
                </div>
                <button
                  type="button"
                  role="switch"
                  aria-checked={settings.codex_dynamic_tool_compat}
                  onClick={() =>
                    patch({
                      codex_dynamic_tool_compat: !settings.codex_dynamic_tool_compat,
                    })
                  }
                  className={`relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                    settings.codex_dynamic_tool_compat ? 'bg-primary' : 'bg-muted'
                  }`}
                >
                  <span
                    className={`pointer-events-none block h-5 w-5 rounded-full bg-background shadow-lg ring-0 transition-transform ${
                      settings.codex_dynamic_tool_compat
                        ? 'translate-x-5'
                        : 'translate-x-0'
                    }`}
                  />
                </button>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-sm font-medium uppercase tracking-wider text-muted-foreground">
                Code Mode
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex items-center justify-between">
                <div className="space-y-1">
                  <Label>Enable Code Mode</Label>
                  <p className="text-xs text-muted-foreground">
                    Expose execute_code and get_code_api tools. LLMs write JavaScript to compose
                    tools instead of calling them individually.
                  </p>
                </div>
                <button
                  type="button"
                  role="switch"
                  aria-checked={settings.code_mode_enabled}
                  onClick={() => patch({ code_mode_enabled: !settings.code_mode_enabled })}
                  className={`relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                    settings.code_mode_enabled ? 'bg-primary' : 'bg-muted'
                  }`}
                >
                  <span
                    className={`pointer-events-none block h-5 w-5 rounded-full bg-background shadow-lg ring-0 transition-transform ${
                      settings.code_mode_enabled ? 'translate-x-5' : 'translate-x-0'
                    }`}
                  />
                </button>
              </div>

              {settings.code_mode_enabled && (
                <div className="space-y-2 border-t pt-4">
                  <Label>Execution timeout (seconds)</Label>
                  <p className="text-xs text-muted-foreground">
                    Maximum time a code execution can run before being killed (1-120)
                  </p>
                  <Input
                    type="number"
                    min={1}
                    max={120}
                    value={settings.code_mode_timeout_sec}
                    onChange={(e) =>
                      patch({ code_mode_timeout_sec: parseInt(e.target.value, 10) || 30 })
                    }
                    className="w-32"
                  />
                </div>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-sm font-medium uppercase tracking-wider text-muted-foreground">
                Caching
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="space-y-2">
                <Label>Tools list cache TTL (seconds)</Label>
                <p className="text-xs text-muted-foreground">
                  How often tools/list re-queries downstream servers (0-300)
                </p>
                <Input
                  type="number"
                  min={0}
                  max={300}
                  value={settings.tools_cache_ttl_sec}
                  onChange={(e) =>
                    patch({ tools_cache_ttl_sec: parseInt(e.target.value, 10) || 0 })
                  }
                  className="w-32"
                />
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-sm font-medium uppercase tracking-wider text-muted-foreground">
                Logging
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="space-y-2">
                <Label>Log level</Label>
                <Select
                  value={settings.log_level}
                  onValueChange={(v) => patch({ log_level: v })}
                >
                  <SelectTrigger className="w-40">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="debug">debug</SelectItem>
                    <SelectItem value="info">info</SelectItem>
                    <SelectItem value="warn">warn</SelectItem>
                    <SelectItem value="error">error</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </CardContent>
          </Card>
        </div>

        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-medium uppercase tracking-wider text-muted-foreground">
              Builtin Tool Descriptions
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-5">
            <p className="text-xs text-muted-foreground">
              Override the descriptions shown to MCP clients for each built-in tool.
              Clear a field to reset to the default.
            </p>
            {builtinNames.map((name) => {
              const defaultDesc = builtinDefaults[name]
              const currentOverride = settings.tool_description_overrides?.[name] ?? ''
              const isOverridden = currentOverride !== '' && currentOverride !== defaultDesc

              return (
                <div key={name} className="space-y-1.5">
                  <div className="flex items-center gap-2">
                    <Label className="font-mono text-xs">{name}</Label>
                    {isOverridden && (
                      <button
                        type="button"
                        onClick={() => resetOverride(name)}
                        className="flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] text-muted-foreground hover:bg-muted hover:text-foreground transition-colors"
                        title="Reset to default"
                      >
                        <RotateCcw className="h-3 w-3" />
                        Reset
                      </button>
                    )}
                  </div>
                  <Textarea
                    value={currentOverride || defaultDesc}
                    onChange={(e) => patchOverride(name, e.target.value)}
                    className={`min-h-[60px] font-mono text-xs ${
                      isOverridden
                        ? 'border-primary/40 bg-primary/5'
                        : ''
                    }`}
                    rows={2}
                  />
                </div>
              )
            })}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
