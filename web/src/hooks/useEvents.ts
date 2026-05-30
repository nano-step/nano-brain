import { useEffect, useRef } from 'react'
import { create } from 'zustand'
import type { SSEEvent, EmbedQueuePayload } from '../api/types'

interface EventStore {
  connected: boolean
  embedQueueDepth: number
  lastEvent: SSEEvent | null
  setConnected: (v: boolean) => void
  setEmbedQueueDepth: (n: number) => void
  setLastEvent: (e: SSEEvent) => void
}

export const useEventStore = create<EventStore>((set) => ({
  connected: false,
  embedQueueDepth: 0,
  lastEvent: null,
  setConnected: (v) => set({ connected: v }),
  setEmbedQueueDepth: (n) => set({ embedQueueDepth: n }),
  setLastEvent: (e) => set({ lastEvent: e }),
}))

export function useEvents(workspace: string | null): void {
  const esRef = useRef<EventSource | null>(null)
  const { setConnected, setEmbedQueueDepth, setLastEvent } = useEventStore()

  useEffect(() => {
    if (!workspace) return

    const url = `/api/v1/events?workspace=${encodeURIComponent(workspace)}`
    const es = new EventSource(url)
    esRef.current = es

    es.onopen = () => setConnected(true)

    es.onerror = () => {
      setConnected(false)
    }

    es.onmessage = (e) => {
      try {
        const event = JSON.parse(e.data as string) as SSEEvent
        setLastEvent(event)

        if (event.type === 'embed_queue') {
          const p = event.payload as EmbedQueuePayload
          setEmbedQueueDepth(p.depth ?? 0)
        }
      } catch (_) {
        void _
      }
    }

    return () => {
      es.close()
      esRef.current = null
      setConnected(false)
    }
  }, [workspace, setConnected, setEmbedQueueDepth, setLastEvent])
}
