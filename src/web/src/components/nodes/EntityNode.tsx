import { memo } from 'react';
import { Handle, Position } from '@xyflow/react';
import { TYPE_COLORS, DEFAULT_TYPE_COLOR } from '../../lib/colors';

const HANDLE_STYLE = { opacity: 0, width: 0, height: 0, pointerEvents: 'none' as const };

export type EntityNodeData = {
  label: string;
  entityType: string;
  degree: number;
  description?: string | null;
  firstLearnedAt?: string | null;
  lastConfirmedAt?: string | null;
  contradictedAt?: string | null;
};

const EntityNode = memo(function EntityNode({ data }: { data: EntityNodeData }) {
  const tc = TYPE_COLORS[data.entityType] || DEFAULT_TYPE_COLOR;
  const t = tc.dark; // nano-brain UI is dark-only
  const isHub = data.degree >= 4;
  return (
    <div
      className="px-4 py-1.5 cursor-grab"
      style={{
        background: t.bg,
        border: `2px solid ${tc.border}`,
        borderRadius: 20,
        boxShadow: isHub ? `0 0 8px ${tc.border}40` : undefined,
      }}
    >
      <Handle type="target" position={Position.Top} style={HANDLE_STYLE} />
      <Handle type="source" position={Position.Bottom} style={HANDLE_STYLE} />
      <div className="text-[11px] font-semibold whitespace-nowrap" style={{ color: t.text }}>
        {data.label}
      </div>
      <div className="text-[9px] text-center" style={{ color: t.text, opacity: 0.6 }}>
        {data.entityType}
      </div>
    </div>
  );
});

export default EntityNode;
