import { useCallback, useEffect, useState } from 'react';
import {
  ReactFlow,
  ReactFlowProvider,
  useNodesState,
  useEdgesState,
  useReactFlow,
  Background,
  Controls,
  type Node,
  type Edge,
  type NodeTypes,
} from '@xyflow/react';

type ReactFlowGraphProps = {
  nodes: Node[];
  edges: Edge[];
  onNodeClick?: (nodeId: string) => void;
  nodeTypes?: NodeTypes;
};

function ReactFlowGraphInner({ nodes: inputNodes, edges: inputEdges, onNodeClick, nodeTypes }: ReactFlowGraphProps) {
  const { fitView } = useReactFlow();
  const [nodes, setNodes, onNodesChange] = useNodesState(inputNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(inputEdges);
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);

  // Sync nodes from props (data or layout changed)
  useEffect(() => {
    setNodes(inputNodes);
    requestAnimationFrame(() => fitView({ padding: 0.15, duration: 300 }));
  }, [inputNodes, setNodes, fitView]);

  // Sync edges from props
  useEffect(() => {
    setEdges(inputEdges);
  }, [inputEdges, setEdges]);

  // Focus mode: dim nodes and edges not connected to selected node
  useEffect(() => {
    if (!selectedNodeId) {
      setNodes((nds) => nds.map((n) => ({ ...n, style: { ...n.style, opacity: 1 } })));
      setEdges((eds) => eds.map((e) => ({ ...e, animated: false, style: { ...e.style, opacity: 0.6, strokeWidth: 2 } })));
      return;
    }

    // Build 1-hop neighbor set
    const neighbors = new Set<string>([selectedNodeId]);
    for (const e of inputEdges) {
      if (e.source === selectedNodeId) neighbors.add(e.target);
      if (e.target === selectedNodeId) neighbors.add(e.source);
    }

    setNodes((nds) =>
      nds.map((n) => ({
        ...n,
        style: {
          ...n.style,
          opacity: neighbors.has(n.id) ? 1 : 0.08,
          transition: 'opacity 0.15s ease',
        },
      }))
    );

    setEdges((eds) =>
      eds.map((e) => {
        const connected = e.source === selectedNodeId || e.target === selectedNodeId;
        return connected
          ? { ...e, animated: true, style: { ...e.style, opacity: 0.9, strokeWidth: 3 } }
          : { ...e, animated: false, style: { ...e.style, opacity: 0.04, strokeWidth: 1 } };
      })
    );
  }, [selectedNodeId, inputEdges, setNodes, setEdges]);

  const handleNodeClick = useCallback(
    (_: React.MouseEvent, node: Node) => {
      setSelectedNodeId((prev) => (prev === node.id ? null : node.id));
      onNodeClick?.(node.id);
    },
    [onNodeClick]
  );

  const handlePaneClick = useCallback(() => {
    setSelectedNodeId(null);
    onNodeClick?.('');
  }, [onNodeClick]);

  if (inputNodes.length === 0) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-[#8888a0]">
        No graph data available
      </div>
    );
  }

  return (
    <ReactFlow
      nodes={nodes}
      edges={edges}
      onNodesChange={onNodesChange}
      onEdgesChange={onEdgesChange}
      onNodeClick={handleNodeClick}
      onPaneClick={handlePaneClick}
      nodeTypes={nodeTypes}
      colorMode="dark"
      minZoom={0.1}
      maxZoom={3}
      nodesConnectable={false}
      nodesDraggable={true}
      elementsSelectable={true}
      proOptions={{ hideAttribution: true }}
    >
      <Background gap={20} size={1} />
      <Controls showInteractive={false} />
    </ReactFlow>
  );
}

export default function ReactFlowGraph(props: ReactFlowGraphProps) {
  return (
    <ReactFlowProvider>
      <ReactFlowGraphInner {...props} />
    </ReactFlowProvider>
  );
}
