import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import GraphCanvas from '../components/GraphCanvas';
import NodeDetail from '../components/NodeDetail';
import QueryStatus from '../components/QueryStatus';
import { SkeletonGraph } from '../components/Skeleton';
import { fetchGraphEntities } from '../api/client';
import { buildEntityGraph } from '../lib/graph-adapter';
import { useAppStore } from '../store/app';

export default function GraphExplorer() {
  const workspace = useAppStore((state) => state.workspace);
  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['graph-entities', workspace],
    queryFn: () => fetchGraphEntities(workspace),
  });
  const [selectedNode, setSelectedNode] = useState<string | null>(null);
  const [nodeLimit, setNodeLimit] = useState(300);

  const graph = useMemo(() => {
    if (!data) return null;
    try {
      return buildEntityGraph(data, nodeLimit);
    } catch (err) {
      console.error('[GraphExplorer] graph build failed:', err);
      return null;
    }
  }, [data]);

  const selectedEntity = data?.nodes.find((node) => String(node.id) === selectedNode);

  if (isError) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-2xl font-semibold">Knowledge Graph</h1>
          <p className="mt-1 text-sm text-[#8888a0]">Explore memory entities and their relationships.</p>
        </div>
        <QueryStatus isLoading={false} isError={true} error={error} refetch={refetch} />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-semibold">Knowledge Graph</h1>
          <p className="mt-1 text-sm text-[#8888a0]">Explore memory entities and their relationships.</p>
        </div>
        <div className="flex items-center gap-4 text-xs text-[#8888a0]">
          <label className="flex items-center gap-2">
            Node limit:
            <input
              type="range"
              min={50}
              max={Math.max(500, data?.stats.nodeCount ?? 500)}
              step={50}
              value={nodeLimit}
              onChange={(e) => setNodeLimit(Number(e.target.value))}
              className="w-28 accent-indigo-500"
            />
            <span className="w-8 text-right text-[#e4e4ed]">{nodeLimit}</span>
          </label>
          <div className="text-right">
            <p>Nodes: {data?.stats.nodeCount ?? '—'}</p>
            <p>Edges: {data?.stats.edgeCount ?? '—'}</p>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-[1fr_320px] gap-6 max-lg:grid-cols-1">
        {isLoading ? (
          <SkeletonGraph />
        ) : (
          <div className="card graph-shell overflow-hidden">
            {graph ? (
              <GraphCanvas graph={graph} onNodeClick={(id) => setSelectedNode(id)} />
            ) : (
              <QueryStatus isLoading={false} isError={false} isEmpty={true} emptyText="No graph data available." />
            )}
          </div>
        )}
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
