import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { renderHook } from '@testing-library/react'
import { usePositionCache } from '../panels/graph/usePositionCache'

import type { EdgeType, GraphDirection, NodeKind } from '../api/types'

const baseKey = {
  workspace: 'ws-test',
  mode: 'symbol' as NodeKind,
  focus: 'processQuery',
  depth: 2,
  direction: 'both' as GraphDirection,
  edge_types: ['calls', 'contains'] as EdgeType[],
}

describe('usePositionCache', () => {
  beforeEach(() => localStorage.clear())
  afterEach(() => localStorage.clear())

  it('read returns null when nothing cached', () => {
    const { result } = renderHook(() => usePositionCache(baseKey))
    expect(result.current.read()).toBeNull()
  })

  it('write stores positions and read retrieves them', () => {
    const { result } = renderHook(() => usePositionCache(baseKey))
    const pos = { processQuery: [1.5, 2.3] as [number, number] }
    result.current.write(pos)
    expect(result.current.read()).toEqual(pos)
  })

  it('clear removes stored positions', () => {
    const { result } = renderHook(() => usePositionCache(baseKey))
    result.current.write({ node1: [0, 0] })
    result.current.clear()
    expect(result.current.read()).toBeNull()
  })

  it('Code and Knowledge modes use separate keys (no cross-mode bleed)', () => {
    const codeKey = { ...baseKey, mode: 'symbol' as const }
    const knowledgeKey = { ...baseKey, mode: 'doc' as const }

    const { result: codeCache } = renderHook(() => usePositionCache(codeKey))
    const { result: knowCache } = renderHook(() => usePositionCache(knowledgeKey))

    codeCache.current.write({ nodeA: [10, 20] })
    expect(knowCache.current.read()).toBeNull()
  })

  it('storageKey includes mode for separation', () => {
    const codeKey = { ...baseKey, mode: 'symbol' as const }
    const knowledgeKey = { ...baseKey, mode: 'doc' as const }
    const { result: c } = renderHook(() => usePositionCache(codeKey))
    const { result: k } = renderHook(() => usePositionCache(knowledgeKey))
    expect(c.current.storageKey).not.toBe(k.current.storageKey)
  })
})
