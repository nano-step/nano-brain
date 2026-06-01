import { useMutation } from '@tanstack/react-query'
import { apiFetch } from '../../api/client'
import type {
  GraphNeighborhoodResponse,
  EdgeType,
  NodeKind,
} from '../../api/types'

export interface GraphOverviewRequest {
  workspace: string
  mode: 'code' | 'knowledge'
  limit?: number
  edge_types?: EdgeType[]
}

async function fetchOverview(req: GraphOverviewRequest): Promise<GraphNeighborhoodResponse> {
  const r = await apiFetch('/api/v1/graph/overview', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!r.ok) throw new Error(`${r.status} ${r.statusText}`)
  return r.json() as Promise<GraphNeighborhoodResponse>
}

export interface UseGraphOverviewOptions {
  workspace: string | null
}

export function useGraphOverview({ workspace }: UseGraphOverviewOptions) {
  return useMutation<GraphNeighborhoodResponse, Error, Omit<GraphOverviewRequest, 'workspace'>>({
    mutationFn: (params) => {
      if (!workspace) return Promise.reject(new Error('No workspace selected'))
      return fetchOverview({ ...params, workspace })
    },
  })
}

export function nodeKindToMode(kind: NodeKind): 'code' | 'knowledge' {
  return kind === 'doc' ? 'knowledge' : 'code'
}
