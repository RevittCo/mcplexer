import { useEffect, useRef, useState } from 'react'
import type { ApprovalEvent, ToolApproval } from '@/api/types'

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

      const es = new EventSource('/api/v1/approvals/stream')
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

        const delay = Math.min(1000 * 2 ** retryRef.current, 30000)
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
