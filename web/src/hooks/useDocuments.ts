import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../api/client'
import type { Document } from '../api/types'

export interface DocumentsParams {
  workspace: string | null
  text?: string
  tags?: string[]
  collection?: string
}

export function useDocuments(params: DocumentsParams) {
  const { workspace, text, tags, collection } = params
  return useQuery({
    queryKey: ['documents', workspace, text, tags, collection],
    queryFn: async () => {
      const q = new URLSearchParams()
      if (workspace) q.set('workspace', workspace)
      if (text) q.set('text', text)
      if (tags && tags.length > 0) q.set('tags', tags.join(','))
      if (collection) q.set('collection', collection)
      const r = await apiFetch(`/api/v1/query?${q.toString()}`)
      if (!r.ok) throw new Error(`${r.status} ${r.statusText}`)
      const data = await r.json() as { documents?: Document[] } | Document[]
      return Array.isArray(data) ? data : (data.documents ?? [])
    },
    enabled: !!workspace,
    staleTime: 15_000,
  })
}
