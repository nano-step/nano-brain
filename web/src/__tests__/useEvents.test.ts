import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useEvents, useEventStore } from '../hooks/useEvents'

interface FakeES {
  url: string
  onopen: (() => void) | null
  onerror: ((e: Event) => void) | null
  onmessage: ((e: MessageEvent) => void) | null
  readyState: number
  close: () => void
}

let captured: FakeES | null = null
const OriginalEventSource = globalThis.EventSource

function makeFakeEventSource(url: string): FakeES {
  const es: FakeES = {
    url,
    onopen: null,
    onerror: null,
    onmessage: null,
    readyState: 0,
    close() { es.readyState = 2 },
  }
  captured = es
  return es
}

describe('useEvents', () => {
  beforeEach(() => {
    captured = null
    vi.stubGlobal('EventSource', makeFakeEventSource)
    useEventStore.setState({ connected: false, embedQueueDepth: 0, lastEvent: null })
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    globalThis.EventSource = OriginalEventSource as typeof EventSource
  })

  it('opens EventSource with workspace URL', () => {
    renderHook(() => useEvents('abc123'))
    expect(captured?.url).toBe('/api/v1/events?workspace=abc123')
  })

  it('sets connected on open', () => {
    renderHook(() => useEvents('abc123'))
    act(() => captured?.onopen?.())
    expect(useEventStore.getState().connected).toBe(true)
  })

  it('sets connected=false on error', () => {
    renderHook(() => useEvents('abc123'))
    act(() => captured?.onopen?.())
    act(() => captured?.onerror?.(new Event('error')))
    expect(useEventStore.getState().connected).toBe(false)
  })

  it('updates embedQueueDepth from embed_queue event', () => {
    renderHook(() => useEvents('abc123'))
    act(() =>
      captured?.onmessage?.({
        data: JSON.stringify({
          type: 'embed_queue',
          workspace: 'abc123',
          payload: { depth: 42, processing: 1 },
          ts: new Date().toISOString(),
        }),
      } as MessageEvent),
    )
    expect(useEventStore.getState().embedQueueDepth).toBe(42)
  })

  it('skips when workspace is null', () => {
    renderHook(() => useEvents(null))
    expect(captured).toBeNull()
  })

  it('closes EventSource on unmount', () => {
    const { unmount } = renderHook(() => useEvents('ws1'))
    const es = captured!
    unmount()
    expect(es.readyState).toBe(2)
    expect(useEventStore.getState().connected).toBe(false)
  })
})
