import { useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../api/client'

export function useRemoveWorkspace() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (hash: string) => {
      const r = await apiFetch(`/api/v1/workspaces/${encodeURIComponent(hash)}`, {
        method: 'DELETE',
      })
      if (!r.ok) {
        const text = await r.text().catch(() => '')
        throw new Error(`${r.status} ${r.statusText}${text ? `: ${text}` : ''}`)
      }
      return hash
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['workspaces'] })
    },
  })
}
