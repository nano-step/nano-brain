import Graph from 'graphology';
import { ConnectionsResponse, GraphEntity, GraphEntitiesResponse, SymbolsResponse } from '../api/client';
import { edgeTypeColorMap, fallbackColors, relationshipColorMap, symbolKindColorMap, typeColorMap } from './colors';

function randomLayout(graph: Graph, scale: number) {
  graph.forEachNode((node) => {
    graph.setNodeAttribute(node, 'x', (Math.random() - 0.5) * scale);
    graph.setNodeAttribute(node, 'y', (Math.random() - 0.5) * scale);
  });
}

type GraphNodeMeta = {
  id: string;
  label: string;
  entityType: string;
  size: number;
  color: string;
  description?: string | null;
  firstLearnedAt?: string | null;
  lastConfirmedAt?: string | null;
  contradictedAt?: string | null;
};

type SymbolNodeMeta = {
  id: string;
  label: string;
  entityType: string;
  size: number;
  color: string;
  filePath?: string;
  startLine?: number;
  clusterId?: number | null;
};

export function buildEntityGraph(data: GraphEntitiesResponse) {
  const graph = new Graph();
  const edgeCounts = new Map<number, number>();

  for (const edge of data.edges) {
    edgeCounts.set(edge.sourceId, (edgeCounts.get(edge.sourceId) || 0) + 1);
    edgeCounts.set(edge.targetId, (edgeCounts.get(edge.targetId) || 0) + 1);
  }

  for (const node of data.nodes) {
    const count = edgeCounts.get(node.id) || 0;
    const size = Math.max(5, Math.min(25, 5 + count * 1.2));
    graph.addNode(String(node.id), {
      id: String(node.id),
      label: node.name,
      entityType: node.type,
      size,
      color: typeColorMap[node.type] || '#64748b',
      description: node.description,
      firstLearnedAt: node.firstLearnedAt,
      lastConfirmedAt: node.lastConfirmedAt,
      contradictedAt: node.contradictedAt,
    } satisfies GraphNodeMeta);
  }

  for (const edge of data.edges) {
    const source = String(edge.sourceId);
    const target = String(edge.targetId);
    if (graph.hasNode(source) && graph.hasNode(target)) {
      graph.addEdgeWithKey(String(edge.id), source, target, {
        label: edge.edgeType,
        edgeType: edge.edgeType,
        size: 1,
        color: 'rgba(148, 163, 184, 0.4)',
      });
    }
  }

  randomLayout(graph, 800);
  return graph;
}

export function buildCodeGraph(files: Array<{ path: string; centrality: number; clusterId: number | null }>, edges: Array<{ source: string; target: string }>, clusterColors: Record<number, string>) {
  const graph = new Graph();
  for (const file of files) {
    const size = Math.max(4, Math.min(20, 4 + file.centrality * 24));
    const clusterColor = file.clusterId !== null ? clusterColors[file.clusterId] : '#64748b';
    graph.addNode(file.path, {
      id: file.path,
      label: file.path.split('/').slice(-2).join('/'),
      fullPath: file.path,
      clusterId: file.clusterId,
      size,
      color: clusterColor,
    });
  }

  for (const edge of edges) {
    if (graph.hasNode(edge.source) && graph.hasNode(edge.target)) {
      graph.addEdge(edge.source, edge.target, {
        label: 'imports',
        size: 1,
        color: 'rgba(148, 163, 184, 0.35)',
      });
    }
  }

  randomLayout(graph, 900);
  return graph;
}

export function buildSymbolGraph(data: SymbolsResponse, clusterMode: boolean) {
  const graph = new Graph();

  if (clusterMode && data.clusters.length > 0) {
    for (const cluster of data.clusters) {
      graph.addNode(`cluster-${cluster.clusterId}`, {
        id: `cluster-${cluster.clusterId}`,
        label: `Cluster ${cluster.clusterId} (${cluster.memberCount})`,
        entityType: 'cluster',
        size: Math.max(10, Math.min(40, 10 + cluster.memberCount * 0.5)),
        color: fallbackColors[cluster.clusterId % fallbackColors.length],
      } satisfies SymbolNodeMeta);
    }

    for (const sym of data.symbols) {
      if (sym.clusterId === null) {
        graph.addNode(String(sym.id), {
          id: String(sym.id),
          label: sym.name,
          entityType: sym.kind,
          size: 6,
          color: symbolKindColorMap[sym.kind] || '#64748b',
          filePath: sym.filePath,
          startLine: sym.startLine,
        } satisfies SymbolNodeMeta);
      }
    }

    const clusterEdges = new Map<string, number>();
    const symbolById = new Map(data.symbols.map((sym) => [sym.id, sym]));
    for (const edge of data.edges) {
      const src = symbolById.get(edge.sourceId);
      const tgt = symbolById.get(edge.targetId);
      if (!src || !tgt || src.clusterId == null || tgt.clusterId == null) continue;
      if (src.clusterId === tgt.clusterId) continue;
      const key = `cluster-${src.clusterId}||cluster-${tgt.clusterId}`;
      clusterEdges.set(key, (clusterEdges.get(key) || 0) + 1);
    }
    for (const [key, count] of clusterEdges) {
      const [srcNode, tgtNode] = key.split('||');
      if (graph.hasNode(srcNode) && graph.hasNode(tgtNode)) {
        try {
          graph.addEdge(srcNode, tgtNode, {
            label: `${count}`,
            size: Math.min(5, 1 + count * 0.2),
            color: 'rgba(148,163,184,0.4)',
          });
        } catch { /* duplicate edge */ }
      }
    }
  } else {
    for (const sym of data.symbols) {
      graph.addNode(String(sym.id), {
        id: String(sym.id),
        label: sym.name,
        entityType: sym.kind,
        size: 6,
        color: symbolKindColorMap[sym.kind] || '#64748b',
        filePath: sym.filePath,
        startLine: sym.startLine,
        clusterId: sym.clusterId,
      } satisfies SymbolNodeMeta);
    }
    for (const edge of data.edges) {
      const src = String(edge.sourceId);
      const tgt = String(edge.targetId);
      if (graph.hasNode(src) && graph.hasNode(tgt)) {
        graph.addEdge(src, tgt, {
          label: edge.edgeType,
          edgeType: edge.edgeType,
          size: 1,
          color: edgeTypeColorMap[edge.edge_type] || 'rgba(148, 163, 184, 0.3)',
        });
      }
    }
  }

  randomLayout(graph, 1000);
  return graph;
}

export function buildConnectionGraph(data: ConnectionsResponse) {
  const graph = new Graph();
  const docNodes = new Set<string>();

  for (const conn of data.connections) {
    const fromId = String(conn.from_doc_id);
    const toId = String(conn.to_doc_id);

    if (!docNodes.has(fromId)) {
      docNodes.add(fromId);
      graph.addNode(fromId, {
        id: fromId,
        label: conn.from_title || conn.from_path.split('/').pop() || fromId,
        entityType: 'document',
        size: 8,
        color: '#3b82f6',
        fullPath: conn.from_path,
      });
    }
    if (!docNodes.has(toId)) {
      docNodes.add(toId);
      graph.addNode(toId, {
        id: toId,
        label: conn.to_title || conn.to_path.split('/').pop() || toId,
        entityType: 'document',
        size: 8,
        color: '#3b82f6',
        fullPath: conn.to_path,
      });
    }

    graph.addEdge(fromId, toId, {
      label: conn.relationship_type,
      edgeType: conn.relationship_type,
      size: Math.max(1, Math.min(5, conn.strength * 5)),
      color: relationshipColorMap[conn.relationship_type] || 'rgba(148,163,184,0.4)',
    });
  }

  randomLayout(graph, 800);
  return graph;
}

export function getNodeMeta(entity?: GraphEntity) {
  if (!entity) return undefined;
  return {
    id: String(entity.id),
    label: entity.name,
    type: entity.type,
    description: entity.description,
    firstLearnedAt: entity.firstLearnedAt,
    lastConfirmedAt: entity.lastConfirmedAt,
    contradictedAt: entity.contradictedAt,
  };
}
