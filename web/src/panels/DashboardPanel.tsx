import { useStats } from '../hooks/useStats'
import { useEvents, useEventStore } from '../hooks/useEvents'
import { getCurrentWorkspace } from '../api/workspace'
import type { StatsResponse, GraphEdgesByType } from '../api/types'

function fmtNum(n: number): string {
  return n.toLocaleString()
}

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

function fmtUptime(sec: number): string {
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  return `${h}h ${m}m`
}

function StatCard({ label, value, sub }: { label: string; value: string; sub?: string }) {
  return (
    <div className="stat-card">
      <div className="stat-label">{label}</div>
      <div className="stat-value mono">{value}</div>
      {sub && <div className="stat-sub">{sub}</div>}
    </div>
  )
}

function Pill({
  children,
  variant,
}: {
  children: React.ReactNode
  variant?: 'ok' | 'warn' | 'err' | 'accent'
}) {
  return <span className={`pill${variant ? ' pill-' + variant : ''}`}>{children}</span>
}

function EmbedStatusCard({ stats }: { stats: StatsResponse }) {
  const { pending, embedded, embed_failed } = stats.chunks_by_embed_status
  const total = pending + embedded + embed_failed
  const pct = total > 0 ? Math.round((embedded / total) * 100) : 0

  return (
    <div className="surface">
      <div className="surface-head">
        <span>Embed status</span>
        <Pill variant={embed_failed > 0 ? 'warn' : 'ok'}>{embed_failed} failed</Pill>
      </div>
      <div className="surface-pad" style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12 }}>
          <span style={{ color: 'var(--text-2)' }}>Embedded</span>
          <span className="mono">{fmtNum(embedded)}</span>
        </div>
        <div className="progress">
          <div className="progress-bar ok" style={{ width: `${pct}%` }} />
        </div>
        <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12 }}>
          <span style={{ color: 'var(--text-2)' }}>Pending</span>
          <span className="mono">{fmtNum(pending)}</span>
        </div>
        <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12 }}>
          <span style={{ color: 'var(--text-2)' }}>Failed</span>
          <span className="mono" style={{ color: 'var(--err)' }}>
            {fmtNum(embed_failed)}
          </span>
        </div>
      </div>
    </div>
  )
}

function GraphCardinalityCard({ edges }: { edges: GraphEdgesByType }) {
  return (
    <div className="surface">
      <div className="surface-head">
        <span>Graph cardinality</span>
        <Pill>by edge type</Pill>
      </div>
      <div className="surface-pad">
        <table className="table" style={{ fontSize: 12 }}>
          <tbody>
            {Object.entries(edges).map(([k, v]) => (
              <tr key={k} style={{ cursor: 'default' }}>
                <td>
                  <Pill variant={k === 'calls' ? 'accent' : k === 'imports' ? 'warn' : 'ok'}>
                    {k}
                  </Pill>
                </td>
                <td className="num">{fmtNum(v ?? 0)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function RecentDocsTable({ stats }: { stats: StatsResponse }) {
  const docs = stats.recent_docs ?? []

  return (
    <div className="surface">
      <div className="surface-head">
        <span>Recent documents</span>
        <span style={{ color: 'var(--text-3)', fontSize: 11 }}>10 most recently updated</span>
      </div>
      <div>
        {docs.map((d) => (
          <div key={d.id} className="mem-row">
            <div className="mem-icon" aria-hidden="true">
              <svg
                width="13"
                height="13"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.6"
                strokeLinecap="square"
              >
                <path d="M21 3H3v6h18V3zm0 8H3v10h18V11zM7 15h6v2H7v-2z" />
              </svg>
            </div>
            <div>
              <div className="mem-title">{d.title}</div>
              <div className="mem-meta mono">
                {d.collection} · {d.id}
              </div>
            </div>
            <div className="mem-tags">
              {d.tags.map((t) => (
                <span key={t} className="mem-tag">
                  {t}
                </span>
              ))}
            </div>
            <div className="mem-time">{fmtAge(d.updated_at)}</div>
            <div className="mem-chain">
              {d.supersedes
                ? '↑ ' + d.supersedes
                : d.superseded_by
                  ? '↓ ' + d.superseded_by
                  : '—'}
            </div>
          </div>
        ))}
        {docs.length === 0 && (
          <div style={{ padding: '32px', textAlign: 'center', color: 'var(--text-3)', fontSize: 12 }}>
            No documents yet. Write your first memory note to get started.
          </div>
        )}
      </div>
    </div>
  )
}

function SkeletonGrid() {
  return (
    <div className="stat-grid">
      {Array.from({ length: 6 }).map((_, i) => (
        <div key={i} className="stat-card">
          <div className="stat-label skel" style={{ width: '60%', height: 10, marginBottom: 8 }} />
          <div className="stat-value skel" style={{ width: '80%', height: 22 }} />
        </div>
      ))}
    </div>
  )
}

export function DashboardPanel() {
  const workspace = getCurrentWorkspace()
  const { data: stats, isLoading, error } = useStats(workspace)
  const embedQueueDepth = useEventStore((s) => s.embedQueueDepth)

  useEvents(workspace)

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

  if (error) {
    return (
      <div className="panel">
        <div style={{ padding: '48px', textAlign: 'center', color: 'var(--err)', fontSize: 13 }}>
          Failed to load stats:{' '}
          {error instanceof Error ? error.message : 'Unknown error'}
        </div>
      </div>
    )
  }

  if (isLoading || !stats) {
    return (
      <div className="panel">
        <SkeletonGrid />
      </div>
    )
  }

  const totalEdges = Object.values(stats.graph_edges_by_type).reduce(
    (a: number, b) => a + (b ?? 0),
    0,
  )

  return (
    <div className="panel">
      <div className="stat-grid">
        <StatCard
          label="Server"
          value={'v' + stats.server_version}
          sub={'up ' + fmtUptime(stats.uptime_sec)}
        />
        <StatCard
          label="Embeddings"
          value={fmtNum(stats.embeddings_total)}
          sub={stats.embedding.provider + ' · ' + stats.embedding.model}
        />
        <StatCard label="Documents" value={fmtNum(stats.docs_total)} />
        <StatCard
          label="Chunks"
          value={fmtNum(stats.chunks_total)}
          sub={
            stats.chunks_total > 0
              ? String(Math.round(
                  (stats.chunks_by_embed_status.embedded / stats.chunks_total) * 100,
                )) + '% embedded'
              : '0% embedded'
          }
        />
        <StatCard
          label="Graph edges"
          value={fmtNum(totalEdges)}
          sub="contains · imports · calls"
        />
        <StatCard
          label="Embed queue"
          value={fmtNum(embedQueueDepth)}
          sub={embedQueueDepth > 0 ? 'live · streaming' : 'idle'}
        />
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 24 }}>
        <EmbedStatusCard stats={stats} />
        <GraphCardinalityCard edges={stats.graph_edges_by_type} />
      </div>

      <RecentDocsTable stats={stats} />
    </div>
  )
}
