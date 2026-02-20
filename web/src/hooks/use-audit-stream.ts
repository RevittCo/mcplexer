import { useCallback, useEffect, useRef, useState } from 'react'
import type { AuditRecord } from '@/api/types'
import { getBackoffDelay } from '@/lib/sse-backoff'

interface AuditStreamFilter {
  workspace_id?: string
  tool_name?: string
  status?: string
}

const MAX_RECORDS = 200

export function useAuditStream(filter: AuditStreamFilter) {
  const [records, setRecords] = useState<AuditRecord[]>([])
  const [connected, setConnected] = useState(false)
  const retryRef = useRef(0)
  const esRef = useRef<EventSource | null>(null)

  const clear = useCallback(() => setRecords([]), [])

  useEffect(() => {
    let cancelled = false
    let retryTimeout: ReturnType<typeof setTimeout>

    function connect() {
      if (cancelled) return

      const params = new URLSearchParams()
      if (filter.workspace_id) params.set('workspace_id', filter.workspace_id)
      if (filter.tool_name) params.set('tool_name', filter.tool_name)
      if (filter.status) params.set('status', filter.status)

      const qs = params.toString()
      const apiBase = import.meta.env.VITE_API_BASE_URL?.replace(/\/api\/v1$/, '') || ''
      const url = `${apiBase}/api/v1/audit/stream${qs ? `?${qs}` : ''}`

      const es = new EventSource(url)
      esRef.current = es

      es.onopen = () => {
        if (cancelled) return
        setConnected(true)
        retryRef.current = 0
      }

      es.onmessage = (event) => {
        if (cancelled) return
        try {
          const record = JSON.parse(event.data) as AuditRecord
          setRecords((prev) => [record, ...prev].slice(0, MAX_RECORDS))
        } catch {
          // skip malformed events
        }
      }

      es.onerror = () => {
        if (cancelled) return
        es.close()
        esRef.current = null
        setConnected(false)

        const delay = getBackoffDelay(retryRef.current)
        retryRef.current++
        retryTimeout = setTimeout(connect, delay)
      }
    }

    connect()

    return () => {
      cancelled = true
      clearTimeout(retryTimeout)
      esRef.current?.close()
      esRef.current = null
      setConnected(false)
    }
  }, [filter.workspace_id, filter.tool_name, filter.status])

  return { records, connected, clear }
}
