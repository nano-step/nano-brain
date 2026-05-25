import type { Store, MemoryConnection, MemoryConnectionRelationshipType } from './types.js';

export function isValidRelationshipType(type: string): type is MemoryConnectionRelationshipType {
  return (['supports', 'contradicts', 'extends', 'supersedes', 'related', 'caused_by', 'refines', 'implements'] as string[]).includes(type);
}

export interface TraversalNode {
  docId: number;
  depth: number;
  path: MemoryConnection[];
}

export function traverse(
  store: Store,
  startDocId: number,
  options?: { maxDepth?: number; relationshipTypes?: string[] }
): TraversalNode[] {
  const maxDepth = options?.maxDepth ?? 2;
  const visited = new Set<number>();
  const result: TraversalNode[] = [];
  const queue: TraversalNode[] = [{ docId: startDocId, depth: 0, path: [] }];
  visited.add(startDocId);

  while (queue.length > 0) {
    const current = queue.shift()!;
    if (current.depth > 0) result.push(current);
    if (current.depth >= maxDepth) continue;

    const connections = store.getConnectionsForDocument(current.docId, {
      relationshipType: options?.relationshipTypes?.length === 1 ? options.relationshipTypes[0] : undefined,
    });

    for (const conn of connections) {
      if (options?.relationshipTypes && options.relationshipTypes.length > 1) {
        if (!options.relationshipTypes.includes(conn.relationshipType)) continue;
      }
      const neighborId = conn.fromDocId === current.docId ? conn.toDocId : conn.fromDocId;
      if (visited.has(neighborId)) continue;
      visited.add(neighborId);
      queue.push({ docId: neighborId, depth: current.depth + 1, path: [...current.path, conn] });
    }
  }

  return result;
}

export function getRelatedDocuments(store: Store, docId: number, relationshipType?: string): number[] {
  const connections = store.getConnectionsForDocument(docId, { relationshipType });
  return connections.map(c => c.fromDocId === docId ? c.toDocId : c.fromDocId);
}
