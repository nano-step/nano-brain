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
      clickNode: (event) => onNodeClick?.(event.node),
      enterNode: (event) => onNodeHover?.(event.node),
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
    const settings = forceAtlas2.inferSettings(graph);
    const iterations = Math.min(120, Math.max(40, graph.order));
    forceAtlas2.assign(graph, {
      iterations,
      settings: {
        ...settings,
        gravity: 0.6,
        scalingRatio: 10,
        slowDown: 10,
      },
    });
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
    }),
    []
  );

  return (
    <SigmaContainer key={key} graph={graph} settings={settings} className="h-full w-full">
      <GraphLoader graph={graph} />
      <GraphLayout graph={graph} />
      <GraphInteractions onNodeClick={onNodeClick} onNodeHover={onNodeHover} />
    </SigmaContainer>
  );
}
