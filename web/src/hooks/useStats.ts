import { useQuery } from '@tanstack/react-query'
import { apiGetJSON } from '../api/client'
import type { StatsResponse } from '../api/types'

export function useStats(workspace: string | null) {
  return useQuery({
    queryKey: ['stats', workspace],
    queryFn: () => apiGetJSON<StatsResponse>(`/api/v1/stats?workspace=${encodeURIComponent(workspace ?? '')}`),
    enabled: !!workspace,
    staleTime: 30_000,
  })
}
