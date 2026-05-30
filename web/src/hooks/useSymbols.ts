import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../api/client'
import type { Symbol } from '../api/types'

export interface SymbolsParams {
  workspace: string | null
  q?: string
  kind?: string
  language?: string
}

export function useSymbols(params: SymbolsParams) {
  const { workspace, q, kind, language } = params
  return useQuery({
    queryKey: ['symbols', workspace, q, kind, language],
    queryFn: async () => {
      const qs = new URLSearchParams()
      if (workspace) qs.set('workspace', workspace)
      if (q) qs.set('q', q)
      if (kind && kind !== 'all') qs.set('kind', kind)
      if (language && language !== 'all') qs.set('language', language)
      const r = await apiFetch(`/api/v1/symbols?${qs.toString()}`)
      if (!r.ok) throw new Error(`${r.status} ${r.statusText}`)
      const data = await r.json() as { symbols?: Symbol[] } | Symbol[]
      return Array.isArray(data) ? data : (data.symbols ?? [])
    },
    enabled: !!workspace,
    staleTime: 30_000,
  })
}
