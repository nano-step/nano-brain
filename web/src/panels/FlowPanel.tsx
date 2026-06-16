import React, { useState, useEffect, useRef, useCallback } from 'react'
import { getCurrentWorkspace } from '../api/workspace'
import { apiFetch } from '../api/client'
import mermaid from 'mermaid'

interface FlowNode {
  id: string
  name: string
  role: string
  ambiguous?: boolean
}

interface FlowResponse {
  found: boolean
  entry: string
  method?: string
  path?: string
  chain: FlowNode[]
  externals: FlowNode[]
  mermaid?: string
  message?: string
}

interface HttpEndpoint {
  source: string
  target: string
}

mermaid.initialize({
  startOnLoad: false,
  theme: 'default',
  securityLevel: 'loose',
  maxTextSize: 200000,
})

export function FlowPanel() {
  const workspace = getCurrentWorkspace()
  const [entry, setEntry] = useState('')
  const [endpoints, setEndpoints] = useState<HttpEndpoint[]>([])
  const [flowData, setFlowData] = useState<FlowResponse | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [isLoadingEndpoints, setIsLoadingEndpoints] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const mermaidRef = useRef<HTMLDivElement>(null)
  const renderId = useRef(0)

  const fetchEndpoints = useCallback(async () => {
    if (!workspace) return

    setIsLoadingEndpoints(true)
    try {
      const res = await apiFetch(`/api/v1/graph/flow/endpoints?workspace=${workspace}`)
      if (res.ok) {
        const data = await res.json()
        setEndpoints(data.endpoints || [])
      }
    } catch (err) {
      console.error('Failed to fetch endpoints:', err)
    } finally {
      setIsLoadingEndpoints(false)
    }
  }, [workspace])

  useEffect(() => {
    fetchEndpoints()
  }, [fetchEndpoints])

  const fetchFlow = useCallback(async (entryPoint: string) => {
    if (!workspace || !entryPoint.trim()) return

    setIsLoading(true)
    setError(null)

    try {
      const res = await apiFetch('/api/v1/graph/flow', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          workspace,
          entry: entryPoint.trim(),
          format: 'mermaid',
          max_depth: 4,
          max_fanout: 4,
        }),
      })

      if (!res.ok) {
        throw new Error(`${res.status} ${res.statusText}`)
      }

      const data: FlowResponse = await res.json()
      setFlowData(data)

      if (!data.found) {
        setError(data.message || 'Entry not found')
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch flow')
    } finally {
      setIsLoading(false)
    }
  }, [workspace])

  useEffect(() => {
    if (!flowData?.mermaid || !mermaidRef.current) return

    const id = `mermaid-${++renderId.current}`
    mermaidRef.current.innerHTML = ''

    mermaid.render(id, flowData.mermaid).then(({ svg }) => {
      if (mermaidRef.current) {
        mermaidRef.current.innerHTML = svg
      }
    }).catch((err) => {
      console.error('Mermaid render error:', err)
      if (mermaidRef.current) {
        mermaidRef.current.innerHTML = `<pre style="color: red">${flowData.mermaid}</pre>`
      }
    })
  }, [flowData])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    fetchFlow(entry)
  }

  const handleEndpointClick = (source: string) => {
    setEntry(source)
    fetchFlow(source)
  }

  if (!workspace) {
    return (
      <div className="panel">
        <div style={{ padding: '48px', textAlign: 'center', color: 'var(--text-3)', fontSize: 13 }}>
          No workspace selected. Register a workspace with{' '}
          <span className="mono">nano-brain init</span> and reload.
        </div>
      </div>
    )
  }

  return (
    <div className="panel">
      <div className="filter-bar">
        <form onSubmit={handleSubmit} style={{ display: 'flex', gap: 8, flex: 1 }}>
          <input
            className="filter-input"
            placeholder="Entry point (e.g., POST /api/v1/query)..."
            value={entry}
            onChange={(e) => setEntry(e.target.value)}
            style={{ flex: 1 }}
          />
          <button type="submit" className="chip" disabled={isLoading || !entry.trim()}>
            {isLoading ? 'Loading...' : 'Show Flow'}
          </button>
        </form>
      </div>

      {error && (
        <div style={{ padding: 16, color: 'var(--err)', fontSize: 13 }}>
          {error}
        </div>
      )}

      {flowData?.found && flowData.chain.length > 0 && (
        <div style={{ padding: '8px 16px', fontSize: 12, color: 'var(--text-2)' }}>
          <span className="pill">{flowData.chain.length} nodes</span>
          {flowData.method && <span className="pill" style={{ marginLeft: 4 }}>{flowData.method}</span>}
          {flowData.path && <span className="pill" style={{ marginLeft: 4 }}>{flowData.path}</span>}
        </div>
      )}

      <div
        ref={mermaidRef}
        style={{
          padding: 16,
          minHeight: 200,
          overflow: 'auto',
        }}
      />

      {!flowData && !error && !isLoading && (
        <div style={{
          padding: '16px',
          borderTop: '1px solid var(--border)',
        }}>
          <div style={{ fontSize: 12, color: 'var(--text-3)', marginBottom: 8 }}>
            {isLoadingEndpoints ? 'Loading endpoints...' : `Available endpoints (${endpoints.length})`}
          </div>
          {endpoints.length > 0 && (
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
              {endpoints.map((ep) => (
                <button
                  key={ep.source}
                  className="chip"
                  onClick={() => handleEndpointClick(ep.source)}
                  style={{ fontSize: 11 }}
                >
                  {ep.source}
                </button>
              ))}
            </div>
          )}
          {endpoints.length === 0 && !isLoadingEndpoints && (
            <div style={{ color: 'var(--text-3)', fontSize: 12 }}>
              No HTTP endpoints found. Try indexing some source files first.
            </div>
          )}
        </div>
      )}
    </div>
  )
}
