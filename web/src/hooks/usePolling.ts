import { useEffect, useRef } from 'react'

export function usePolling(fn: () => void, intervalMs: number, active: boolean) {
  const fnRef = useRef(fn)
  fnRef.current = fn

  useEffect(() => {
    if (!active) return
    const id = setInterval(() => fnRef.current(), intervalMs)
    return () => clearInterval(id)
  }, [active, intervalMs])
}
