import { useCallback, useEffect, useState } from 'react'

interface UseApiState<T> {
  data: T | null
  loading: boolean
  error: string | null
}

interface UseApiReturn<T> extends UseApiState<T> {
  refetch: () => void
}

export function useApi<T>(fetcher: () => Promise<T>): UseApiReturn<T> {
  const [state, setState] = useState<UseApiState<T>>({
    data: null,
    loading: true,
    error: null,
  })
  const [trigger, setTrigger] = useState(0)

  const refetch = useCallback(() => {
    setTrigger((t) => t + 1)
  }, [])

  useEffect(() => {
    let active = true

    // Set loading state for subsequent fetches (not the initial one which is already loading: true)
    if (trigger > 0) {
      setState((prev) => ({ ...prev, loading: true, error: null }))
    }
    fetcher()
      .then((data) => {
        if (active) {
          setState({ data, loading: false, error: null })
        }
      })
      .catch((err: unknown) => {
        if (active) {
          const message =
            err instanceof Error ? err.message : 'Unknown error'
          setState((prev) => ({ data: prev.data, loading: false, error: message }))
        }
      })

    return () => {
      active = false
    }
  }, [fetcher, trigger])

  return { ...state, refetch }
}
