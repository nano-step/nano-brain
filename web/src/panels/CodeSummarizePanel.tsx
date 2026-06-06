import { useState, useCallback, useEffect, useRef } from 'react'
import { apiFetch } from '../api/client'
import { getCurrentWorkspace } from '../api/workspace'

interface SummarizeStatus {
  total_symbols: number
  summarized: number
  pending: number
  failed: number
}

interface Failure {
  id: string
  symbol_name: string
  source_file: string
  error_reason: string
  attempts: number
}

export function CodeSummarizePanel() {
  const workspace = getCurrentWorkspace()
  const [status, setStatus] = useState<SummarizeStatus | null>(null)
  const [failures, setFailures] = useState<Failure[]>([])
  const [running, setRunning] = useState(false)
  const [retrying, setRetrying] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const abortRef = useRef<AbortController | null>(null)

  useEffect(() => () => { abortRef.current?.abort() }, [])

  const fetchStatus = useCallback(async () => {
    if (!workspace) return
    try {
      const r = await apiFetch(`/api/v1/code/summarize/status?workspace=${workspace}`, { signal: abortRef.current?.signal })
      if (!r.ok) throw new Error(`${r.status} ${r.statusText}`)
      const data = await r.json() as SummarizeStatus
      setStatus(data)
    } catch (err) {
      if (err instanceof Error && err.name !== 'AbortError') {
        setError(err.message)
      }
    }
  }, [workspace])

  const fetchFailures = useCallback(async () => {
    if (!workspace) return
    try {
      const r = await apiFetch(`/api/v1/code/summarize/failures?workspace=${workspace}`, { signal: abortRef.current?.signal })
      if (!r.ok) throw new Error(`${r.status} ${r.statusText}`)
      const data = await r.json() as { failures: Failure[] }
      setFailures(data.failures ?? [])
    } catch (err) {
      if (!(err instanceof Error && err.name === 'AbortError')) {
        // silently handle non-abort errors for failures
      }
    }
  }, [workspace])

  useEffect(() => {
    if (!workspace) return
    setLoading(true)
    Promise.all([fetchStatus(), fetchFailures()]).finally(() => setLoading(false))
  }, [workspace, fetchStatus, fetchFailures])

  const handleTrigger = useCallback(async () => {
    if (running || !workspace) return
    setRunning(true)
    setError(null)
    try {
      const r = await apiFetch('/api/v1/code/summarize', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ workspace }),
      })
      if (!r.ok) {
        setError(`Summarize failed: ${r.status} ${r.statusText}`)
      } else {
        await fetchStatus()
        await fetchFailures()
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error')
    } finally {
      setRunning(false)
    }
  }, [running, workspace, fetchStatus, fetchFailures])

  const handleRetryAll = useCallback(async () => {
    if (retrying || !workspace) return
    setRetrying(true)
    setError(null)
    try {
      const r = await apiFetch('/api/v1/code/summarize/retry-all', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ workspace }),
      })
      if (!r.ok) {
        setError(`Retry failed: ${r.status} ${r.statusText}`)
      } else {
        await fetchStatus()
        await fetchFailures()
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error')
    } finally {
      setRetrying(false)
    }
  }, [retrying, workspace, fetchStatus, fetchFailures])

  if (!workspace) {
    return (
      <div className="panel">
        <div style={{ padding: 48, textAlign: 'center', color: 'var(--text-3)', fontSize: 13 }}>
          No workspace selected.
        </div>
      </div>
    )
  }

  const total = status?.total_symbols ?? 0
  const summarized = status?.summarized ?? 0
  const pct = total > 0 ? Math.round((summarized / total) * 100) : 0

  return (
    <div className="panel">
      <div className="surface">
        <div className="surface-head">
          <span>Code Summarization</span>
          <div style={{ display: 'flex', gap: 8 }}>
            {failures.length > 0 && (
              <button
                className="btn"
                disabled={retrying}
                onClick={handleRetryAll}
              >
                {retrying ? 'Retrying…' : 'Retry All Failed'}
              </button>
            )}
            <button
              className="btn btn-primary"
              disabled={running}
              onClick={handleTrigger}
            >
              {running ? 'Summarizing…' : 'Summarize Now'}
            </button>
          </div>
        </div>
        <div className="surface-pad" style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          {error && (
            <div style={{ color: 'var(--err)', fontSize: 12 }}>{error}</div>
          )}

          {loading && <div className="skel" style={{ height: 60 }} />}

          {!loading && status && (
            <>
              <div style={{ display: 'flex', gap: 32, fontSize: 12 }}>
                <div>
                  <span style={{ color: 'var(--text-3)' }}>Total</span>
                  <div className="stat-value" style={{ fontSize: 20 }}>{total.toLocaleString()}</div>
                </div>
                <div>
                  <span style={{ color: 'var(--text-3)' }}>Summarized</span>
                  <div className="stat-value" style={{ fontSize: 20 }}>{summarized.toLocaleString()}</div>
                </div>
                <div>
                  <span style={{ color: 'var(--text-3)' }}>Pending</span>
                  <div className="stat-value" style={{ fontSize: 20 }}>{(status.pending ?? 0).toLocaleString()}</div>
                </div>
                <div>
                  <span style={{ color: 'var(--text-3)' }}>Failed</span>
                  <div className="stat-value" style={{ fontSize: 20, color: status.failed > 0 ? 'var(--err)' : undefined }}>
                    {(status.failed ?? 0).toLocaleString()}
                  </div>
                </div>
              </div>
              <div className="progress">
                <div className="progress-bar" style={{ width: `${pct}%`, transition: 'width 300ms' }} />
              </div>
              <div style={{ color: 'var(--text-3)', fontSize: 11 }}>{pct}% complete</div>
            </>
          )}

          {!loading && !status && !error && (
            <div style={{ color: 'var(--text-3)', fontSize: 12 }}>
              Click "Summarize Now" to generate LLM summaries of code symbols.
            </div>
          )}
        </div>
      </div>

      {failures.length > 0 && (
        <div className="surface">
          <div className="surface-head">
            <span>Failures</span>
            <span style={{ color: 'var(--text-3)', fontSize: 11 }}>{failures.length} failed</span>
          </div>
          <table className="table" style={{ fontSize: 13 }}>
            <thead>
              <tr>
                <th style={{ textAlign: 'left' }}>Symbol</th>
                <th style={{ textAlign: 'left' }}>File</th>
                <th style={{ textAlign: 'left' }}>Error</th>
                <th style={{ textAlign: 'right' }}>Attempts</th>
              </tr>
            </thead>
            <tbody>
              {failures.map((f) => (
                <tr key={f.id}>
                  <td className="mono" style={{ fontSize: 11 }}>{f.symbol_name}</td>
                  <td className="mono" style={{ fontSize: 11, color: 'var(--text-2)' }}>{f.source_file}</td>
                  <td style={{ color: 'var(--err)', fontSize: 11 }}>{f.error_reason}</td>
                  <td className="num">{f.attempts}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
