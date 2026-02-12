import { useCallback, useState } from 'react'
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
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { useApi } from '@/hooks/use-api'
import { listWorkspaces, queryAuditLogs } from '@/api/client'
import type { AuditFilter, AuditRecord } from '@/api/types'
import { ChevronLeft, ChevronRight, FileSearch } from 'lucide-react'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

const PAGE_SIZE = 25

export function AuditPage() {
  const [filter, setFilter] = useState<AuditFilter>({
    limit: PAGE_SIZE,
    offset: 0,
  })
  const [selected, setSelected] = useState<AuditRecord | null>(null)

  const fetcher = useCallback(() => queryAuditLogs(filter), [filter])
  const { data, loading, error } = useApi(fetcher)

  const workspacesFetcher = useCallback(() => listWorkspaces(), [])
  const { data: workspaces } = useApi(workspacesFetcher)

  const page = Math.floor((filter.offset ?? 0) / PAGE_SIZE) + 1
  const totalPages = data ? Math.ceil(data.total / PAGE_SIZE) : 0

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Audit Logs</h1>

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
                {workspaces?.map((w) => (
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
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardContent className="pt-6">
          {loading && !data && (
            <div className="flex items-center gap-2 text-muted-foreground">
              <div className="h-2 w-2 animate-pulse rounded-full bg-primary" />
              Loading...
            </div>
          )}
          {error && <p className="text-destructive">Error: {error}</p>}
          {data && (
            <>
              <Table>
                <TableHeader>
                  <TableRow className="border-border/50 hover:bg-transparent">
                    <TableHead>Timestamp</TableHead>
                    <TableHead>Tool</TableHead>
                    <TableHead className="hidden md:table-cell">Workspace</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead className="hidden sm:table-cell text-right">Latency</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {data.data.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={5} className="h-32">
                        <div className="flex flex-col items-center justify-center text-muted-foreground">
                          <FileSearch className="mb-2 h-8 w-8 text-muted-foreground/50" />
                          <p className="text-sm">No audit records found</p>
                          <p className="text-xs text-muted-foreground/60">
                            Try adjusting your filters
                          </p>
                        </div>
                      </TableCell>
                    </TableRow>
                  ) : (
                    data.data.map((record) => (
                      <TableRow
                        key={record.id}
                        className="cursor-pointer border-border/30 hover:bg-muted/30"
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
                          {record.workspace_id || '-'}
                        </TableCell>
                        <TableCell>
                          <Badge
                            variant={
                              record.status === 'success' ? 'secondary' : 'destructive'
                            }
                            className=""
                          >
                            {record.status}
                          </Badge>
                        </TableCell>
                        <TableCell className="hidden sm:table-cell text-right font-mono text-sm text-muted-foreground">
                          {record.latency_ms}ms
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>

              <div className="mt-4 flex items-center justify-between">
                <p className="text-sm text-muted-foreground">
                  {data.total} total records
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
            </>
          )}
        </CardContent>
      </Card>

      <AuditDetailDialog record={selected} onClose={() => setSelected(null)} />
    </div>
  )
}

function AuditDetailDialog({
  record,
  onClose,
}: {
  record: AuditRecord | null
  onClose: () => void
}) {
  if (!record) return null

  return (
    <Dialog open={!!record} onOpenChange={() => onClose()}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Audit Record Detail</DialogTitle>
        </DialogHeader>
        <div className="min-w-0 space-y-3 text-sm">
          <DetailRow label="ID" value={record.id} mono />
          <DetailRow label="Timestamp" value={new Date(record.timestamp).toLocaleString()} />
          <DetailRow label="Tool" value={record.tool_name} mono />
          <DetailRow label="Session" value={record.session_id} mono />
          <DetailRow label="Workspace" value={record.workspace_id || '-'} />
          <DetailRow label="Subpath" value={record.subpath || '-'} mono />
          <DetailRow label="Status" value={record.status} />
          {record.status === 'error' && (
            <>
              <DetailRow label="Error Code" value={record.error_code} mono />
              <DetailRow label="Error Message" value={record.error_message} />
            </>
          )}
          <DetailRow label="Latency" value={`${record.latency_ms}ms`} mono />
          <DetailRow label="Response Size" value={`${record.response_size} bytes`} mono />
          <DetailRow label="Route Rule" value={record.route_rule_id || '-'} mono />
          <DetailRow label="Downstream" value={record.downstream_server_id || '-'} mono />
          <DetailRow label="Auth Scope" value={record.auth_scope_id || '-'} mono />
          {Object.keys(record.params_redacted ?? {}).length > 0 && (
            <div className="pt-2">
              <span className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
                Redacted Params
              </span>
              <pre className="mt-2 max-h-64 overflow-auto rounded-md border border-border bg-background p-3 font-mono text-xs leading-relaxed text-accent-foreground">
                {JSON.stringify(record.params_redacted, null, 2)}
              </pre>
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}

function DetailRow({
  label,
  value,
  mono,
}: {
  label: string
  value: string
  mono?: boolean
}) {
  return (
    <div className="flex items-baseline justify-between gap-4 border-b border-border/30 pb-2">
      <span className="shrink-0 text-xs text-muted-foreground">{label}</span>
      <span
        className={`min-w-0 truncate text-right ${mono ? 'font-mono text-xs text-accent-foreground' : ''}`}
      >
        {value}
      </span>
    </div>
  )
}
