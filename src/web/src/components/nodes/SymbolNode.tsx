import { memo } from 'react';
import { Handle, Position } from '@xyflow/react';

const HANDLE_STYLE = { opacity: 0, width: 0, height: 0, pointerEvents: 'none' as const };

export type SymbolNodeData = {
  label: string;
  entityType: string;
  color: string;
  filePath?: string;
  startLine?: number;
  clusterId?: number | null;
};

const SymbolNode = memo(function SymbolNode({ data }: { data: SymbolNodeData }) {
  return (
    <div
      className="px-3 py-1 cursor-grab"
      style={{
        background: `${data.color}18`,
        border: `2px solid ${data.color}`,
        borderRadius: 12,
      }}
    >
      <Handle type="target" position={Position.Top} style={HANDLE_STYLE} />
      <Handle type="source" position={Position.Bottom} style={HANDLE_STYLE} />
      <div className="text-[10px] font-semibold whitespace-nowrap" style={{ color: data.color }}>
        {data.label}
      </div>
      <div className="text-[8px] text-center" style={{ color: data.color, opacity: 0.5 }}>
        {data.entityType}
      </div>
    </div>
  );
});

export default SymbolNode;
