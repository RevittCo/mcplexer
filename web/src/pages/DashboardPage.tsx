import { useCallback, useMemo, useState } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { useApi } from '@/hooks/use-api'
import { useInterval } from '@/hooks/use-interval'
import { useAuditStream } from '@/hooks/use-audit-stream'
import { getDashboard, listAuthScopes, listWorkspaces } from '@/api/client'
import type {
  AuditRecord,
  AuditStats,
  TimeSeriesPoint,
  ToolLeaderboardEntry,
  ServerHealthEntry,
  ErrorBreakdownEntry,
  RouteHitEntry,
  ApprovalMetrics,
  SessionInfo,
  CacheStats,
} from '@/api/types'
import { Activity, AlertTriangle, Clock, Database, Server, ShieldCheck } from 'lucide-react'
import {
  Area,
  AreaChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { AuditDetailDialog, ReasonBadge } from '@/components/AuditDetailDialog'
import { useApprovalStream } from '@/hooks/use-approval-stream'
import { Link } from 'react-router-dom'

type TimeRange = '1h' | '6h' | '24h' | '7d'

function formatTime(ts: string): string {
  return new Date(ts).toLocaleTimeString()
}

function formatHHMM(ts: string, range: TimeRange): string {
  const d = new Date(ts)
  if (range === '7d') {
    return `${String(d.getMonth() + 1).padStart(2, '0')}/${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
  }
  if (range === '24h') {
    return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
  }
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

function formatDuration(connectedAt: string): string {
  const ms = Date.now() - new Date(connectedAt).getTime()
  const minutes = Math.floor(ms / 60000)
  if (minutes < 60) return `${minutes}m`
  const hours = Math.floor(minutes / 60)
  const remainMin = minutes % 60
  if (hours < 24) return `${hours}h ${remainMin}m`
  const days = Math.floor(hours / 24)
  return `${days}d ${hours % 24}h`
}

interface ChartPoint {
  time: string
  value: number
}

function prepareChartData(
  points: TimeSeriesPoint[],
  accessor: (p: TimeSeriesPoint) => number,
): ChartPoint[] {
  return points.map((p) => ({
    time: p.bucket,
    value: accessor(p),
  }))
}

function ChartTooltip({
  active,
  payload,
  label,
  suffix,
  range,
}: {
  active?: boolean
  payload?: { value: number }[]
  label?: string
  suffix?: string
  range: TimeRange
}) {
  if (!active || !payload?.length || !label) return null
  return (
    <div className="border border-border bg-card px-3 py-1.5 font-mono text-xs shadow-lg">
      <div className="text-muted-foreground">{formatHHMM(label, range)}</div>
      <div className="text-foreground">
        {payload[0].value}
        {suffix}
      </div>
    </div>
  )
}

const chartColors = {
  primary: { stroke: 'hsl(205, 95%, 55%)', fill: 'hsl(205, 95%, 55%)' },
  green: { stroke: 'hsl(160, 70%, 45%)', fill: 'hsl(160, 70%, 45%)' },
  red: { stroke: 'hsl(0, 72%, 51%)', fill: 'hsl(0, 72%, 51%)' },
  amber: { stroke: 'hsl(38, 92%, 50%)', fill: 'hsl(38, 92%, 50%)' },
} as const

function MetricChart({
  label,
  value,
  subtitle,
  icon,
  data,
  colorKey,
  suffix,
  range,
}: {
  label: string
  value: React.ReactNode
  subtitle?: React.ReactNode
  icon: React.ReactNode
  data: ChartPoint[]
  colorKey: keyof typeof chartColors
  suffix?: string
  range: TimeRange
}) {
  const color = chartColors[colorKey]
  const gradientId = `gradient-${colorKey}`
  const colorClass =
    colorKey === 'primary' ? 'text-primary'
    : colorKey === 'green' ? 'text-chart-2'
    : colorKey === 'amber' ? 'text-amber-400'
    : 'text-chart-5'

  return (
    <div className="relative overflow-hidden rounded-lg border border-border/50 bg-card pb-24">
      <div className="p-5">
        <div className="flex items-center gap-2 text-muted-foreground">
          <span className={colorClass}>{icon}</span>
          <span className="text-[11px] uppercase tracking-widest">{label}</span>
        </div>
        <div className={`mt-3 text-3xl font-bold tracking-tight md:text-4xl ${colorClass}`}>
          {value}
        </div>
        {subtitle && <div className="mt-1">{subtitle}</div>}
      </div>
      <div className="absolute bottom-0 left-0 right-0 h-24 select-none [&_svg]:outline-none [&_svg]:!cursor-default [&_.recharts-surface]:!outline-none">
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={data} margin={{ top: 0, right: 0, left: 0, bottom: 0 }}>
            <defs>
              <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor={color.fill} stopOpacity={0.2} />
                <stop offset="100%" stopColor={color.fill} stopOpacity={0} />
              </linearGradient>
            </defs>
            <YAxis domain={[0, (max: number) => Math.max(max, 100)]} hide />
            <XAxis
              dataKey="time"
              tickFormatter={(ts: string) => formatHHMM(ts, range)}
              tick={{ fontSize: 9, fontFamily: 'monospace', fill: '#6b7280' }}
              axisLine={false}
              tickLine={false}
              interval="preserveStartEnd"
              minTickGap={40}
            />
            <Tooltip
              content={<ChartTooltip suffix={suffix} range={range} />}
              cursor={{ stroke: '#374151', strokeDasharray: '3 3' }}
            />
            <Area
              type="monotone"
              dataKey="value"
              stroke={color.stroke}
              strokeWidth={1.5}
              fill={`url(#${gradientId})`}
              dot={false}
              activeDot={false}
            />
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </div>
  )
}

function isBlocked(record: AuditRecord): boolean {
  return record.status === 'blocked'
}

function errorRateColor(rate: number): string {
  if (rate < 5) return 'text-chart-2'
  if (rate < 20) return 'text-amber-400'
  return 'text-destructive'
}

function healthBorderColor(entry: ServerHealthEntry, state: string): string {
  if (state === 'stopped') return 'border-destructive/50'
  if (state === 'external') return 'border-border/50'
  if (entry.error_rate > 25) return 'border-destructive/50'
  if (entry.error_rate > 10) return 'border-amber-500/50'
  return 'border-chart-2/50'
}

function ToolLeaderboardTable({ entries }: { entries: ToolLeaderboardEntry[] }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium uppercase tracking-wider text-muted-foreground">
          Tool Leaderboard
        </CardTitle>
      </CardHeader>
      <CardContent>
        {entries.length === 0 ? (
          <div className="py-6 text-center text-xs text-muted-foreground/60">
            No tool calls in this period
          </div>
        ) : (
        <Table>
          <TableHeader>
            <TableRow className="border-border/50 hover:bg-transparent">
              <TableHead>Tool</TableHead>
              <TableHead className="text-right">Calls</TableHead>
              <TableHead className="text-right">Errors</TableHead>
              <TableHead className="text-right">Error %</TableHead>
              <TableHead className="hidden sm:table-cell text-right">Avg Lat.</TableHead>
              <TableHead className="hidden md:table-cell text-right">P95 Lat.</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {entries.map((e) => (
              <TableRow key={e.tool_name} className="border-border/30">
                <TableCell>
                  <div className="max-w-[14rem] truncate font-mono text-sm">{e.tool_name}</div>
                  {e.server_name && (
                    <div className="text-xs text-muted-foreground">{e.server_name}</div>
                  )}
                </TableCell>
                <TableCell className="text-right font-mono text-sm">{e.call_count}</TableCell>
                <TableCell className="text-right font-mono text-sm">{e.error_count}</TableCell>
                <TableCell className={`text-right font-mono text-sm ${errorRateColor(e.error_rate)}`}>
                  {e.error_rate.toFixed(1)}%
                </TableCell>
                <TableCell className="hidden sm:table-cell text-right font-mono text-sm text-muted-foreground">
                  {Math.round(e.avg_latency_ms)}ms
                </TableCell>
                <TableCell className="hidden md:table-cell text-right font-mono text-sm text-muted-foreground">
                  {e.p95_latency_ms}ms
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        )}
      </CardContent>
    </Card>
  )
}

function ErrorBreakdownCard({ entries }: { entries: ErrorBreakdownEntry[] }) {
  if (entries.length === 0) return null
  const max = entries[0]?.count ?? 1
  return (
    <Card className="border-destructive/30">
      <CardHeader>
        <CardTitle className="text-sm font-medium uppercase tracking-wider text-destructive">
          Error Breakdown
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-2">
        {entries.map((e) => {
          const blocked = e.error_type === 'blocked'
          return (
            <div key={`${e.group_key}-${e.error_type}`} className="space-y-1">
              <div className="flex items-center justify-between text-sm">
                <div className="flex items-center gap-2 min-w-0">
                  <span className="max-w-[10rem] truncate font-mono">{e.group_key}</span>
                  <Badge
                    variant="outline"
                    className={`shrink-0 text-[10px] px-1.5 py-0 ${
                      blocked
                        ? 'border-amber-500/40 text-amber-500'
                        : 'border-destructive/40 text-destructive'
                    }`}
                  >
                    {blocked ? 'blocked' : 'error'}
                  </Badge>
                </div>
                <span className={`font-mono ${blocked ? 'text-amber-500' : 'text-destructive'}`}>
                  {e.count}
                </span>
              </div>
              <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
                <div
                  className={`h-full rounded-full ${blocked ? 'bg-amber-500/60' : 'bg-destructive/60'}`}
                  style={{ width: `${(e.count / max) * 100}%` }}
                />
              </div>
            </div>
          )
        })}
      </CardContent>
    </Card>
  )
}

function ServerHealthCards({
  entries,
  downstreams,
}: {
  entries: ServerHealthEntry[]
  downstreams: { server_id: string; state: string; disabled?: boolean }[]
}) {
  const [showIdle, setShowIdle] = useState(false)
  const [showDisabled, setShowDisabled] = useState(false)
  const dsMap = new Map(downstreams.map((d) => [d.server_id, d]))
  const stateMap = new Map(downstreams.map((d) => [d.server_id, d.state]))
  const healthMap = new Map(entries.map((e) => [e.server_id, e]))
  const allServerIds = [...new Set([
    ...downstreams.map((d) => d.server_id),
    ...entries.map((e) => e.server_id),
  ])]

  // Split into disabled, active, and idle
  const disabledIds = allServerIds.filter((id) => dsMap.get(id)?.disabled)
  const enabledIds = allServerIds.filter((id) => !dsMap.get(id)?.disabled)
  const activeIds = enabledIds.filter((id) => {
    const health = healthMap.get(id)
    const state = stateMap.get(id) ?? 'unknown'
    return (health && health.call_count > 0) || (state !== 'stopped' && state !== 'external')
  })
  const idleIds = enabledIds.filter((id) => !activeIds.includes(id))

  const renderCard = (id: string) => {
    const health = healthMap.get(id)
    const state = stateMap.get(id) ?? 'unknown'
    const ds = downstreams.find((d) => d.server_id === id)
    const serverName = health?.server_name ?? ds?.server_id ?? id
    const isDisabled = state === 'disabled'
    const borderColor = isDisabled ? 'border-border/30' : health ? healthBorderColor(health, state) : 'border-border/50'
    return (
      <div
        key={id}
        className={`rounded-lg border-2 ${borderColor} bg-card p-4 space-y-2 ${isDisabled ? 'opacity-50' : ''}`}
      >
        <div className="flex items-center justify-between">
          <span className="text-sm font-medium truncate max-w-[10rem]">{serverName}</span>
          <Badge variant={
            state === 'stopped' ? 'destructive'
            : state === 'disabled' ? 'outline'
            : state === 'external' ? 'outline'
            : 'secondary'
          }>
            {state}
          </Badge>
        </div>
        {!isDisabled && health && health.call_count > 0 ? (
          <div className="grid grid-cols-3 gap-2 text-xs">
            <div>
              <div className="text-muted-foreground">Calls</div>
              <div className="font-mono font-medium">{health.call_count}</div>
            </div>
            <div>
              <div className="text-muted-foreground">Error %</div>
              <div className={`font-mono font-medium ${errorRateColor(health.error_rate)}`}>
                {health.error_rate.toFixed(1)}%
              </div>
            </div>
            <div>
              <div className="text-muted-foreground">Avg Lat.</div>
              <div className="font-mono font-medium">{Math.round(health.avg_latency_ms)}ms</div>
            </div>
          </div>
        ) : !isDisabled ? (
          <div className="text-xs text-muted-foreground">No calls in period</div>
        ) : null}
      </div>
    )
  }

  const collapsibleSection = (
    ids: string[],
    label: string,
    show: boolean,
    setShow: (v: boolean) => void,
  ) => (
    <div>
      <button
        onClick={() => setShow(!show)}
        className="text-xs text-muted-foreground hover:text-foreground transition-colors"
      >
        {show ? 'Hide' : 'Show'} {ids.length} {label} server{ids.length !== 1 ? 's' : ''}
        <span className="ml-1">{show ? '\u25B2' : '\u25BC'}</span>
      </button>
      {show && (
        <div className="mt-3 grid gap-3 sm:grid-cols-2">
          {ids.map(renderCard)}
        </div>
      )}
    </div>
  )

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium uppercase tracking-wider text-muted-foreground">
          Server Health
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {activeIds.length === 0 && idleIds.length === 0 && disabledIds.length === 0 && (
          <div className="py-6 text-center text-xs text-muted-foreground/60">
            No servers configured
          </div>
        )}
        {activeIds.length > 0 && (
          <div className="grid gap-3 sm:grid-cols-2">
            {activeIds.map(renderCard)}
          </div>
        )}
        {idleIds.length > 0 && collapsibleSection(idleIds, 'idle', showIdle, setShowIdle)}
        {disabledIds.length > 0 && collapsibleSection(disabledIds, 'disabled', showDisabled, setShowDisabled)}
      </CardContent>
    </Card>
  )
}

function ActiveSessionsTable({
  sessions,
  wsName,
}: {
  sessions: SessionInfo[]
  wsName: (id: string) => string
}) {
  const displayLimit = 10
  const shown = sessions.slice(0, displayLimit)
  const remaining = sessions.length - shown.length
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle className="text-sm font-medium uppercase tracking-wider text-muted-foreground">
            Active Sessions
          </CardTitle>
          <span className="text-xs text-muted-foreground font-mono">{sessions.length} total</span>
        </div>
      </CardHeader>
      <CardContent>
        {sessions.length === 0 ? (
          <div className="py-6 text-center text-xs text-muted-foreground/60">
            No active sessions
          </div>
        ) : (
        <>
        <Table>
          <TableHeader>
            <TableRow className="border-border/50 hover:bg-transparent">
              <TableHead>Session</TableHead>
              <TableHead>Model</TableHead>
              <TableHead>Client</TableHead>
              <TableHead className="hidden sm:table-cell">Workspace</TableHead>
              <TableHead className="hidden md:table-cell text-right">PID</TableHead>
              <TableHead className="text-right">Connected</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {shown.map((s) => (
              <TableRow key={s.id} className="border-border/30">
                <TableCell className="font-mono text-xs text-muted-foreground">
                  {s.id.slice(0, 8)}
                </TableCell>
                <TableCell className="font-mono text-sm">
                  {s.model_hint || '-'}
                </TableCell>
                <TableCell className="text-sm text-muted-foreground">{s.client_type}</TableCell>
                <TableCell className="hidden sm:table-cell text-sm text-muted-foreground">
                  {s.workspace_id ? wsName(s.workspace_id) : '-'}
                </TableCell>
                <TableCell className="hidden md:table-cell text-right font-mono text-xs text-muted-foreground">
                  {s.client_pid ?? '-'}
                </TableCell>
                <TableCell className="text-right font-mono text-sm text-muted-foreground">
                  {formatDuration(s.connected_at)}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        {remaining > 0 && (
          <div className="mt-2 text-center text-xs text-muted-foreground">
            +{remaining} more session{remaining !== 1 ? 's' : ''}
          </div>
        )}
        </>
        )}
      </CardContent>
    </Card>
  )
}

function RouteHitMapTable({ entries }: { entries: RouteHitEntry[] }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium uppercase tracking-wider text-muted-foreground">
          Route Hits
        </CardTitle>
      </CardHeader>
      <CardContent>
        {entries.length === 0 ? (
          <div className="py-6 text-center text-xs text-muted-foreground/60">
            No route hits in this period
          </div>
        ) : (
        <Table>
          <TableHeader>
            <TableRow className="border-border/50 hover:bg-transparent">
              <TableHead>Rule</TableHead>
              <TableHead className="text-right">Hits</TableHead>
              <TableHead className="text-right">Errors</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {entries.map((e) => (
              <TableRow key={e.route_rule_id} className="border-border/30">
                <TableCell>
                  <div className="font-mono text-sm">{e.rule_name || e.path_glob || e.route_rule_id}</div>
                  {e.rule_name && e.path_glob && (
                    <div className="text-xs text-muted-foreground font-mono">{e.path_glob}</div>
                  )}
                </TableCell>
                <TableCell className="text-right font-mono text-sm">{e.hit_count}</TableCell>
                <TableCell className="text-right font-mono text-sm text-destructive">
                  {e.error_count > 0 ? e.error_count : '-'}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        )}
      </CardContent>
    </Card>
  )
}

function ApprovalMetricsCard({ metrics }: { metrics: ApprovalMetrics }) {
  const total = metrics.pending_count + metrics.approved_count + metrics.denied_count + metrics.timed_out_count
  if (total === 0) return null
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium uppercase tracking-wider text-muted-foreground">
          Approval Metrics
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <div className="text-xs text-muted-foreground">Pending</div>
            <div className="text-lg font-bold text-amber-400">{metrics.pending_count}</div>
          </div>
          <div>
            <div className="text-xs text-muted-foreground">Approved</div>
            <div className="text-lg font-bold text-chart-2">{metrics.approved_count}</div>
          </div>
          <div>
            <div className="text-xs text-muted-foreground">Denied</div>
            <div className="text-lg font-bold text-destructive">{metrics.denied_count}</div>
          </div>
          <div>
            <div className="text-xs text-muted-foreground">Timed Out</div>
            <div className="text-lg font-bold text-muted-foreground">{metrics.timed_out_count}</div>
          </div>
        </div>
        {metrics.avg_wait_ms > 0 && (
          <div className="mt-3 border-t border-border/50 pt-3 text-xs text-muted-foreground">
            Avg. wait: <span className="font-mono">{(metrics.avg_wait_ms / 1000).toFixed(1)}s</span>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function CacheStatsCard({ stats, auditStats }: { stats: CacheStats; auditStats: AuditStats | null }) {
  const tc = stats.tool_call
  const tcReqs = tc.hits + tc.misses
  const cacheRate = tcReqs > 0 ? ((tc.hits / tcReqs) * 100).toFixed(1) : '0.0'

  const allowed = auditStats ? auditStats.success_count : 0
  const blocked = auditStats ? auditStats.blocked_count : 0
  const errors = auditStats ? auditStats.error_count : 0

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle className="text-sm font-medium uppercase tracking-wider text-muted-foreground">
            Cache & Routing
          </CardTitle>
          <span className="text-xs font-mono text-muted-foreground">{cacheRate}% cache hit rate</span>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-2">
          <div className="flex items-center gap-2 text-xs text-muted-foreground">
            <Database className="h-3 w-3" />
            <span className="uppercase tracking-wider">Tool Call Cache</span>
          </div>
          <div className="grid grid-cols-4 gap-2 text-xs">
            <div>
              <div className="text-muted-foreground">Hits</div>
              <div className="font-mono font-medium text-chart-2">{tc.hits}</div>
            </div>
            <div>
              <div className="text-muted-foreground">Misses</div>
              <div className="font-mono font-medium">{tc.misses}</div>
            </div>
            <div>
              <div className="text-muted-foreground">Entries</div>
              <div className="font-mono font-medium">{tc.entries}</div>
            </div>
            <div>
              <div className="text-muted-foreground">Hit %</div>
              <div className="font-mono font-medium text-chart-2">
                {tc.hit_rate > 0 ? `${(tc.hit_rate * 100).toFixed(0)}%` : '-'}
              </div>
            </div>
          </div>
        </div>
        <div className="space-y-2 border-t border-border/50 pt-3">
          <div className="flex items-center gap-2 text-xs text-muted-foreground">
            <Activity className="h-3 w-3" />
            <span className="uppercase tracking-wider">Route Decisions</span>
          </div>
          <div className="grid grid-cols-3 gap-2 text-xs">
            <div>
              <div className="text-muted-foreground">Allowed</div>
              <div className="font-mono font-medium text-chart-2">{allowed}</div>
            </div>
            <div>
              <div className="text-muted-foreground">Blocked</div>
              <div className="font-mono font-medium text-amber-400">{blocked}</div>
            </div>
            <div>
              <div className="text-muted-foreground">Errors</div>
              <div className="font-mono font-medium text-destructive">{errors}</div>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

export function DashboardPage() {
  const [selected, setSelected] = useState<AuditRecord | null>(null)
  const [range, setRange] = useState<TimeRange>('1h')

  const fetcher = useCallback(() => getDashboard(range), [range])
  const { data, loading, error, refetch } = useApi(fetcher)

  const workspacesFetcher = useCallback(() => listWorkspaces(), [])
  const { data: workspaces } = useApi(workspacesFetcher)

  const authScopesFetcher = useCallback(() => listAuthScopes(), [])
  const { data: authScopes } = useApi(authScopesFetcher)

  const wsName = (id: string) => workspaces?.find((w) => w.id === id)?.name ?? id
  const asName = (id: string) => authScopes?.find((a) => a.id === id)?.name ?? id

  const { records: liveRecords, connected } = useAuditStream({})

  const recentCalls = useMemo(() => {
    const dbRecords = data?.recent_calls ?? []
    const seen = new Set(liveRecords.map((r) => r.id))
    const merged = [...liveRecords, ...dbRecords.filter((r) => !seen.has(r.id))]
    merged.sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime())
    return merged.slice(0, 50)
  }, [liveRecords, data?.recent_calls])

  const recentErrors = useMemo(() => {
    const dbErrors = data?.recent_errors ?? []
    const seen = new Set(liveRecords.map((r) => r.id))
    const liveErrors = liveRecords.filter((r) => r.status === 'error' || r.status === 'blocked')
    const merged = [...liveErrors, ...dbErrors.filter((r) => !seen.has(r.id))]
    merged.sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime())
    return merged.slice(0, 20)
  }, [liveRecords, data?.recent_errors])

  const { pending: pendingApprovals } = useApprovalStream()

  useInterval(refetch, 10000)

  if (loading && !data) {
    return (
      <div className="flex items-center gap-2 text-muted-foreground">
        <div className="h-2 w-2 animate-pulse rounded-full bg-primary" />
        Loading dashboard...
      </div>
    )
  }

  if (error) {
    return <div className="text-destructive">Error: {error}</div>
  }

  if (!data) return null

  const ts = data.timeseries ?? []
  const sessionsData = prepareChartData(ts, (p) => p.sessions)
  const serversData = prepareChartData(ts, (p) => p.servers)
  const errorRateData = prepareChartData(ts, (p) =>
    p.total > 0 ? Math.round((p.errors / p.total) * 100) : 0,
  )
  const latencyData = prepareChartData(ts, (p) => Math.round(p.avg_latency_ms))

  const activeServers = (data.active_downstreams ?? []).filter(
    (d) => d.state !== 'stopped',
  ).length
  const totalServers = (data.active_downstreams ?? []).length

  const pureErrorCount = data.stats?.error_count ?? 0
  const blockedCount = data.stats?.blocked_count ?? 0

  const errorRate =
    data.stats && data.stats.total_requests > 0
      ? `${((pureErrorCount / data.stats.total_requests) * 100).toFixed(1)}%`
      : '0%'

  const blockedRate =
    data.stats && data.stats.total_requests > 0
      ? `${((blockedCount / data.stats.total_requests) * 100).toFixed(1)}%`
      : '0%'

  const avgLatency = data.stats ? `${Math.round(data.stats.avg_latency_ms)}ms` : '0ms'

  return (
    <div className="space-y-6">
      {/* Header + time range toggle */}
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Dashboard</h1>
        <ToggleGroup
          type="single"
          value={range}
          onValueChange={(v) => { if (v) setRange(v as TimeRange) }}
          variant="outline"
          size="sm"
        >
          <ToggleGroupItem value="1h" className="font-mono text-xs">1h</ToggleGroupItem>
          <ToggleGroupItem value="6h" className="font-mono text-xs">6h</ToggleGroupItem>
          <ToggleGroupItem value="24h" className="font-mono text-xs">24h</ToggleGroupItem>
          <ToggleGroupItem value="7d" className="font-mono text-xs">7d</ToggleGroupItem>
        </ToggleGroup>
      </div>

      {/* Pending approvals banner */}
      {pendingApprovals.length > 0 && (
        <Link
          to="/approvals"
          className="flex items-center gap-3 rounded-lg border border-amber-500/30 bg-amber-500/5 px-4 py-3 transition-colors hover:bg-amber-500/10"
        >
          <span className="relative flex h-2.5 w-2.5 shrink-0">
            <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-amber-400 opacity-75" />
            <span className="relative inline-flex h-2.5 w-2.5 rounded-full bg-amber-500" />
          </span>
          <ShieldCheck className="h-4 w-4 text-amber-400" />
          <span className="text-sm font-medium text-amber-200">
            {pendingApprovals.length} pending approval{pendingApprovals.length !== 1 ? 's' : ''}
          </span>
          <span className="ml-auto text-xs text-amber-400/60">View &rarr;</span>
        </Link>
      )}

      {/* Metric cards — 4 columns */}
      <div className="grid gap-4 sm:grid-cols-2 md:grid-cols-4">
        <MetricChart
          label="sessions"
          value={data.active_sessions}
          icon={<Activity className="h-3.5 w-3.5" />}
          data={sessionsData}
          colorKey="primary"
          range={range}
        />
        <MetricChart
          label="servers"
          value={`${activeServers} / ${totalServers}`}
          icon={<Server className="h-3.5 w-3.5" />}
          data={serversData}
          colorKey="green"
          range={range}
        />
        <MetricChart
          label="Errors / Blocked"
          value={
            <span className="flex items-baseline gap-3">
              <span>{errorRate}</span>
              {blockedCount > 0 && (
                <span className="text-lg text-amber-400">{blockedRate}</span>
              )}
            </span>
          }
          subtitle={
            <div className="flex gap-3 text-xs text-muted-foreground">
              <span>{pureErrorCount} errors</span>
              {blockedCount > 0 && <span className="text-amber-400/80">{blockedCount} blocked</span>}
            </div>
          }
          icon={<AlertTriangle className="h-3.5 w-3.5" />}
          data={errorRateData}
          colorKey="red"
          suffix="%"
          range={range}
        />
        <MetricChart
          label="Avg Latency"
          value={avgLatency}
          subtitle={
            data.stats && data.stats.p95_latency_ms > 0 ? (
              <div className="text-xs text-muted-foreground">
                P95: <span className="font-mono">{data.stats.p95_latency_ms}ms</span>
              </div>
            ) : undefined
          }
          icon={<Clock className="h-3.5 w-3.5" />}
          data={latencyData}
          colorKey="amber"
          suffix="ms"
          range={range}
        />
      </div>

      {/* Active Sessions + Cache Stats */}
      <div className="grid gap-4 lg:grid-cols-2">
        <ActiveSessionsTable
          sessions={(data.active_session_list ?? []) as SessionInfo[]}
          wsName={wsName}
        />
        <CacheStatsCard
          stats={data.cache_stats ?? {
            tool_call: { hits: 0, misses: 0, evictions: 0, entries: 0, hit_rate: 0 },
            route_resolution: { hits: 0, misses: 0, evictions: 0, entries: 0, hit_rate: 0 },
          }}
          auditStats={data.stats}
        />
      </div>

      {/* Recent Calls table */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle className="text-sm font-medium uppercase tracking-wider text-muted-foreground">
              Recent Calls
            </CardTitle>
            <div className="flex items-center gap-2 text-xs">
              {connected ? (
                <>
                  <span className="relative flex h-2 w-2">
                    <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-75" />
                    <span className="relative inline-flex h-2 w-2 rounded-full bg-emerald-500" />
                  </span>
                  <span className="text-emerald-400">Live</span>
                </>
              ) : (
                <span className="text-muted-foreground">Connecting...</span>
              )}
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {recentCalls.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
              <div className="w-full max-w-xs rounded-lg border border-border/30 bg-muted/30 font-mono text-sm">
                <div className="flex items-center gap-1.5 border-b border-border/20 px-3 py-1.5">
                  <span className="h-2 w-2 rounded-full bg-muted-foreground/20" />
                  <span className="h-2 w-2 rounded-full bg-muted-foreground/20" />
                  <span className="h-2 w-2 rounded-full bg-muted-foreground/20" />
                  <span className="ml-2 text-[10px] text-muted-foreground/40">mcplexer</span>
                </div>
                <div className="space-y-1 px-3 py-3">
                  <p className="text-muted-foreground/40">$ mcplexer serve --mode=stdio</p>
                  <p className="text-muted-foreground/50">listening for tool calls...</p>
                  <p>
                    <span className="text-primary">$</span>
                    <span className="ml-1 inline-block h-3.5 w-1.5 translate-y-[1px] animate-pulse bg-primary/70" />
                  </p>
                </div>
              </div>
              <p className="mt-4 text-xs text-muted-foreground/60">
                Tool calls will appear here once sessions are active
              </p>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow className="border-border/50 hover:bg-transparent">
                  <TableHead>Time</TableHead>
                  <TableHead>Tool</TableHead>
                  <TableHead className="hidden md:table-cell">Workspace</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Reason</TableHead>
                  <TableHead className="hidden sm:table-cell text-right">Latency</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {recentCalls.map((call, idx) => (
                  <TableRow
                    key={call.id}
                    className={`cursor-pointer border-border/30 hover:bg-muted/30 ${
                      idx === 0 ? 'animate-[audit-in_0.3s_ease-out]' : ''
                    }`}
                    onClick={() => setSelected(call)}
                  >
                    <TableCell className="whitespace-nowrap font-mono text-xs text-muted-foreground">
                      {formatTime(call.timestamp)}
                    </TableCell>
                    <TableCell>
                      <div className="max-w-[14rem] truncate font-mono text-sm text-accent-foreground">
                        {call.tool_name}
                      </div>
                    </TableCell>
                    <TableCell className="hidden md:table-cell text-muted-foreground">
                      {call.workspace_name || (call.workspace_id ? wsName(call.workspace_id) : '-')}
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant={call.status === 'success' ? 'secondary' : call.status === 'blocked' ? 'outline' : 'destructive'}
                        className={call.status === 'blocked' ? 'border-amber-500/40 text-amber-500' : ''}
                      >
                        {call.status}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <ReasonBadge record={call} />
                    </TableCell>
                    <TableCell className="hidden sm:table-cell text-right font-mono text-sm text-muted-foreground">
                      {call.latency_ms}ms
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {/* Recent Errors & Blocked table */}
      {recentErrors.length > 0 && (
        <Card className="border-destructive/30">
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle className="text-sm font-medium uppercase tracking-wider text-destructive">
                Recent Errors & Blocked
              </CardTitle>
              <div className="flex gap-3 text-xs text-muted-foreground">
                <span className="flex items-center gap-1.5">
                  <span className="h-2 w-2 rounded-full bg-destructive" />
                  {recentErrors.filter((e) => !isBlocked(e)).length} errors
                </span>
                <span className="flex items-center gap-1.5">
                  <span className="h-2 w-2 rounded-full bg-amber-500" />
                  {recentErrors.filter((e) => isBlocked(e)).length} blocked
                </span>
              </div>
            </div>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow className="border-border/50 hover:bg-transparent">
                  <TableHead>Time</TableHead>
                  <TableHead>Tool</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Reason</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {recentErrors.map((err) => {
                  const blocked = isBlocked(err)
                  return (
                    <TableRow
                      key={err.id}
                      className={`cursor-pointer border-border/30 ${
                        blocked ? 'hover:bg-amber-500/5' : 'hover:bg-destructive/5'
                      }`}
                      onClick={() => setSelected(err)}
                    >
                      <TableCell className="whitespace-nowrap font-mono text-xs text-muted-foreground">
                        {formatTime(err.timestamp)}
                      </TableCell>
                      <TableCell>
                        <div className="max-w-[14rem] truncate font-mono text-sm text-accent-foreground">
                          {err.tool_name}
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge
                          variant="outline"
                          className={blocked
                            ? 'border-amber-500/40 text-amber-500'
                            : 'border-destructive/40 text-destructive'
                          }
                        >
                          {blocked ? 'blocked' : 'error'}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <ReasonBadge record={err} />
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}

      {/* Tool Leaderboard + Error Breakdown */}
      <div className={`grid gap-4 ${(data.error_breakdown ?? []).length > 0 ? 'lg:grid-cols-2' : ''}`}>
        <ToolLeaderboardTable entries={data.tool_leaderboard ?? []} />
        {(data.error_breakdown ?? []).length > 0 && (
          <ErrorBreakdownCard entries={data.error_breakdown ?? []} />
        )}
      </div>

      {/* Server Health + Route Hits side by side */}
      <div className="grid gap-4 lg:grid-cols-2">
        <ServerHealthCards
          entries={data.server_health ?? []}
          downstreams={data.active_downstreams ?? []}
        />
        <div className="space-y-4">
          <RouteHitMapTable entries={data.route_hit_map ?? []} />
          {data.approval_metrics && (
            data.approval_metrics.pending_count + data.approval_metrics.approved_count +
            data.approval_metrics.denied_count + data.approval_metrics.timed_out_count
          ) > 0 && <ApprovalMetricsCard metrics={data.approval_metrics!} />}
        </div>
      </div>

      <AuditDetailDialog
        record={selected}
        onClose={() => setSelected(null)}
        wsName={wsName}
        asName={asName}
      />
    </div>
  )
}
