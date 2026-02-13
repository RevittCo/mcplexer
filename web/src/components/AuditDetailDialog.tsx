import { Badge } from '@/components/ui/badge'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import type { AuditRecord } from '@/api/types'

export function getErrorReason(record: AuditRecord): string {
  if (record.status === 'success') return ''
  if (record.error_message?.includes('denied')) return 'blocked'
  if (record.error_message === 'no matching route') return 'no route'
  return record.error_message || record.error_code || 'error'
}

export function ReasonBadge({ record }: { record: AuditRecord }) {
  const reason = getErrorReason(record)
  if (!reason) return null
  return (
    <Badge
      variant="outline"
      className={
        reason === 'blocked'
          ? 'border-amber-500/40 text-amber-500'
          : 'text-muted-foreground'
      }
    >
      {reason}
    </Badge>
  )
}

export function AuditDetailDialog({
  record,
  onClose,
  wsName,
  asName,
}: {
  record: AuditRecord | null
  onClose: () => void
  wsName: (id: string) => string
  asName: (id: string) => string
}) {
  if (!record) return null

  const reason = getErrorReason(record)

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
          <DetailRow label="Workspace" value={record.workspace_id ? wsName(record.workspace_id) : '-'} />
          <DetailRow label="Subpath" value={record.subpath || '-'} mono />
          <DetailRow label="Status" value={record.status} />
          {record.status === 'error' && (
            <>
              <DetailRow label="Reason" value={reason} />
              <DetailRow label="Error Code" value={record.error_code} mono />
              <DetailRow label="Error Message" value={record.error_message} />
            </>
          )}
          <DetailRow label="Latency" value={`${record.latency_ms}ms`} mono />
          <DetailRow label="Response Size" value={`${record.response_size} bytes`} mono />
          <DetailRow
            label="Route Rule"
            value={record.route_rule_summary ?? record.route_rule_id ?? '-'}
            mono
            title={record.route_rule_id}
          />
          <DetailRow
            label="Downstream"
            value={record.downstream_server_name ?? record.downstream_server_id ?? '-'}
            mono
            title={record.downstream_server_id}
          />
          <DetailRow
            label="Auth Scope"
            value={record.auth_scope_id ? asName(record.auth_scope_id) : '-'}
            mono
          />
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
  title,
}: {
  label: string
  value: string
  mono?: boolean
  title?: string
}) {
  return (
    <div className="flex items-baseline justify-between gap-4 border-b border-border/30 pb-2">
      <span className="shrink-0 text-xs text-muted-foreground">{label}</span>
      <span
        title={title}
        className={`min-w-0 truncate text-right ${mono ? 'font-mono text-xs text-accent-foreground' : ''}`}
      >
        {value}
      </span>
    </div>
  )
}
