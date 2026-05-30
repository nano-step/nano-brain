import { useState, useMemo } from 'react'
import { useSymbols } from '../hooks/useSymbols'
import { getCurrentWorkspace } from '../api/workspace'

const ALL_KINDS = ['all', 'function', 'method', 'type', 'interface', 'struct', 'const', 'var'] as const
const ALL_LANGS = ['all', 'go', 'python', 'typescript', 'javascript'] as const

export function SymbolsPanel() {
  const workspace = getCurrentWorkspace()
  const [q, setQ] = useState('')
  const [kind, setKind] = useState('all')
  const [lang, setLang] = useState('all')

  const { data: symbols, isLoading, error } = useSymbols({
    workspace,
    q: q || undefined,
    kind: kind !== 'all' ? kind : undefined,
    language: lang !== 'all' ? lang : undefined,
  })

  const sorted = useMemo(
    () => [...(symbols ?? [])].sort((a, b) => b.impact - a.impact),
    [symbols],
  )

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
          placeholder="Symbol name (type-ahead)…"
          value={q}
          onChange={(e) => setQ(e.target.value)}
          autoFocus
          aria-label="Symbol name filter"
        />
        <span style={{ color: 'var(--text-3)', fontSize: 11 }}>kind</span>
        {ALL_KINDS.map((k) => (
          <button
            key={k}
            className="chip"
            data-active={kind === k}
            onClick={() => setKind(k)}
            aria-pressed={kind === k}
          >
            {k}
          </button>
        ))}
        <span style={{ color: 'var(--text-3)', fontSize: 11 }}>lang</span>
        {ALL_LANGS.map((l) => (
          <button
            key={l}
            className="chip"
            data-active={lang === l}
            onClick={() => setLang(l)}
            aria-pressed={lang === l}
          >
            {l}
          </button>
        ))}
      </div>

      {error && (
        <div style={{ padding: 24, color: 'var(--err)', fontSize: 12 }}>
          Failed to load symbols: {error instanceof Error ? error.message : 'Unknown error'}
        </div>
      )}

      <div className="surface">
        <table className="table">
          <thead>
            <tr>
              <th>Symbol</th>
              <th>Kind</th>
              <th>Lang</th>
              <th>Location</th>
              <th className="num">Impact</th>
            </tr>
          </thead>
          <tbody>
            {sorted.map((s) => (
              <tr key={`${s.source_path}:${s.name}`}>
                <td>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                    <span className="mono" style={{ fontSize: 12 }}>{s.name}</span>
                    <span style={{ color: 'var(--text-3)', fontSize: 11 }}>{s.signature}</span>
                  </div>
                </td>
                <td>
                  <span className="pill">{s.kind}</span>
                </td>
                <td className="mono" style={{ color: 'var(--text-3)' }}>{s.language}</td>
                <td className="mono" style={{ fontSize: 11, color: 'var(--text-2)' }}>
                  {s.source_path}<span style={{ color: 'var(--text-3)' }}>:{s.line}</span>
                </td>
                <td className="num">{s.impact || '—'}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {!isLoading && sorted.length === 0 && (
          <div style={{ padding: 32, textAlign: 'center', color: 'var(--text-3)', fontSize: 12 }}>
            {q || kind !== 'all' || lang !== 'all'
              ? 'No symbols match. Adjust filters.'
              : 'No symbols indexed yet.'}
          </div>
        )}
        {isLoading && (
          <div style={{ padding: 24, textAlign: 'center', color: 'var(--text-3)', fontSize: 12 }}>
            Loading…
          </div>
        )}
      </div>
    </div>
  )
}
