import { memo } from 'react';
import { Handle, Position } from '@xyflow/react';

const HANDLE_STYLE = { opacity: 0, width: 0, height: 0, pointerEvents: 'none' as const };

export type DocumentNodeData = {
  label: string;
  fullPath: string;
  color: string;
};

const DocumentNode = memo(function DocumentNode({ data }: { data: DocumentNodeData }) {
  return (
    <div
      className="px-3 py-1.5 cursor-grab"
      style={{
        background: `${data.color}18`,
        border: `2px solid ${data.color}`,
        borderRadius: 10,
      }}
    >
      <Handle type="target" position={Position.Top} style={HANDLE_STYLE} />
      <Handle type="source" position={Position.Bottom} style={HANDLE_STYLE} />
      <div className="text-[10px] font-medium whitespace-nowrap" style={{ color: data.color }}>
        {data.label}
      </div>
    </div>
  );
});

export default DocumentNode;
