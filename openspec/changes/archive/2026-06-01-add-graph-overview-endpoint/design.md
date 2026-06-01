## Context

GraphPanel.tsx has Code (symbol-focused, edges: calls/imports/contains) and Knowledge (doc-focused, edge: references) tabs. Both currently require manual focus input.

Backend has `POST /api/v1/graph/neighborhood` which does BFS from a focus node. Useful but needs a starting point. Missing: "show me the most interesting subgraph" entry point.

The `graph_edges` table has indexes on `(workspace_hash, source_node)` and `(workspace_hash, target_node)`. We can efficiently compute degree per node by counting both directions and aggregating.

## Goals / Non-Goals

**Goals:**
- /ui/graph displays graph on load (no input required)
- Code mode: top symbols by call+import+contains degree
- Knowledge mode: top documents by reference degree
- Reuse existing GraphNode/GraphEdge frontend types — same shape as neighborhood response
- < 100ms response time on workspaces with 100K+ edges

**Non-Goals:**
- Server-side layout (frontend handles via Sigma.js)
- Pagination (top-N is enough for initial view)
- Editing graph data
- Custom edge type weighting

## Decisions

### D1: Endpoint path + method

**Decision:** `POST /api/v1/graph/overview` on the `data` group (read-only, no CSRF needed).

**Rationale:** Mirrors `/graph/neighborhood` pattern. POST allows complex request body (edge_types array, mode enum, limit) without URL parameter encoding gymnastics.

### D2: Request shape

```json
{
  "workspace": "<hash>",
  "mode": "code" | "knowledge",
  "limit": 50,
  "edge_types": ["calls", "imports", "contains"]
}
```

`mode` is optional — when provided, it auto-fills `edge_types`. Explicit `edge_types` overrides mode default.

**Rationale:** Frontend already uses `NodeKind` ("symbol" or "doc") + edge type filters. Mode is the high-level switch; edge_types is the fine-grained filter.

### D3: Top-N selection algorithm

```sql
WITH degrees AS (
    SELECT source_node AS node, COUNT(*) AS deg
    FROM graph_edges
    WHERE workspace_hash = $1 AND edge_type = ANY($2::text[])
    GROUP BY source_node
    UNION ALL
    SELECT target_node AS node, COUNT(*) AS deg
    FROM graph_edges
    WHERE workspace_hash = $1 AND edge_type = ANY($2::text[])
    GROUP BY target_node
),
agg AS (
    SELECT node, SUM(deg) AS total_deg
    FROM degrees
    GROUP BY node
)
SELECT node, total_deg FROM agg ORDER BY total_deg DESC LIMIT $3;
```

**Rationale:** UNION ALL counts incoming + outgoing. GROUP BY collapses to one row per node with total degree. ORDER BY DESC LIMIT N. Single query, no N+1.

### D4: Edge selection — only between top-N nodes

```sql
SELECT * FROM graph_edges
WHERE workspace_hash = $1
  AND edge_type = ANY($2::text[])
  AND source_node = ANY($3::text[])
  AND target_node = ANY($3::text[]);
```

**Rationale:** Avoid edge explosion — only edges where both endpoints are in the top-N set. Keeps frontend rendering bounded.

### D5: Response shape — reuse GraphNeighborhoodResponse

Same fields: `nodes`, `edges`, `truncated`, `frontier_count`. Frontend doesn't need a new type.

**Rationale:** Minimize frontend changes. `truncated=true` if more than `limit` nodes existed in workspace.

### D6: Frontend auto-fetch trigger

```tsx
useEffect(() => {
    if (state.focus) return // existing neighborhood call still triggers
    fetchOverview({ mode, edge_types: state.edgeTypes, limit: 50 })
}, [mode, state.edgeTypes, workspace])
```

When user types focus, neighborhood call fires (existing behavior). When focus cleared, overview re-fires.

**Rationale:** Smooth UX — no manual button, just type or clear.

### D7: Default mode

**Decision:** Default to current `mode` state in GraphPanel (already persists in `usePositionCache`). On first visit: "symbol" (Code) — matches existing default.

## Risks / Trade-offs

| Risk | Mitigation |
|---|---|
| Top-N excludes important isolated subgraphs | Acceptable v1. User can type focus to drill in. |
| Heavy edge count for highly-connected nodes | LIMIT 50 nodes + filter edges to top-N only → bounded edge count |
| Knowledge mode has few references → empty graph | Show clear empty state if 0 nodes returned |
| Race between overview and neighborhood fetches | useEffect deps + isPending state already handle this |
