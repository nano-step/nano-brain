import { useState, useMemo, useCallback } from 'react'
import { useDocuments } from '../hooks/useDocuments'
import { useDocDrawer } from '../hooks/useDocDrawer'
import { DocDrawer } from '../components/DocDrawer'
import { getCurrentWorkspace } from '../api/workspace'
import type { Document } from '../api/types'
import { fmtAge } from '../utils/format'

export function MemoryPanel() {
  const workspace = getCurrentWorkspace()
  const [textFilter, setTextFilter] = useState('')
  const [activeTags, setActiveTags] = useState<string[]>([])
  const [collection] = useState('')

  const { data: docs, isLoading, error } = useDocuments({
    workspace,
    text: textFilter || undefined,
    tags: activeTags.length > 0 ? activeTags : undefined,
    collection: collection || undefined,
  })

  const { doc: openDoc, open, close } = useDocDrawer()

  const allTags = useMemo(() => {
    if (!docs) return []
    const set = new Set<string>()
    docs.forEach((d) => d.tags?.forEach((t) => set.add(t)))
    return Array.from(set).sort()
  }, [docs])

  const toggleTag = useCallback((tag: string) => {
    setActiveTags((curr) =>
      curr.includes(tag) ? curr.filter((t) => t !== tag) : [...curr, tag],
    )
  }, [])

  const handleRowClick = useCallback((doc: Document) => {
    open(doc)
  }, [open])

  if (!workspace) {
    return (
      <div className="panel">
        <div style={{ padding: 48, textAlign: 'center', color: 'var(--text-3)', fontSize: 13 }}>
          No workspace selected. Register a workspace with <span className="mono">nano-brain init</span> and reload.
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="panel">
        <div style={{ padding: 48, textAlign: 'center', color: 'var(--err)', fontSize: 13 }}>
          Failed to load documents: {error instanceof Error ? error.message : 'Unknown error'}
        </div>
      </div>
    )
  }

  const documents = docs ?? []

  return (
    <>
      <div className="panel">
        <div className="filter-bar">
          <input
            className="filter-input"
            placeholder="Filter memories (BM25)…"
            value={textFilter}
            onChange={(e) => setTextFilter(e.target.value)}
            aria-label="Filter by text"
          />
          {allTags.map((tag) => (
            <button
              key={tag}
              className="chip"
              data-active={activeTags.includes(tag)}
              onClick={() => toggleTag(tag)}
              aria-pressed={activeTags.includes(tag)}
            >
              {tag}
            </button>
          ))}
          {activeTags.length > 0 && (
            <button className="chip" onClick={() => setActiveTags([])} aria-label="Clear tag filters">
              clear ✕
            </button>
          )}
          <span style={{ marginLeft: 'auto', color: 'var(--text-3)', fontSize: 12 }}>
            {isLoading ? 'Loading…' : `${documents.length} documents`}
          </span>
        </div>

        <div className="surface">
          {isLoading ? (
            <div style={{ padding: 32, textAlign: 'center', color: 'var(--text-3)', fontSize: 12 }}>
              Loading…
            </div>
          ) : documents.length === 0 ? (
            <div style={{ padding: 48, textAlign: 'center', color: 'var(--text-3)' }}>
              <div style={{ fontSize: 14, marginBottom: 6 }}>No memories match these filters.</div>
              <div style={{ fontSize: 12 }}>Clear filters or adjust the search query.</div>
            </div>
          ) : (
            documents.map((doc) => (
              <div
                key={doc.id}
                className="mem-row"
                onClick={() => handleRowClick(doc)}
                role="button"
                tabIndex={0}
                onKeyDown={(e) => e.key === 'Enter' && handleRowClick(doc)}
              >
                <div className="mem-icon" aria-hidden="true">
                  <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="square">
                    <path d="M21 3H3v6h18V3zm0 8H3v10h18V11zM7 15h6v2H7v-2z" />
                  </svg>
                </div>
                <div>
                  <div className="mem-title">{doc.title}</div>
                  <div className="mem-meta mono">{doc.collection} · {doc.id}</div>
                </div>
                <div className="mem-tags">
                  {(doc.tags ?? []).map((t) => (
                    <span key={t} className="mem-tag">{t}</span>
                  ))}
                </div>
                <div className="mem-time">{fmtAge(doc.updated_at)}</div>
                <div className="mem-chain">
                  {doc.supersedes_id ? '↑ ' + doc.supersedes_id : doc.superseded_by_id ? '↓ ' + doc.superseded_by_id : '—'}
                </div>
              </div>
            ))
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
