import { useMemo, useCallback, useEffect } from 'react'
import { WikilinkRewriter } from './WikilinkRewriter'
import { BacklinksList } from './BacklinksList'
import { apiFetch } from '../api/client'
import type { Document } from '../api/types'
import { fmtAge } from '../utils/format'

const WIKILINK_RE = /\[\[([^\][\n]{1,200})\]\]/g

function countWikilinks(content: string): number {
  WIKILINK_RE.lastIndex = 0
  let count = 0
  let m: RegExpExecArray | null
  while ((m = WIKILINK_RE.exec(content)) !== null) {
    if (m.index === 0 || content[m.index - 1] !== '\\') count++
  }
  return count
}

interface DocDrawerProps {
  doc: Document
  workspace: string | null
  onClose: () => void
  onOpenDoc: (doc: Document) => void
}

export function DocDrawer({ doc, workspace, onClose, onOpenDoc }: DocDrawerProps) {
  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [onClose])

  const wikilinkCount = useMemo(() => countWikilinks(doc.content ?? ''), [doc.content])

  const handleCopyId = useCallback(() => {
    void navigator.clipboard.writeText(doc.id)
  }, [doc.id])

  const handleDelete = useCallback(async () => {
    if (!window.confirm(`Delete "${doc.title}"? This cannot be undone.`)) return
    if (!window.confirm(`Confirm permanent deletion of document ${doc.id}?`)) return
    const r = await apiFetch(`/api/v1/documents/${encodeURIComponent(doc.id)}`, { method: 'DELETE' })
    if (r.ok) onClose()
    else alert(`Delete failed: ${r.status} ${r.statusText}`)
  }, [doc.id, doc.title, onClose])

  const handleEdit = useCallback(async () => {
    const newContent = window.prompt('New content (will supersede this document):', doc.content ?? '')
    if (newContent == null) return
    const body = {
      workspace: workspace ?? '',
      source_path: `supersedes/${doc.id}`,
      content: newContent,
      supersedes_id: doc.id,
    }
    const r = await apiFetch('/api/v1/write', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
    if (!r.ok) alert(`Write failed: ${r.status} ${r.statusText}`)
    else onClose()
  }, [doc, workspace, onClose])

  const hasSupersedesChain = doc.supersedes_id || doc.superseded_by_id

  return (
    <>
      <div className="drawer-backdrop" onClick={onClose} role="presentation" />
      <div className="drawer" role="dialog" aria-modal="true" aria-label={doc.title}>
        <div className="drawer-head">
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: 16 }}>
            <div>
              <div className="drawer-title">{doc.title}</div>
              <div className="drawer-sub">
                {doc.collection} · {doc.id}
                {wikilinkCount > 0 && (
                  <>
                    {' '}·{' '}
                    <span style={{ color: 'var(--warn)' }}>
                      {wikilinkCount} wikilink{wikilinkCount === 1 ? '' : 's'}
                    </span>
                  </>
                )}
              </div>
            </div>
            <button className="btn" onClick={onClose} aria-label="Close drawer">
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="square" aria-hidden="true">
                <path d="M6 6l12 12M18 6L6 18" />
              </svg>
            </button>
          </div>
        </div>

        <div className="drawer-body">
          <div className="kv">
            <div className="kv-k">id</div>
            <div className="kv-v">{doc.id}</div>
            <div className="kv-k">collection</div>
            <div className="kv-v">{doc.collection}</div>
            <div className="kv-k">updated</div>
            <div className="kv-v">{doc.updated_at ? fmtAge(doc.updated_at) : '—'}</div>
            <div className="kv-k">tags</div>
            <div className="kv-v">[{(doc.tags ?? []).join(', ')}]</div>
            <div className="kv-k">supersedes</div>
            <div className="kv-v">{doc.supersedes_id ?? '—'}</div>
            <div className="kv-k">superseded_by</div>
            <div className="kv-v">{doc.superseded_by_id ?? '—'}</div>
          </div>

          {doc.content != null && (
            <div>
              <div className="section-title">Content · sanitized markdown · inline [[wikilinks]] resolved</div>
              <div className="surface surface-pad" style={{ fontSize: 13, lineHeight: 1.7 }}>
                <WikilinkRewriter content={doc.content} workspace={workspace} onOpenDoc={onOpenDoc} />
              </div>
            </div>
          )}

          {hasSupersedesChain && (
            <div>
              <div className="section-title">Supersession chain</div>
              <div className="surface surface-pad mono" style={{ fontSize: 12, color: 'var(--text-2)' }}>
                {doc.superseded_by_id && (
                  <div>↓ {doc.superseded_by_id} <span style={{ color: 'var(--text-4)' }}>(newer)</span></div>
                )}
                <div style={{ color: 'var(--accent)' }}>● {doc.id} <span style={{ color: 'var(--text-4)' }}>(this)</span></div>
                {doc.supersedes_id && (
                  <div>↑ {doc.supersedes_id} <span style={{ color: 'var(--text-4)' }}>(older)</span></div>
                )}
              </div>
            </div>
          )}

          <div>
            <div className="section-title">Referenced by</div>
            <div className="surface">
              <BacklinksList workspace={workspace} docId={doc.id} onOpenDoc={onOpenDoc} />
            </div>
          </div>

          <div style={{ display: 'flex', gap: 8 }}>
            <button className="btn btn-primary" onClick={handleEdit}>Edit (creates supersedes)</button>
            <button className="btn" onClick={handleCopyId}>Copy ID</button>
            <button className="btn btn-danger" onClick={handleDelete}>Delete…</button>
          </div>
        </div>
      </div>
    </>
  )
}
