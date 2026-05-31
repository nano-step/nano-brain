import { useQuery } from '@tanstack/react-query'
import { apiGetJSON } from '../api/client'
import type { WorkspacesResponse } from '../api/types'

export function useWorkspaces() {
  return useQuery({
    queryKey: ['workspaces'],
    queryFn: () => apiGetJSON<WorkspacesResponse>('/api/v1/workspaces'),
    staleTime: 60_000,
  })
}
