import { useState, useEffect, useRef } from 'react'
import { useDocuments } from '../hooks/useDocuments'
import { useStats } from '../hooks/useStats'
import { getCurrentWorkspace } from '../api/workspace'
import { apiFetch } from '../api/client'

interface FlowEntry {
  method: string
  path: string
  fullPath: string
}

interface FlowResponse {
  found: boolean
  entry?: string
  method?: string
  path?: string
  chain?: Array<{ id: string; name: string; role: string }>
  externals?: Array<{ id: string; name: string; role: string }>
  mermaid?: string
  message?: string
}

export function FlowsPanel() {
  const [workspace, setWorkspace] = useState(() => getCurrentWorkspace())
  const [filter, setFilter] = useState('')
  const [selectedEntry, setSelectedEntry] = useState<FlowEntry | null>(null)
  const [flowData, setFlowData] = useState<FlowResponse | null>(null)
  const [loadingFlow, setLoadingFlow] = useState(false)
  const [flowError, setFlowError] = useState<string | null>(null)

  useEffect(() => {
    const handleStorage = () => {
      setWorkspace(getCurrentWorkspace())
      setSelectedEntry(null)
      setFlowData(null)
    }
    window.addEventListener('storage', handleStorage)
    window.addEventListener('popstate', handleStorage)
    return () => {
      window.removeEventListener('storage', handleStorage)
      window.removeEventListener('popstate', handleStorage)
    }
  }, [])

  const [zoom, setZoom] = useState(1)
  const [pan, setPan] = useState({ x: 0, y: 0 })
  const diagramRef = useRef<HTMLDivElement>(null)
  const dragRef = useRef({ dragging: false, startX: 0, startY: 0, startPanX: 0, startPanY: 0 })

  const { data: documents, isLoading } = useDocuments({
    workspace,
    collection: 'flows',
  })

  const { data: stats, refetch: refetchStats } = useStats(workspace)
  const [reindexStatus, setReindexStatus] = useState<'idle' | 'queued' | 'polling' | 'done' | 'error'>('idle')
  const [reindexMsg, setReindexMsg] = useState('')

  const flowCount = stats?.collections?.find((c: { name: string }) => c.name === 'flows')?.doc_count ?? 0
  const httpEdges = stats?.graph_edges_by_type?.http ?? 0

  const handleReindex = async () => {
    if (!workspace || reindexStatus === 'queued' || reindexStatus === 'polling') return
    setReindexStatus('queued')
    setReindexMsg('Sending materialize request…')

    try {
      const r = await apiFetch('/api/v1/flow/materialize', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ workspace }),
      })
      if (!r.ok) throw new Error(`HTTP ${r.status}`)
      const beforeCount = flowCount
      setReindexMsg(`Materializing flows… (current: ${beforeCount})`)

      setReindexStatus('polling')
      let attempts = 0
      const maxAttempts = 30
      const poll = async () => {
        attempts++
        await new Promise(res => setTimeout(res, 2000))
        await refetchStats()
        const freshStats = await refetchStats()
        const newCount = freshStats?.data?.collections?.find((c: { name: string }) => c.name === 'flows')?.doc_count ?? 0
        setReindexMsg(`Polling… ${attempts}/${maxAttempts} — flows: ${newCount}`)

        if (newCount !== beforeCount) {
          setReindexStatus('done')
          setReindexMsg(`Done! Flows: ${beforeCount} → ${newCount}`)
          return
        }
        if (attempts >= maxAttempts) {
          setReindexStatus('done')
          setReindexMsg(`Timeout after ${maxAttempts * 2}s — flows: ${newCount}`)
          return
        }
        poll()
      }
      poll()
    } catch (e) {
      setReindexStatus('error')
      setReindexMsg(`Error: ${e instanceof Error ? e.message : 'Unknown'}`)
    }
  }

  const entries: FlowEntry[] = (documents ?? []).map((doc) => {
    const title = doc.title ?? ''
    const path = title.replace(/\s*flow$/i, '').replace(/^flow:\/\//, '')
    const methodMatch = path.match(/^(GET|POST|PUT|DELETE|PATCH)\s+/)
    const method = methodMatch ? methodMatch[1] : 'GET'
    const pathOnly = path.replace(/^(GET|POST|PUT|DELETE|PATCH)\s+/, '')
    return { method, path: pathOnly, fullPath: path }
  }).sort((a, b) => a.path.localeCompare(b.path))

  const filtered = filter
    ? entries.filter(
        (e) =>
          e.path.toLowerCase().includes(filter.toLowerCase()) ||
          e.method.toLowerCase().includes(filter.toLowerCase()),
      )
    : entries

  useEffect(() => {
    const el = diagramRef.current
    if (!el) return

    const onWheel = (e: WheelEvent) => {
      if (e.ctrlKey || e.metaKey) {
        e.preventDefault()
        setZoom(z => Math.min(3, Math.max(0.25, z - e.deltaY * 0.005)))
      }
    }

    const onMouseDown = (e: MouseEvent) => {
      if (e.button !== 0) return
      dragRef.current = { dragging: true, startX: e.clientX, startY: e.clientY, startPanX: pan.x, startPanY: pan.y }
      el.style.cursor = 'grabbing'
    }

    const onMouseMove = (e: MouseEvent) => {
      const d = dragRef.current
      if (!d.dragging) return
      setPan({ x: d.startPanX + (e.clientX - d.startX), y: d.startPanY + (e.clientY - d.startY) })
    }

    const onMouseUp = () => {
      dragRef.current.dragging = false
      el.style.cursor = 'grab'
    }

    el.addEventListener('wheel', onWheel, { passive: false })
    el.addEventListener('mousedown', onMouseDown)
    window.addEventListener('mousemove', onMouseMove)
    window.addEventListener('mouseup', onMouseUp)
    return () => {
      el.removeEventListener('wheel', onWheel)
      el.removeEventListener('mousedown', onMouseDown)
      window.removeEventListener('mousemove', onMouseMove)
      window.removeEventListener('mouseup', onMouseUp)
    }
  }, [pan.x, pan.y])

  useEffect(() => {
    if (!selectedEntry || !workspace) {
      setFlowData(null)
      return
    }

    let cancelled = false
    setLoadingFlow(true)
    setFlowError(null)

    apiFetch('/api/v1/graph/flow', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        workspace,
        entry: selectedEntry.fullPath,
        max_depth: 5,
        format: 'mermaid',
      }),
    })
      .then((r) => r.json())
      .then((data: FlowResponse) => {
        if (!cancelled) {
          setFlowData(data)
          setLoadingFlow(false)
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setFlowError(err instanceof Error ? err.message : 'Failed to load flow')
          setLoadingFlow(false)
        }
      })

    return () => {
      cancelled = true
    }
  }, [selectedEntry, workspace])

  useEffect(() => {
    if (flowData?.mermaid && diagramRef.current) {
      import('mermaid').then((mermaid) => {
        mermaid.default.initialize({
          startOnLoad: false,
          theme: 'dark',
          securityLevel: 'loose',
        })
        const node = diagramRef.current?.querySelector('.mermaid')
        if (node) {
          mermaid.default.run({ nodes: [node as HTMLElement] }).catch(() => {})
        }
      })
    }
  }, [flowData])

  if (!workspace) {
    return (
      <div className="panel">
        <div style={{ padding: 48, textAlign: 'center', color: 'var(--text-3)', fontSize: 13 }}>
          No workspace selected.
        </div>
      </div>
    )
  }

  return (
    <div className="panel">
      <div className="filter-bar">
        <input
          className="filter-input"
          placeholder="Filter endpoints…"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          autoFocus
          aria-label="Endpoint filter"
        />
        <span className="pill pill-accent">{flowCount} flows</span>
        <span className="pill">{httpEdges} http edges</span>
        <button
          className="btn"
          style={{ fontSize: 11 }}
          onClick={handleReindex}
          disabled={reindexStatus === 'queued' || reindexStatus === 'polling'}
        >
          {reindexStatus === 'queued' ? 'Queuing…' : reindexStatus === 'polling' ? 'Materializing…' : 'Materialize Flows'}
        </button>
        {reindexMsg && (
          <span style={{
            color: reindexStatus === 'error' ? 'var(--err)' : reindexStatus === 'done' ? 'var(--ok)' : 'var(--text-3)',
            fontSize: 11
          }}>
            {reindexMsg}
          </span>
        )}
        <span style={{ color: 'var(--text-3)', fontSize: 11 }}>
          {filtered.length} endpoints
        </span>
      </div>

      <div className="surface" style={{ display: 'grid', gridTemplateColumns: '300px 1fr', gap: 1 }}>
        <div style={{ maxHeight: 'calc(100vh - 200px)', overflowY: 'auto' }}>
          <table className="table">
            <thead>
              <tr>
                <th style={{ width: 50 }}>Method</th>
                <th>Path</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((entry) => (
                <tr
                  key={entry.fullPath}
                  onClick={() => setSelectedEntry(entry)}
                  style={{
                    cursor: 'pointer',
                    background:
                      selectedEntry?.fullPath === entry.fullPath
                        ? 'var(--bg-active)'
                        : undefined,
                  }}
                >
                  <td>
                    <span
                      className="mono"
                      style={{
                        fontSize: 10,
                        fontWeight: 700,
                        color:
                          entry.method === 'GET'
                            ? '#58a6ff'
                            : entry.method === 'POST'
                              ? '#3fb950'
                              : entry.method === 'DELETE'
                                ? '#f85149'
                                : '#d29922',
                      }}
                    >
                      {entry.method}
                    </span>
                  </td>
                  <td className="mono" style={{ fontSize: 11, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {entry.path}
                  </td>
                </tr>
              ))}
              {filtered.length === 0 && !isLoading && (
                <tr>
                  <td colSpan={2} style={{ textAlign: 'center', color: 'var(--text-3)', padding: 24, fontSize: 12 }}>
                    No endpoints found.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>

        <div style={{ borderLeft: '1px solid var(--border)', minHeight: 400, display: 'flex', flexDirection: 'column' }}>
          <div style={{ padding: '8px 16px', borderBottom: '1px solid var(--border)', display: 'flex', alignItems: 'center', gap: 8 }}>
            <span style={{ color: 'var(--text-3)', fontSize: 11 }}>
              {Math.round(zoom * 100)}% · Ctrl+Scroll to zoom · Drag to pan
            </span>
            <button className="btn" style={{ fontSize: 11, padding: '2px 8px', marginLeft: 'auto' }} onClick={() => { setZoom(1); setPan({ x: 0, y: 0 }) }}>Reset</button>
          </div>
          {!selectedEntry && (
            <div style={{ padding: 48, textAlign: 'center', color: 'var(--text-3)', fontSize: 13 }}>
              Select an endpoint to view its execution flow.
            </div>
          )}
          {selectedEntry && loadingFlow && (
            <div style={{ padding: 48, textAlign: 'center', color: 'var(--text-3)', fontSize: 13 }}>
              Loading flow diagram…
            </div>
          )}
          {selectedEntry && flowError && (
            <div style={{ padding: 24, color: 'var(--err)', fontSize: 12 }}>
              Error: {flowError}
            </div>
          )}
          {selectedEntry && flowData && !flowData.found && (
            <div style={{ padding: 24, color: 'var(--text-3)', fontSize: 12 }}>
              No flow found for "{selectedEntry.fullPath}".
              {flowData.message && <div style={{ marginTop: 8 }}>{flowData.message}</div>}
            </div>
          )}
          {selectedEntry && flowData?.found && (
            <div style={{ padding: 16 }}>
              <div className="flow-meta" style={{ marginBottom: 16 }}>
                <dt>Entry</dt>
                <dd>{flowData.entry}</dd>
                {flowData.method && (
                  <>
                    <dt>Method</dt>
                    <dd>{flowData.method}</dd>
                  </>
                )}
                {flowData.path && (
                  <>
                    <dt>Path</dt>
                    <dd>{flowData.path}</dd>
                  </>
                )}
              </div>
              {flowData.externals && flowData.externals.length > 0 && (
                <div style={{ marginBottom: 16 }}>
                  <div style={{ fontSize: 11, color: 'var(--text-3)', marginBottom: 8 }}>
                    External dependencies:
                  </div>
                  <div className="externals">
                    {flowData.externals.map((n) => (
                      <span key={n.id} className="external-tag">
                        {n.name}
                      </span>
                    ))}
                  </div>
                </div>
              )}
              <div
                ref={diagramRef}
                style={{ overflow: 'hidden', minHeight: 200, flex: 1, padding: 16, cursor: 'grab', position: 'relative' }}
              >
                <div style={{ transform: `translate(${pan.x}px, ${pan.y}px) scale(${zoom})`, transformOrigin: '0 0' }}>
                  {flowData.mermaid && <div className="mermaid">{flowData.mermaid}</div>}
                </div>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
