import { useBacklinks } from '../hooks/useBacklinks'
import type { Document } from '../api/types'
import { fmtAge } from '../utils/format'

interface BacklinksListProps {
  workspace: string | null
  docId: string
  onOpenDoc: (doc: Document) => void
}

export function BacklinksList({ workspace, docId, onOpenDoc }: BacklinksListProps) {
  const { data: backlinks, isLoading, error } = useBacklinks(workspace, docId)

  if (isLoading) {
    return (
      <div style={{ padding: 16, color: 'var(--text-3)', fontSize: 12 }}>
        Loading backlinks…
      </div>
    )
  }

  if (error) {
    return (
      <div style={{ padding: 16, color: 'var(--err)', fontSize: 12 }}>
        Failed to load backlinks
      </div>
    )
  }

  if (!backlinks || backlinks.length === 0) {
    return (
      <div style={{ padding: 24, color: 'var(--text-3)', fontSize: 12, textAlign: 'center' }}>
        No other documents reference this yet. Other notes can link here via{' '}
        <code className="mono">[[{docId}]]</code>.
      </div>
    )
  }

  return (
    <div>
      {backlinks.map((b) => (
        <div
          key={b.id}
          className="mem-row"
          style={{ gridTemplateColumns: '22px 1fr 110px' }}
          onClick={() => onOpenDoc({ id: b.id, title: b.title, collection: b.collection, tags: b.tags, updated_at: b.updated_at } as unknown as Document)}
          role="button"
          tabIndex={0}
          onKeyDown={(e) => e.key === 'Enter' && onOpenDoc({ id: b.id, title: b.title, collection: b.collection, tags: b.tags, updated_at: b.updated_at } as unknown as Document)}
        >
          <div className="mem-icon" aria-hidden="true">
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="square">
              <path d="M21 3H3v6h18V3zm0 8H3v10h18V11zM7 15h6v2H7v-2z" />
            </svg>
          </div>
          <div>
            <div className="mem-title">{b.title}</div>
            <div className="mem-meta mono" style={{ whiteSpace: 'normal', color: 'var(--text-3)' }}>
              {b.snippet}
            </div>
          </div>
          <div className="mem-time">{fmtAge(b.updated_at)}</div>
        </div>
      ))}
    </div>
  )
}
