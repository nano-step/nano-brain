import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import GraphCanvas from '../components/GraphCanvas';
import NodeDetail from '../components/NodeDetail';
import { fetchGraphEntities } from '../api/client';
import { buildEntityGraph } from '../lib/graph-adapter';
import { useAppStore } from '../store/app';

export default function GraphExplorer() {
  const workspace = useAppStore((state) => state.workspace);
  const { data, isLoading } = useQuery({
    queryKey: ['graph-entities', workspace],
    queryFn: () => fetchGraphEntities(workspace),
  });
  const [selectedNode, setSelectedNode] = useState<string | null>(null);

  const graph = useMemo(() => (data ? buildEntityGraph(data) : null), [data]);
  const selectedEntity = data?.nodes.find((node) => String(node.id) === selectedNode);

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-semibold">Knowledge Graph</h1>
          <p className="mt-1 text-sm text-[#8888a0]">Explore memory entities and their relationships.</p>
        </div>
        <div className="text-right text-xs text-[#8888a0]">
          <p>Nodes: {data?.stats.nodeCount ?? '—'}</p>
          <p>Edges: {data?.stats.edgeCount ?? '—'}</p>
        </div>
      </div>

      <div className="grid grid-cols-[1fr_320px] gap-6">
        <div className="card graph-shell overflow-hidden">
          {graph ? (
            <GraphCanvas graph={graph} onNodeClick={(id) => setSelectedNode(id)} />
          ) : (
            <div className="flex h-full items-center justify-center text-sm text-[#8888a0]">
              {isLoading ? 'Loading graph...' : 'No graph data.'}
            </div>
          )}
        </div>
        <div className="space-y-4">
          {selectedEntity ? (
            <NodeDetail
              title={selectedEntity.name}
              subtitle={selectedEntity.type}
              description={selectedEntity.description}
              meta={[
                { label: 'First learned', value: selectedEntity.firstLearnedAt },
                { label: 'Last confirmed', value: selectedEntity.lastConfirmedAt },
                { label: 'Contradicted', value: selectedEntity.contradictedAt },
              ]}
            />
          ) : (
            <div className="card p-4 text-sm text-[#8888a0]">Select a node to inspect details.</div>
          )}
          <div className="card p-4">
            <h3 className="text-sm font-semibold">Type distribution</h3>
            <div className="mt-3 grid gap-2 text-xs text-[#8888a0]">
              {data?.stats.typeDistribution
                ? Object.entries(data.stats.typeDistribution).map(([type, count]) => (
                    <div key={type} className="flex items-center justify-between">
                      <span>{type}</span>
                      <span className="text-[#e4e4ed]">{count}</span>
                    </div>
                  ))
                : '—'}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
