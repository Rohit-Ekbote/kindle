import { useState, useCallback } from 'react'
import { envs } from '../api/envs'
import type { EnvStatus } from '../types/api'

export function useEnvStatus(envName: string) {
  const [status, setStatus] = useState<EnvStatus | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const s = await envs.status(envName)
      setStatus(s)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to fetch status')
    } finally {
      setLoading(false)
    }
  }, [envName])

  return { status, loading, error, refresh }
}
