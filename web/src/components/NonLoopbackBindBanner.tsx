import { useQuery } from '@tanstack/react-query'
import { apiGetJSON } from '../api/client'
import type { ConfigResponse } from '../api/types'

const LOOPBACK = new Set(['localhost', '127.0.0.1', '::1'])

function isLoopback(host: string): boolean {
  return LOOPBACK.has(host)
}

export function NonLoopbackBindBanner() {
  const { data } = useQuery({
    queryKey: ['config'],
    queryFn: () => apiGetJSON<ConfigResponse>('/api/v1/config'),
    staleTime: 300_000,
  })

  if (!data) return null

  const host = data.config?.server?.host ?? 'localhost'
  const port = data.config?.server?.port ?? 3100
  const authEnabled = data.config?.server?.auth?.enabled ?? false

  if (isLoopback(host) || authEnabled) return null

  return (
    <div className="banner-danger" role="alert" aria-live="polite">
      {`\u26a0 Server bound to ${host}:${port} without authentication. Anyone on the network can read and modify your memory. Bind to loopback or configure auth.`}
    </div>
  )
}
