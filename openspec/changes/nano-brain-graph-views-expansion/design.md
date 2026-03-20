## Context

nano-brain's web dashboard currently exposes 2 graph views (Knowledge Graph for memory entities, Code Dependencies for file relationships). The database stores 6 additional graph types that are computed but not visualized: code symbol call graphs, execution flows, document connections, infrastructure symbols, query behavior chains, and contradiction/lineage graphs.

The existing REST API pattern (`/api/v1/*`) and web architecture (React + Sigma.js + Tailwind) provide a proven foundation. This design extends that pattern to expose 4 high-value graph types.

**Constraints:**
- Must not impact existing dashboard performance
- Symbol graphs can have 5k+ nodes — requires cluster-first rendering
- Reuse existing GraphCanvas component and Sigma.js setup
- No new npm dependencies

## Goals / Non-Goals

**Goals:**
- Expose symbol call graphs, execution flows, document connections, and infrastructure symbols via REST API
- Render each graph type with appropriate visualization (graph, list, table)
- Handle large datasets (5k+ symbols) with cluster-first approach
- Maintain 30+ FPS rendering for symbol graphs

**Non-Goals:**
- Query behavior Markov chain visualization (v2)
- Contradiction/lineage graph visualization (v2)
- Real-time WebSocket updates (v2)
- Graph editing or mutation (read-only views)

## Decisions

### 1. REST API Response Shapes

**Decision:** Return denormalized JSON with all data needed for rendering in a single request.

**Rationale:** Avoids N+1 queries from the client. The web dashboard can render immediately without follow-up fetches.

**Alternatives considered:**
- GraphQL: More flexible but adds complexity; not worth it for 4 fixed endpoints
- Paginated REST: Would require multiple round-trips for graph rendering

**Endpoints:**

```
GET /api/v1/graph/symbols?workspace=<hash>
Response: {
  symbols: [{id, name, kind, filePath, startLine, endLine, exported, clusterId}],
  edges: [{sourceId, targetId, edgeType, confidence}],
  clusters: [{id, memberCount}]
}

GET /api/v1/graph/flows?workspace=<hash>
Response: {
  flows: [{id, label, flowType, entrySymbol, terminalSymbol, stepCount,
           steps: [{symbolId, symbolName, filePath, stepIndex}]}]
}

GET /api/v1/graph/connections?workspace=<hash>
Response: {
  connections: [{id,
    fromDoc: {id, title, path},
    toDoc: {id, title, path},
    relationshipType, strength, description, createdAt}]
}

GET /api/v1/graph/infrastructure?workspace=<hash>
Response: {
  symbols: [{type, pattern, operation, repo, filePath, lineNumber}],
  grouped: {
    redis_key: [{pattern, operations: [{op, repo, file}]}],
    mysql_table: [...],
    ...
  }
}
```

### 2. Symbol Graph Rendering Strategy

**Decision:** Cluster-first view for graphs with >500 nodes. Show Louvain clusters as super-nodes, expand on click.

**Rationale:** Sigma.js handles 5k nodes but becomes unusable for exploration. Cluster-first provides overview + detail.

**Alternatives considered:**
- Force-directed with all nodes: Too slow and cluttered for large graphs
- Server-side filtering: Loses context; user can't see full graph structure
- Virtual viewport: Complex to implement; cluster-first is simpler and more useful

**Implementation:**
- If `symbols.length > 500`: render clusters as super-nodes (size = memberCount)
- Click cluster → replace super-node with individual symbols (scoped subgraph)
- Node colors by kind: function=blue, class=green, method=cyan, interface=purple
- Edge colors by type: CALLS=gray, INHERITS=orange, IMPLEMENTS=teal

### 3. Execution Flows Visualization

**Decision:** List + detail layout with horizontal step chain.

**Rationale:** Flows are sequential by nature. A list allows filtering; the detail view shows the call chain clearly.

**Alternatives considered:**
- Sankey diagram: Good for aggregated flows but overkill for individual flow inspection
- Vertical timeline: Takes too much vertical space
- Graph view: Flows are linear; graph adds visual noise

**Implementation:**
- List shows: entry→terminal label, flow_type badge, step count
- Click flow → expand to show step chain (horizontal boxes with arrows)
- Filter by: flow_type, entry symbol, file path
- Paginate: 20 flows per page, lazy-load steps on expand

### 4. Document Connections Visualization

**Decision:** Force-directed graph using existing GraphCanvas/Sigma.js.

**Rationale:** Document connections form a network; force-directed layout reveals clusters and hubs naturally.

**Implementation:**
- Nodes = documents (title as label)
- Edges = connections (colored by relationship_type)
- Edge colors: supports=green, contradicts=red, extends=blue, supersedes=orange, related=gray, caused_by=yellow, refines=purple, implements=teal
- Edge thickness by strength (0.0-1.0 → 1-5px)
- Limit to 500 nodes max; paginate if more

### 5. Infrastructure Symbols Visualization

**Decision:** Grouped table view instead of graph.

**Rationale:** Infrastructure symbols are categorical (type × operation × repo). A matrix/table is more useful than a graph for this data.

**Alternatives considered:**
- Graph view: Symbols don't have natural edges; would be disconnected nodes
- Heatmap: Good for density but loses detail

**Implementation:**
- Group by symbol type (redis_key, mysql_table, api_endpoint, etc.)
- Columns: pattern, operations (read/write/define badges), repos, file count
- Click pattern → expand to show all files + line numbers
- Filter by: type, repo, operation
- Virtual scrolling for large lists

## Risks / Trade-offs

**[Risk] Large symbol graphs cause browser freeze**
→ Mitigation: Cluster-first rendering caps visible nodes at ~50 clusters. Expand only on user action.

**[Risk] Slow API responses for large workspaces**
→ Mitigation: Add database indexes on workspace_hash columns if missing. Consider response caching.

**[Risk] Inconsistent UX across 4 new views**
→ Mitigation: Reuse existing components (GraphCanvas, FilterBar, DetailPanel). Follow existing color/layout patterns.

**[Trade-off] Denormalized API responses increase payload size**
→ Accepted: Simplifies client code and reduces round-trips. Payload size is acceptable for graph data.

**[Trade-off] No real-time updates**
→ Accepted: Deferred to v2. Current use case is exploration, not monitoring.
