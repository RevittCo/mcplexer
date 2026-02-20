import { useCallback, useMemo, useState } from 'react'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { useApi } from '@/hooks/use-api'
import { useAuditStream } from '@/hooks/use-audit-stream'
import { listAuthScopes, listWorkspaces, queryAuditLogs } from '@/api/client'
import type { AuditFilter, AuditRecord } from '@/api/types'
import { ChevronLeft, ChevronRight, Radio } from 'lucide-react'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { AuditDetailDialog, ReasonBadge } from '@/components/AuditDetailDialog'

const PAGE_SIZE = 25

export function AuditPage() {
  const [filter, setFilter] = useState<AuditFilter>({
    limit: PAGE_SIZE,
    offset: 0,
  })
  const [selected, setSelected] = useState<AuditRecord | null>(null)

  const workspacesFetcher = useCallback(() => listWorkspaces(), [])
  const { data: workspaces } = useApi(workspacesFetcher)

  const authScopesFetcher = useCallback(() => listAuthScopes(), [])
  const { data: authScopes } = useApi(authScopesFetcher)

  const wsName = (id: string) => workspaces?.find((w) => w.id === id)?.name ?? id
  const asName = (id: string) => authScopes?.find((a) => a.id === id)?.name ?? id

  // Live stream (always connected)
  const streamFilter = useMemo(
    () => ({
      workspace_id: filter.workspace_id,
      tool_name: filter.tool_name,
      status: filter.status,
    }),
    [filter.workspace_id, filter.tool_name, filter.status],
  )
  const { records: liveRecords, connected, clear } = useAuditStream(streamFilter)

  // History (paginated)
  const historyFetcher = useCallback(() => queryAuditLogs(filter), [filter])
  const { data: historyData, loading, error } = useApi(historyFetcher)

  const page = Math.floor((filter.offset ?? 0) / PAGE_SIZE) + 1
  const totalPages = historyData ? Math.ceil(historyData.total / PAGE_SIZE) : 0
  const isFirstPage = page === 1

  // On page 1, show live events (deduped) then history. Other pages: just history.
  const historyRecords = historyData?.data ?? []
  const historyIds = new Set(historyRecords.map((r) => r.id))
  const uniqueLive = isFirstPage ? liveRecords.filter((r) => !historyIds.has(r.id)) : []
  const allRecords = [...uniqueLive, ...historyRecords]

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Audit Logs</h1>
        <div className="flex items-center gap-3">
          {uniqueLive.length > 0 && (
            <Button variant="ghost" size="sm" onClick={clear}>
              Clear live
            </Button>
          )}
          <div className="flex items-center gap-2 text-sm">
            {connected ? (
              <>
                <span className="relative flex h-2.5 w-2.5">
                  <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-75" />
                  <span className="relative inline-flex h-2.5 w-2.5 rounded-full bg-emerald-500" />
                </span>
                <span className="text-emerald-400">Live</span>
              </>
            ) : (
              <>
                <span className="h-2.5 w-2.5 rounded-full bg-muted-foreground/40" />
                <span className="text-muted-foreground">Connecting...</span>
              </>
            )}
          </div>
        </div>
      </div>

      <FilterBar filter={filter} setFilter={setFilter} workspaces={workspaces ?? []} />

      <Card>
        <CardContent className="pt-6">
          {loading && !historyData && (
            <div className="flex items-center gap-2 text-muted-foreground">
              <div className="h-2 w-2 animate-pulse rounded-full bg-primary" />
              Loading...
            </div>
          )}
          {error && <p className="text-destructive">Error: {error}</p>}

          <Table>
            <TableHeader>
              <TableRow className="border-border/50 hover:bg-transparent">
                <TableHead>Timestamp</TableHead>
                <TableHead>Tool</TableHead>
                <TableHead className="hidden md:table-cell">Workspace</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Reason</TableHead>
                <TableHead className="hidden lg:table-cell">Cache</TableHead>
                <TableHead className="hidden sm:table-cell text-right">Latency</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {allRecords.length === 0 && !loading ? (
                <TableRow>
                  <TableCell colSpan={7} className="h-32">
                    <div className="flex flex-col items-center justify-center text-muted-foreground">
                      <Radio className="mb-2 h-8 w-8 text-muted-foreground/50" />
                      <p className="text-sm">Waiting for events...</p>
                      <p className="text-xs text-muted-foreground/60">
                        New audit records will appear here in real-time
                      </p>
                    </div>
                  </TableCell>
                </TableRow>
              ) : (
                allRecords.map((record, idx) => {
                  const isLive = idx < uniqueLive.length
                  return (
                    <TableRow
                      key={record.id}
                      className={`cursor-pointer border-border/30 hover:bg-muted/30 ${
                        isLive && idx === 0 ? 'animate-[audit-in_0.3s_ease-out]' : ''
                      }`}
                      onClick={() => setSelected(record)}
                    >
                      <TableCell className="whitespace-nowrap font-mono text-xs text-muted-foreground">
                        {new Date(record.timestamp).toLocaleString()}
                      </TableCell>
                      <TableCell>
                        <div className="max-w-[14rem] truncate font-mono text-sm text-accent-foreground">
                          {record.tool_name}
                        </div>
                      </TableCell>
                      <TableCell className="hidden md:table-cell text-muted-foreground">
                        {record.workspace_name || (record.workspace_id ? wsName(record.workspace_id) : '-')}
                      </TableCell>
                      <TableCell>
                        <Badge variant={record.status === 'success' ? 'secondary' : 'destructive'}>
                          {record.status}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <ReasonBadge record={record} />
                      </TableCell>
                      <TableCell className="hidden lg:table-cell">
                        {record.cache_hit && (
                          <Badge variant="outline" className="border-blue-500/40 text-blue-400">
                            cached
                          </Badge>
                        )}
                      </TableCell>
                      <TableCell className="hidden sm:table-cell text-right font-mono text-sm text-muted-foreground">
                        {record.latency_ms}ms
                      </TableCell>
                    </TableRow>
                  )
                })
              )}
            </TableBody>
          </Table>

          {historyData && (
            <div className="mt-4 flex items-center justify-between">
              <p className="text-sm text-muted-foreground">
                {historyData.total} total records
              </p>
              <div className="flex items-center gap-1">
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-8 w-8 p-0"
                      disabled={page <= 1}
                      onClick={() =>
                        setFilter((f) => ({
                          ...f,
                          offset: Math.max(0, (f.offset ?? 0) - PAGE_SIZE),
                        }))
                      }
                    >
                      <ChevronLeft className="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Previous page</TooltipContent>
                </Tooltip>
                <span className="rounded-md bg-secondary px-3 py-1 text-xs font-medium">
                  {page} / {Math.max(1, totalPages)}
                </span>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-8 w-8 p-0"
                      disabled={page >= totalPages}
                      onClick={() =>
                        setFilter((f) => ({
                          ...f,
                          offset: (f.offset ?? 0) + PAGE_SIZE,
                        }))
                      }
                    >
                      <ChevronRight className="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Next page</TooltipContent>
                </Tooltip>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      <AuditDetailDialog
        record={selected}
        onClose={() => setSelected(null)}
        wsName={wsName}
        asName={asName}
      />
    </div>
  )
}

function FilterBar({
  filter,
  setFilter,
  workspaces,
}: {
  filter: AuditFilter
  setFilter: React.Dispatch<React.SetStateAction<AuditFilter>>
  workspaces: { id: string; name: string }[]
}) {
  return (
    <Card>
      <CardContent className="pt-6">
        <div className="flex flex-wrap items-center gap-3">
          <Select
            value={filter.workspace_id ?? 'all'}
            onValueChange={(v) =>
              setFilter((f) => ({
                ...f,
                workspace_id: v === 'all' ? undefined : v,
                offset: 0,
              }))
            }
          >
            <SelectTrigger className="w-full sm:w-48">
              <SelectValue placeholder="All workspaces" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All workspaces</SelectItem>
              {workspaces.map((w) => (
                <SelectItem key={w.id} value={w.id}>
                  {w.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>

          <Input
            placeholder="Filter by tool name..."
            className="w-full sm:w-48"
            value={filter.tool_name ?? ''}
            onChange={(e) =>
              setFilter((f) => ({
                ...f,
                tool_name: e.target.value || undefined,
                offset: 0,
              }))
            }
          />

          <Select
            value={filter.status ?? 'all'}
            onValueChange={(v) =>
              setFilter((f) => ({
                ...f,
                status: v === 'all' ? undefined : (v as 'success' | 'error'),
                offset: 0,
              }))
            }
          >
            <SelectTrigger className="w-full sm:w-36">
              <SelectValue placeholder="All statuses" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All statuses</SelectItem>
              <SelectItem value="success">Success</SelectItem>
              <SelectItem value="error">Error</SelectItem>
            </SelectContent>
          </Select>

          <Input
            type="datetime-local"
            className="w-full sm:w-48"
            value={filter.after ?? ''}
            onChange={(e) =>
              setFilter((f) => ({
                ...f,
                after: e.target.value || undefined,
                offset: 0,
              }))
            }
            placeholder="After"
          />
          <Input
            type="datetime-local"
            className="w-full sm:w-48"
            value={filter.before ?? ''}
            onChange={(e) =>
              setFilter((f) => ({
                ...f,
                before: e.target.value || undefined,
                offset: 0,
              }))
            }
            placeholder="Before"
          />
        </div>
      </CardContent>
    </Card>
  )
}
