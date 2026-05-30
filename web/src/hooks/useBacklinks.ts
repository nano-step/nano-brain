import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../api/client'
import type { Backlink } from '../api/types'

export function useBacklinks(workspace: string | null, docId: string | null) {
  return useQuery({
    queryKey: ['backlinks', workspace, docId],
    queryFn: async () => {
      const qs = new URLSearchParams()
      if (workspace) qs.set('workspace', workspace)
      const r = await apiFetch(`/api/v1/links/${encodeURIComponent(docId!)}/backlinks?${qs.toString()}`)
      if (!r.ok) throw new Error(`${r.status} ${r.statusText}`)
      const data = await r.json() as { backlinks?: Backlink[] } | Backlink[]
      return Array.isArray(data) ? data : (data.backlinks ?? [])
    },
    enabled: !!workspace && !!docId,
    staleTime: 10_000,
  })
}
