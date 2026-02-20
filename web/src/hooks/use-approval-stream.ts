import { useEffect, useRef, useState } from 'react'
import type { ApprovalEvent, ToolApproval } from '@/api/types'
import { getBackoffDelay } from '@/lib/sse-backoff'

export function useApprovalStream() {
  const [pending, setPending] = useState<ToolApproval[]>([])
  const [connected, setConnected] = useState(false)
  const retryRef = useRef(0)
  const esRef = useRef<EventSource | null>(null)

  useEffect(() => {
    let cancelled = false
    let retryTimeout: ReturnType<typeof setTimeout>

    function connect() {
      if (cancelled) return

      const apiBase = import.meta.env.VITE_API_BASE_URL?.replace(/\/api\/v1$/, '') || ''
      const es = new EventSource(`${apiBase}/api/v1/approvals/stream`)
      esRef.current = es

      es.onopen = () => {
        if (cancelled) return
        setConnected(true)
        retryRef.current = 0
      }

      es.onmessage = (event) => {
        if (cancelled) return
        try {
          const evt = JSON.parse(event.data) as ApprovalEvent
          if (evt.type === 'pending') {
            setPending((prev) => [...prev, evt.approval])
          } else if (evt.type === 'resolved') {
            setPending((prev) => prev.filter((a) => a.id !== evt.approval.id))
          }
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
  }, [])

  return { pending, connected }
}
