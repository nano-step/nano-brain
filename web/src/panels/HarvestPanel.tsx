import { useState, useCallback } from 'react'
import { useDocuments } from '../hooks/useDocuments'
import { useDocDrawer } from '../hooks/useDocDrawer'
import { DocDrawer } from '../components/DocDrawer'
import { apiFetch } from '../api/client'
import { getCurrentWorkspace } from '../api/workspace'

function fmtAge(iso: string): string {
  const ms = Date.now() - new Date(iso).getTime()
  const s = Math.floor(ms / 1000)
  if (s < 60) return s + 's ago'
  const m = Math.floor(s / 60)
  if (m < 60) return m + 'm ago'
  const h = Math.floor(m / 60)
  if (h < 24) return h + 'h ago'
  return Math.floor(h / 24) + 'd ago'
}



export function HarvestPanel() {
  const workspace = getCurrentWorkspace()
  const [running, setRunning] = useState(false)
  const [sessionsSeen, setSessionsSeen] = useState(0)
  const [error, setError] = useState<string | null>(null)

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

      const eventSource = new EventSource('/api/v1/events')
      eventSource.addEventListener('harvest', (e: MessageEvent) => {
        try {
          const payload = JSON.parse(e.data) as { sessions_seen?: number }
          if (typeof payload.sessions_seen === 'number') {
            setSessionsSeen(payload.sessions_seen)
          }
        } catch {
          // non-JSON event payload, ignore
        }
      })

      const cleanup = () => {
        eventSource.close()
        setRunning(false)
      }

      eventSource.addEventListener('harvest_done', cleanup)
      eventSource.onerror = cleanup

      setTimeout(cleanup, 60_000)
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
