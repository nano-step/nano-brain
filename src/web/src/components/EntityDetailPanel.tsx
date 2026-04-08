import { TYPE_COLORS, DEFAULT_TYPE_COLOR } from '../lib/colors';
import type { Edge } from '@xyflow/react';

type EntityData = {
  id: string;
  name: string;
  entityType: string;
  description?: string | null;
  firstLearnedAt?: string | null;
  lastConfirmedAt?: string | null;
  contradictedAt?: string | null;
};

type EntityDetailPanelProps = {
  entity: EntityData | null;
  edges: Edge[];
  nodeNames: Map<string, string>;
};

export default function EntityDetailPanel({ entity, edges, nodeNames }: EntityDetailPanelProps) {
  if (!entity) {
    return <div className="card p-4 text-sm text-[#8888a0]">Select a node to inspect details.</div>;
  }

  const tc = TYPE_COLORS[entity.entityType] || DEFAULT_TYPE_COLOR;

  // Find connected edges
  const connectedEdges = edges.filter(
    (e) => e.source === entity.id || e.target === entity.id
  );

  return (
    <div className="card overflow-hidden p-4">
      <div>
        <p className="text-sm uppercase tracking-[0.3em] text-[#8888a0]">Selected</p>
        <h3 className="mt-2 break-all text-lg font-semibold">{entity.name}</h3>
        <span
          className="mt-1 inline-block rounded-full px-2 py-0.5 text-[10px] font-medium"
          style={{ background: tc.dark.bg, color: tc.dark.text, border: `1px solid ${tc.border}` }}
        >
          {entity.entityType}
        </span>
      </div>

      {entity.description && (
        <p className="mt-3 break-words text-sm text-[#c2c2d6]">{entity.description}</p>
      )}

      <div className="mt-4 grid gap-3 text-sm">
        <div className="flex items-center justify-between border-b border-[#1f1f2c] pb-2">
          <span className="text-[#8888a0]">First learned</span>
          <span className="text-right text-[#e4e4ed]">{entity.firstLearnedAt ?? '—'}</span>
        </div>
        <div className="flex items-center justify-between border-b border-[#1f1f2c] pb-2">
          <span className="text-[#8888a0]">Last confirmed</span>
          <span className="text-right text-[#e4e4ed]">{entity.lastConfirmedAt ?? '—'}</span>
        </div>
        <div className="flex items-center justify-between border-b border-[#1f1f2c] pb-2">
          <span className="text-[#8888a0]">Contradicted</span>
          <span className="text-right text-[#e4e4ed]">{entity.contradictedAt ?? '—'}</span>
        </div>
      </div>

      {/* Relations list */}
      <div className="mt-4">
        <h4 className="text-xs font-semibold text-[#8888a0] uppercase tracking-wider">
          Relations ({connectedEdges.length})
        </h4>
        {connectedEdges.length === 0 ? (
          <p className="mt-2 text-xs text-[#646478]">No relations</p>
        ) : (
          <div className="mt-2 max-h-48 overflow-y-auto space-y-1">
            {connectedEdges.map((edge) => {
              const isSource = edge.source === entity.id;
              const otherId = isSource ? edge.target : edge.source;
              const otherName = nodeNames.get(otherId) ?? otherId;
              const edgeLabel = (edge.data as { edgeType?: string })?.edgeType
                ?? (typeof edge.label === 'string' ? edge.label : '—');
              return (
                <div
                  key={edge.id}
                  className="flex items-center justify-between rounded-lg bg-[#111118] px-2 py-1.5 text-xs"
                >
                  <span className="text-[#e4e4ed] truncate max-w-[140px]">{otherName}</span>
                  <span className="text-[#8888a0] text-[10px]">{edgeLabel}</span>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
