import { useEffect, useMemo, useState } from 'react';
import { SigmaContainer, useLoadGraph, useRegisterEvents, useSigma } from '@react-sigma/core';
import '@react-sigma/core/lib/style.css';
import Graph from 'graphology';
import forceAtlas2 from 'graphology-layout-forceatlas2';

type GraphCanvasProps = {
  graph: Graph;
  onNodeClick?: (nodeId: string) => void;
  onNodeHover?: (nodeId?: string) => void;
};

type GraphInteractionProps = {
  onNodeClick?: (nodeId: string) => void;
  onNodeHover?: (nodeId?: string) => void;
};

function GraphLoader({ graph }: { graph: Graph }) {
  const loadGraph = useLoadGraph();
  useEffect(() => {
    loadGraph(graph);
  }, [graph, loadGraph]);
  return null;
}

function GraphInteractions({ onNodeClick, onNodeHover }: GraphInteractionProps) {
  const sigma = useSigma();
  const registerEvents = useRegisterEvents();

  useEffect(() => {
    registerEvents({
      clickNode: (event: any) => onNodeClick?.(event.node),
      enterNode: (event: any) => onNodeHover?.(event.node),
      leaveNode: () => onNodeHover?.(undefined),
    });
  }, [registerEvents, onNodeClick, onNodeHover]);

  useEffect(() => {
    sigma.setSetting('labelRenderedSizeThreshold', 10);
    sigma.setSetting('labelFont', 'Space Grotesk');
    sigma.setSetting('labelColor', { color: '#e4e4ed' });
    sigma.setSetting('edgeLabelColor', { color: '#94a3b8' });
  }, [sigma]);

  return null;
}

function GraphLayout({ graph }: { graph: Graph }) {
  useEffect(() => {
    if (graph.order < 2) return;
    try {
      const inferred = forceAtlas2.inferSettings(graph);
      forceAtlas2.assign(graph, {
        iterations: Math.min(120, Math.max(40, graph.order)),
        settings: {
          ...(inferred ?? {}),
          gravity: 0.6,
          scalingRatio: 10,
          slowDown: 10,
        },
      });
    } catch {
      // graph may lack edges for layout — positions from random init are fine
    }
  }, [graph]);
  return null;
}

export default function GraphCanvas({ graph, onNodeClick, onNodeHover }: GraphCanvasProps) {
  const [key, setKey] = useState(0);

  useEffect(() => {
    setKey((prev) => prev + 1);
  }, [graph]);

  const settings = useMemo(
    () => ({
      renderEdgeLabels: true,
      labelDensity: 0.5,
      zIndex: true,
      defaultNodeColor: '#64748b',
      defaultEdgeColor: 'rgba(148, 163, 184, 0.4)',
      labelColor: { color: '#e4e4ed' },
      labelFont: 'Space Grotesk, system-ui, sans-serif',
      labelRenderedSizeThreshold: 8,
    }),
    []
  );

  return (
    <SigmaContainer
      key={key}
      settings={settings}
      className="h-full w-full"
      style={{ background: '#0d0d14' }}
    >
      <GraphLoader graph={graph} />
      <GraphLayout graph={graph} />
      <GraphInteractions onNodeClick={onNodeClick} onNodeHover={onNodeHover} />
    </SigmaContainer>
  );
}
