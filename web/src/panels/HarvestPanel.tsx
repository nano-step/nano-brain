import { useState, useCallback, useRef, useEffect } from 'react'
import { useDocuments } from '../hooks/useDocuments'
import { useDocDrawer } from '../hooks/useDocDrawer'
import { DocDrawer } from '../components/DocDrawer'
import { apiFetch } from '../api/client'
import { getCurrentWorkspace } from '../api/workspace'
import { fmtAge } from '../utils/format'

export function HarvestPanel() {
  const workspace = getCurrentWorkspace()
  const [running, setRunning] = useState(false)
  const [sessionsSeen, setSessionsSeen] = useState(0)
  const [error, setError] = useState<string | null>(null)
  const abortRef = useRef<AbortController | null>(null)

  useEffect(() => () => { abortRef.current?.abort() }, [])

  const { data: sessions, isLoading } = useDocuments({
    workspace,
    collection: 'session-summary',
  })

  const { doc: openDoc, open, close } = useDocDrawer()

  const handleTrigger = useCallback(async () => {
    if (running) return
    setRunning(true)
    setError(null)
    setSessionsSeen(0)

    try {
      const r = await apiFetch('/api/harvest', { method: 'POST' })
      if (!r.ok) {
        setError(`Harvest failed: ${r.status} ${r.statusText}`)
        setRunning(false)
        return
      }

      const abort = new AbortController()
      abortRef.current = abort

      const sseResp = await apiFetch('/api/v1/events', { signal: abort.signal })
      if (!sseResp.ok || !sseResp.body) {
        setRunning(false)
        return
      }

      const reader = sseResp.body.getReader()
      const decoder = new TextDecoder()
      let done = false
      const timeout = setTimeout(() => { done = true; abort.abort() }, 60_000)

      try {
        while (!done) {
          const { value, done: streamDone } = await reader.read()
          if (streamDone) break
          const chunk = decoder.decode(value, { stream: true })
          for (const line of chunk.split('\n')) {
            const trimmed = line.trim()
            if (trimmed.startsWith('data:')) {
              try {
                const payload = JSON.parse(trimmed.slice(5).trim()) as { sessions_seen?: number; event?: string }
                if (typeof payload.sessions_seen === 'number') {
                  setSessionsSeen(payload.sessions_seen)
                }
                if (payload.event === 'harvest_done') { done = true }
              } catch { }
            }
            if (trimmed === 'event: harvest_done') { done = true }
          }
        }
      } finally {
        clearTimeout(timeout)
        reader.cancel()
        setRunning(false)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error')
      setRunning(false)
    }
  }, [running])

  if (!workspace) {
    return (
      <div className="panel">
        <div style={{ padding: 48, textAlign: 'center', color: 'var(--text-3)', fontSize: 13 }}>
          No workspace selected.
        </div>
      </div>
    )
  }

  const docs = sessions ?? []

  return (
    <>
      <div className="panel">
        <div className="surface">
          <div className="surface-head">
            <span>Harvest pipeline</span>
            <button
              className="btn btn-primary"
              disabled={running}
              onClick={handleTrigger}
            >
              {running ? 'Harvesting…' : 'Trigger harvest'}
            </button>
          </div>
          <div className="surface-pad" style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            {error && (
              <div style={{ color: 'var(--err)', fontSize: 12 }}>{error}</div>
            )}
            {running && (
              <>
                <div style={{ display: 'flex', gap: 32, fontSize: 12 }}>
                  <div>
                    <span style={{ color: 'var(--text-3)' }}>Sessions seen</span>
                    <div className="stat-value" style={{ fontSize: 20 }}>{sessionsSeen}</div>
                  </div>
                </div>
                <div className="progress">
                  <div className="progress-bar" style={{ width: running ? '100%' : '0%', transition: 'width 600ms' }} />
                </div>
              </>
            )}
            {!running && !error && (
              <div style={{ color: 'var(--text-3)', fontSize: 12 }}>
                Click "Trigger harvest" to ingest new sessions.
              </div>
            )}
          </div>
        </div>

        <div className="surface">
          <div className="surface-head">
            <span>Recent sessions</span>
            <span style={{ color: 'var(--text-3)', fontSize: 11 }}>collection: session-summary:*</span>
          </div>
          <table className="table">
            <thead>
              <tr>
                <th>Session</th>
                <th>Collection</th>
                <th>When</th>
              </tr>
            </thead>
            <tbody>
              {docs.map((doc) => (
                <tr
                  key={doc.id}
                  onClick={() => open(doc)}
                >
                  <td>
                    <div className="mem-title">{doc.title}</div>
                    <div style={{ color: 'var(--text-3)', fontSize: 11, marginTop: 2, fontFamily: 'JetBrains Mono, monospace' }}>
                      {(doc.content ?? '').slice(0, 110)}{doc.content && doc.content.length > 110 ? '…' : ''}
                    </div>
                  </td>
                  <td className="mono" style={{ color: 'var(--text-2)' }}>{doc.collection}</td>
                  <td className="mem-time">{fmtAge(doc.updated_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {!isLoading && docs.length === 0 && (
            <div style={{ padding: 32, textAlign: 'center', color: 'var(--text-3)', fontSize: 12 }}>
              No sessions harvested yet. Trigger a harvest to get started.
            </div>
          )}
          {isLoading && (
            <div style={{ padding: 24, textAlign: 'center', color: 'var(--text-3)', fontSize: 12 }}>
              Loading…
            </div>
          )}
        </div>
      </div>

      {openDoc && (
        <DocDrawer
          doc={openDoc}
          workspace={workspace}
          onClose={close}
          onOpenDoc={open}
        />
      )}
    </>
  )
}
