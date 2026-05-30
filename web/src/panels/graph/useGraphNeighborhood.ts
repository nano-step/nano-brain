import { useMutation } from '@tanstack/react-query'
import { apiFetch } from '../../api/client'
import type {
  GraphNeighborhoodRequest,
  GraphNeighborhoodResponse,
} from '../../api/types'

async function fetchNeighborhood(req: GraphNeighborhoodRequest): Promise<GraphNeighborhoodResponse> {
  const r = await apiFetch('/api/v1/graph/neighborhood', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!r.ok) throw new Error(`${r.status} ${r.statusText}`)
  return r.json() as Promise<GraphNeighborhoodResponse>
}

export interface UseGraphNeighborhoodOptions {
  workspace: string | null
}

export function useGraphNeighborhood({ workspace }: UseGraphNeighborhoodOptions) {
  return useMutation<GraphNeighborhoodResponse, Error, Omit<GraphNeighborhoodRequest, 'workspace'>>({
    mutationFn: (params) => {
      if (!workspace) return Promise.reject(new Error('No workspace selected'))
      return fetchNeighborhood({ ...params, workspace })
    },
  })
}
