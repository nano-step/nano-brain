import type { NodeKind } from '../../api/types'

interface LegendRowProps {
  color: string
  label: string
}

function LegendRow({ color, label }: LegendRowProps) {
  return (
    <div className="legend-row">
      <span className="legend-dot" style={{ background: color }} />
      {label}
    </div>
  )
}

interface GraphLegendProps {
  mode: NodeKind
}

export function GraphLegend({ mode }: GraphLegendProps) {
  return (
    <div className="graph-legend">
      {mode === 'symbol' ? (
        <>
          <LegendRow color="var(--accent)" label="calls" />
          <LegendRow color="var(--warn)" label="imports" />
          <LegendRow color="var(--ok)" label="contains" />
          <div className="legend-row" style={{ marginTop: 6, color: 'var(--text-3)' }}>
            Click → select · Double-click → Symbols panel
          </div>
        </>
      ) : (
        <>
          <LegendRow color="var(--accent)" label="memory" />
          <LegendRow color="var(--ok)" label="session-summary:opencode" />
          <LegendRow color="var(--warn)" label="session-summary:claudecode" />
          <LegendRow color="var(--text-3)" label="symbol:*" />
          <div className="legend-row" style={{ marginTop: 6, color: 'var(--text-3)' }}>
            Double-click → open drawer · Edges = wikilinks
          </div>
        </>
      )}
    </div>
  )
}
