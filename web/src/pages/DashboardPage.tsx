import { useCallback } from 'react'
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
import { useApi } from '@/hooks/use-api'
import { useInterval } from '@/hooks/use-interval'
import { getDashboard } from '@/api/client'
import type { AuditRecord, TimeSeriesPoint } from '@/api/types'
import { Activity, AlertTriangle, Server, Terminal } from 'lucide-react'
import {
  Area,
  AreaChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
} from 'recharts'

function StatusBadge({ status }: { status: AuditRecord['status'] }) {
  return (
    <Badge
      variant={status === 'success' ? 'secondary' : 'destructive'}
      className=""
    >
      {status}
    </Badge>
  )
}

function formatTime(ts: string): string {
  return new Date(ts).toLocaleTimeString()
}

function formatHHMM(ts: string): string {
  const d = new Date(ts)
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
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
}: {
  active?: boolean
  payload?: { value: number }[]
  label?: string
  suffix?: string
}) {
  if (!active || !payload?.length || !label) return null
  return (
    <div className="border border-border bg-card px-3 py-1.5 font-mono text-xs shadow-lg">
      <div className="text-muted-foreground">{formatHHMM(label)}</div>
      <div className="text-foreground">
        {payload[0].value}
        {suffix}
      </div>
    </div>
  )
}

const chartColors = {
  primary: { stroke: 'hsl(205 95% 55%)', fill: 'hsl(205 95% 55%)' },
  green: { stroke: 'hsl(160 70% 45%)', fill: 'hsl(160 70% 45%)' },
  red: { stroke: 'hsl(0 72% 51%)', fill: 'hsl(0 72% 51%)' },
} as const

function MetricChart({
  label,
  value,
  icon,
  data,
  colorKey,
  suffix,
}: {
  label: string
  value: React.ReactNode
  icon: React.ReactNode
  data: ChartPoint[]
  colorKey: keyof typeof chartColors
  suffix?: string
}) {
  const color = chartColors[colorKey]
  const gradientId = `gradient-${colorKey}`

  return (
    <div className="overflow-hidden rounded-lg border border-border/50 bg-card">
      <div className="p-5">
        <div className="flex items-center gap-2 text-muted-foreground">
          <span className={colorKey === 'primary' ? 'text-primary' : colorKey === 'green' ? 'text-chart-2' : 'text-chart-5'}>
            {icon}
          </span>
          <span className="text-[11px] uppercase tracking-widest">{label}</span>
        </div>
        <div className={`mt-3 text-3xl font-bold tracking-tight md:text-4xl ${colorKey === 'primary' ? 'text-primary' : colorKey === 'green' ? 'text-chart-2' : 'text-chart-5'}`}>
          {value}
        </div>
      </div>
      <div className="h-24 w-full">
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={data} margin={{ top: 0, right: 0, left: 0, bottom: 0 }}>
            <defs>
              <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor={color.fill} stopOpacity={0.2} />
                <stop offset="100%" stopColor={color.fill} stopOpacity={0} />
              </linearGradient>
            </defs>
            <XAxis
              dataKey="time"
              tickFormatter={formatHHMM}
              tick={{ fontSize: 9, fontFamily: 'monospace', fill: 'hsl(var(--muted-foreground))' }}
              axisLine={false}
              tickLine={false}
              interval="preserveStartEnd"
              minTickGap={40}
            />
            <Tooltip
              content={<ChartTooltip suffix={suffix} />}
              cursor={{ stroke: 'hsl(var(--border))', strokeDasharray: '3 3' }}
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

export function DashboardPage() {
  const fetcher = useCallback(() => getDashboard(), [])
  const { data, loading, error, refetch } = useApi(fetcher)

  useInterval(refetch, 5000)

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

  const activeServers = (data.active_downstreams ?? []).filter(
    (d) => d.state !== 'stopped',
  ).length
  const totalServers = (data.active_downstreams ?? []).length

  const errorRate =
    data.stats && data.stats.total_requests > 0
      ? `${((data.stats.error_count / data.stats.total_requests) * 100).toFixed(1)}%`
      : '0%'

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Dashboard</h1>

      <div className="grid gap-4 sm:grid-cols-2 md:grid-cols-3">
        <MetricChart
          label="sessions"
          value={data.active_sessions}
          icon={<Activity className="h-3.5 w-3.5" />}
          data={sessionsData}
          colorKey="primary"
        />
        <MetricChart
          label="servers"
          value={`${activeServers} / ${totalServers}`}
          icon={<Server className="h-3.5 w-3.5" />}
          data={serversData}
          colorKey="green"
        />
        <MetricChart
          label="error rate"
          value={errorRate}
          icon={<AlertTriangle className="h-3.5 w-3.5" />}
          data={errorRateData}
          colorKey="red"
          suffix="%"
        />
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-medium uppercase tracking-wider text-muted-foreground">
            Recent Calls
          </CardTitle>
        </CardHeader>
        <CardContent>
          {(data.recent_calls ?? []).length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
              <Terminal className="mb-3 h-8 w-8 text-muted-foreground/50" />
              <p className="font-mono text-sm">
                <span className="text-primary">&gt;</span> no recent activity
                <span className="ml-0.5 inline-block w-1.5 animate-pulse bg-primary/70">
                  &nbsp;
                </span>
              </p>
              <p className="mt-2 text-xs text-muted-foreground/60">
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
                  <TableHead className="hidden sm:table-cell text-right">Latency</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(data.recent_calls ?? []).map((call) => (
                  <TableRow key={call.id} className="border-border/30 hover:bg-muted/30">
                    <TableCell className="whitespace-nowrap font-mono text-xs text-muted-foreground">
                      {formatTime(call.timestamp)}
                    </TableCell>
                    <TableCell>
                      <div className="max-w-[14rem] truncate font-mono text-sm text-accent-foreground">
                        {call.tool_name}
                      </div>
                    </TableCell>
                    <TableCell className="hidden md:table-cell text-muted-foreground">
                      {call.workspace_id || '-'}
                    </TableCell>
                    <TableCell>
                      <StatusBadge status={call.status} />
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

      {(data.recent_errors ?? []).length > 0 && (
        <Card className="border-destructive/30">
          <CardHeader>
            <CardTitle className="text-sm font-medium uppercase tracking-wider text-destructive">
              Recent Errors
            </CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow className="border-border/50 hover:bg-transparent">
                  <TableHead>Time</TableHead>
                  <TableHead>Tool</TableHead>
                  <TableHead>Error</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(data.recent_errors ?? []).map((err) => (
                  <TableRow key={err.id} className="border-border/30 hover:bg-destructive/5">
                    <TableCell className="whitespace-nowrap font-mono text-xs text-muted-foreground">
                      {formatTime(err.timestamp)}
                    </TableCell>
                    <TableCell>
                      <div className="max-w-[14rem] truncate font-mono text-sm text-accent-foreground">
                        {err.tool_name}
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="max-w-[20rem] truncate text-destructive">
                        {err.error_message}
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}
    </div>
  )
}
