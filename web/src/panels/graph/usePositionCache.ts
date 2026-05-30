import { useCallback } from 'react'
import type { EdgeType, GraphDirection, NodeKind } from '../../api/types'

export type PositionMap = Record<string, [number, number]>

interface CacheKey {
  workspace: string
  mode: NodeKind
  focus: string
  depth: number
  direction: GraphDirection
  edge_types: EdgeType[]
}

function buildKey(k: CacheKey): string {
  const sorted = [...k.edge_types].sort().join(',')
  return `nano-brain.graph-positions.${k.workspace}.${k.mode}.${encodeURIComponent(k.focus)}.${k.depth}.${k.direction}.${sorted}`
}

export function usePositionCache(key: CacheKey) {
  const storageKey = buildKey(key)

  const read = useCallback((): PositionMap | null => {
    try {
      const raw = localStorage.getItem(storageKey)
      if (!raw) return null
      return JSON.parse(raw) as PositionMap
    } catch {
      return null
    }
  }, [storageKey])

  const write = useCallback((positions: PositionMap): void => {
    try {
      localStorage.setItem(storageKey, JSON.stringify(positions))
    } catch {
      // quota exceeded or private mode — silently ignore
    }
  }, [storageKey])

  const clear = useCallback((): void => {
    try {
      localStorage.removeItem(storageKey)
    } catch {
      // ignore
    }
  }, [storageKey])

  return { read, write, clear, storageKey }
}
