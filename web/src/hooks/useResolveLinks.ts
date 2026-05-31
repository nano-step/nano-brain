import { useQueries } from '@tanstack/react-query'
import { apiFetch } from '../api/client'
import type { ResolveResponse } from '../api/types'

export function useResolveLinks(workspace: string | null, queries: string[]) {
  return useQueries({
    queries: queries.map((query) => ({
      queryKey: ['resolve-link', workspace, query],
      queryFn: async (): Promise<ResolveResponse> => {
        const qs = new URLSearchParams()
        if (workspace) qs.set('workspace', workspace)
        qs.set('query', query)
        const r = await apiFetch(`/api/v1/links/resolve?${qs.toString()}`)
        if (!r.ok) return { matched: [], ambiguous: false, kind: 'title' }
        return r.json() as Promise<ResolveResponse>
      },
      enabled: !!workspace && query.length > 0,
      staleTime: 60_000,
    })),
  })
}
