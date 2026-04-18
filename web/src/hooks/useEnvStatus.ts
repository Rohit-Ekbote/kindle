import { useState, useCallback, useRef, useEffect } from 'react'
import { envs } from '../api/envs'
import type { EnvStatus } from '../types/api'

export function useEnvStatus(envName: string) {
  const [status, setStatus] = useState<EnvStatus | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const mountedRef = useRef(true)

  useEffect(() => {
    mountedRef.current = true
    return () => { mountedRef.current = false }
  }, [])

  const refresh = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const s = await envs.status(envName)
      if (mountedRef.current) setStatus(s)
    } catch (e) {
      if (mountedRef.current) setError(e instanceof Error ? e.message : 'Failed to fetch status')
    } finally {
      if (mountedRef.current) setLoading(false)
    }
  }, [envName])

  return { status, loading, error, refresh }
}
