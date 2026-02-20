import { useCallback, useMemo, useState } from 'react'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { useApi } from '@/hooks/use-api'
import { useInterval } from '@/hooks/use-interval'
import { useAuditStream } from '@/hooks/use-audit-stream'
import { getDashboard, listAuthScopes, listWorkspaces } from '@/api/client'
import type { AuditRecord, SessionInfo } from '@/api/types'
import { Activity, AlertTriangle, Clock, Server, ShieldCheck } from 'lucide-react'
import { AuditDetailDialog } from '@/components/AuditDetailDialog'
import { useApprovalStream } from '@/hooks/use-approval-stream'
import { Link } from 'react-router-dom'

import { type TimeRange, prepareChartData, MetricChart } from './chart-components'
import { ErrorBreakdownCard, ApprovalMetricsCard, CacheStatsCard } from './stats-cards'
import { ToolLeaderboardTable } from './leaderboard-table'
import { ServerHealthCards } from './server-health'
import { ActiveSessionsTable } from './sessions-table'
import { RouteHitMapTable } from './route-hits-table'
import { RecentCallsTable, RecentErrorsTable } from './recent-tables'

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

      <RecentCallsTable
        recentCalls={recentCalls}
        connected={connected}
        wsName={wsName}
        onSelect={setSelected}
      />

      {recentErrors.length > 0 && (
        <RecentErrorsTable
          recentErrors={recentErrors}
          onSelect={setSelected}
        />
      )}

      <div className={`grid gap-4 ${(data.error_breakdown ?? []).length > 0 ? 'lg:grid-cols-2' : ''}`}>
        <ToolLeaderboardTable entries={data.tool_leaderboard ?? []} />
        {(data.error_breakdown ?? []).length > 0 && (
          <ErrorBreakdownCard entries={data.error_breakdown ?? []} />
        )}
      </div>

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
