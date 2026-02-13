import { useCallback, useState } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { useApprovalStream } from '@/hooks/use-approval-stream'
import { useApi } from '@/hooks/use-api'
import { listApprovals, resolveApproval } from '@/api/client'
import type { ToolApproval } from '@/api/types'
import { Check, ChevronDown, ChevronRight, Clock, ShieldCheck, X } from 'lucide-react'
import { toast } from 'sonner'

function formatTime(ts: string): string {
  return new Date(ts).toLocaleTimeString()
}

function formatTimeRemaining(createdAt: string, timeoutSec: number): string {
  const elapsed = (Date.now() - new Date(createdAt).getTime()) / 1000
  const remaining = Math.max(0, timeoutSec - elapsed)
  if (remaining <= 0) return 'expired'
  const mins = Math.floor(remaining / 60)
  const secs = Math.floor(remaining % 60)
  return mins > 0 ? `${mins}m ${secs}s` : `${secs}s`
}

function statusBadge(status: string) {
  switch (status) {
    case 'approved':
      return <Badge className="bg-emerald-500/10 text-emerald-400 border-emerald-500/30">approved</Badge>
    case 'denied':
      return <Badge variant="destructive">denied</Badge>
    case 'timeout':
      return <Badge variant="outline" className="text-muted-foreground">timeout</Badge>
    case 'cancelled':
      return <Badge variant="outline" className="text-muted-foreground">cancelled</Badge>
    default:
      return <Badge variant="secondary">{status}</Badge>
  }
}

function PendingCard({
  approval,
  onResolved,
}: {
  approval: ToolApproval
  onResolved: () => void
}) {
  const [reason, setReason] = useState('')
  const [resolving, setResolving] = useState(false)
  const [expanded, setExpanded] = useState(false)

  async function handleResolve(approved: boolean) {
    if (!approved && !reason.trim()) {
      toast.error('A reason is required when denying')
      return
    }
    setResolving(true)
    try {
      await resolveApproval(approval.id, { approved, reason })
      toast.success(approved ? 'Approved' : 'Denied')
      onResolved()
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : 'Failed to resolve')
    } finally {
      setResolving(false)
    }
  }

  return (
    <Card className="border-primary/30">
      <CardContent className="pt-5 space-y-3">
        <div className="flex items-start justify-between gap-4">
          <div className="space-y-1 min-w-0">
            <div className="font-mono text-sm text-accent-foreground truncate">
              {approval.tool_name}
            </div>
            <div className="text-xs text-muted-foreground">
              Requested by {approval.request_client_type || 'unknown'}
              {approval.request_model ? ` (${approval.request_model})` : ''}
            </div>
          </div>
          <div className="flex items-center gap-1.5 text-xs text-muted-foreground shrink-0">
            <Clock className="h-3 w-3" />
            {formatTimeRemaining(approval.created_at, approval.timeout_sec)}
          </div>
        </div>

        <div className="rounded-md bg-muted/50 p-3">
          <div className="text-xs font-medium text-muted-foreground mb-1">Justification</div>
          <div className="text-sm">{approval.justification || 'No justification provided'}</div>
        </div>

        <button
          type="button"
          className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
          onClick={() => setExpanded(!expanded)}
        >
          {expanded ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
          Arguments
        </button>

        {expanded && (
          <pre className="rounded-md bg-muted/50 p-3 text-xs font-mono overflow-x-auto max-h-40">
            {(() => {
              try {
                return JSON.stringify(JSON.parse(approval.arguments), null, 2)
              } catch {
                return approval.arguments
              }
            })()}
          </pre>
        )}

        <div className="space-y-2">
          <Input
            placeholder="Reason (required for deny)"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            className="text-sm"
          />
          <div className="flex gap-2">
            <Button
              size="sm"
              onClick={() => handleResolve(true)}
              disabled={resolving}
              className="bg-emerald-600 hover:bg-emerald-700"
            >
              <Check className="mr-1 h-3.5 w-3.5" />
              Approve
            </Button>
            <Button
              size="sm"
              variant="destructive"
              onClick={() => handleResolve(false)}
              disabled={resolving}
            >
              <X className="mr-1 h-3.5 w-3.5" />
              Deny
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

export function ApprovalsPage() {
  const { pending, connected } = useApprovalStream()

  const historyFetcher = useCallback(() => listApprovals('pending'), [])
  const { data: dbPending, refetch } = useApi(historyFetcher)

  // Merge SSE pending with initial DB load, deduplicate by ID.
  const allPending = (() => {
    const seen = new Set<string>()
    const merged: ToolApproval[] = []
    for (const a of pending) {
      if (!seen.has(a.id)) {
        seen.add(a.id)
        merged.push(a)
      }
    }
    for (const a of dbPending ?? []) {
      if (!seen.has(a.id)) {
        seen.add(a.id)
        merged.push(a)
      }
    }
    return merged
  })()

  function handleResolved() {
    refetch()
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <h1 className="text-2xl font-bold">Approvals</h1>
          {connected ? (
            <div className="flex items-center gap-1.5 text-xs">
              <span className="relative flex h-2 w-2">
                <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-75" />
                <span className="relative inline-flex h-2 w-2 rounded-full bg-emerald-500" />
              </span>
              <span className="text-emerald-400">Live</span>
            </div>
          ) : (
            <span className="text-xs text-muted-foreground">Connecting...</span>
          )}
        </div>
      </div>

      {allPending.length > 0 ? (
        <div className="space-y-4">
          <h2 className="text-sm font-medium uppercase tracking-wider text-muted-foreground">
            Pending ({allPending.length})
          </h2>
          <div className="grid gap-4 md:grid-cols-2">
            {allPending.map((a) => (
              <PendingCard key={a.id} approval={a} onResolved={handleResolved} />
            ))}
          </div>
        </div>
      ) : (
        <Card>
          <CardContent className="pt-6">
            <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
              <ShieldCheck className="mb-3 h-8 w-8 text-muted-foreground/50" />
              <p className="text-sm">No pending approvals</p>
              <p className="mt-1 text-xs text-muted-foreground/60">
                Tool calls requiring approval will appear here
              </p>
            </div>
          </CardContent>
        </Card>
      )}

      <RecentHistory />
    </div>
  )
}

function RecentHistory() {
  const fetcher = useCallback(() => listApprovals(), [])
  const { data } = useApi(fetcher)

  const resolved = (data ?? []).filter((a) => a.status !== 'pending')

  if (resolved.length === 0) return null

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium uppercase tracking-wider text-muted-foreground">
          Recent History
        </CardTitle>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow className="border-border/50 hover:bg-transparent">
              <TableHead>Time</TableHead>
              <TableHead>Tool</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="hidden md:table-cell">Approver</TableHead>
              <TableHead className="hidden sm:table-cell">Resolution</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {resolved.map((a) => (
              <TableRow key={a.id} className="border-border/30 hover:bg-muted/30">
                <TableCell className="whitespace-nowrap font-mono text-xs text-muted-foreground">
                  {a.resolved_at ? formatTime(a.resolved_at) : '-'}
                </TableCell>
                <TableCell>
                  <div className="max-w-[14rem] truncate font-mono text-sm text-accent-foreground">
                    {a.tool_name}
                  </div>
                </TableCell>
                <TableCell>{statusBadge(a.status)}</TableCell>
                <TableCell className="hidden md:table-cell text-muted-foreground text-sm">
                  {a.approver_type || '-'}
                </TableCell>
                <TableCell className="hidden sm:table-cell text-muted-foreground text-sm max-w-[12rem] truncate">
                  {a.resolution || '-'}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  )
}
