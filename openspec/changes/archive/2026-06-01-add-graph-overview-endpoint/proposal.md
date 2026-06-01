## Why

/ui/graph panel is empty on load. User must manually type a symbol or document name before any graph renders. Backend has 33,849 edges across 3 edge types but no endpoint exposes "show me a default view of the graph".

Knowledge graph tools (Obsidian, Logseq, Roam) all auto-display the workspace graph on open. Users expect the same here.

## What Changes

- Add SQL query `ListTopGraphNodesByDegree :many` returning the top-N most-connected nodes for a workspace, filtered by edge type set, with their total degree count.
- Add SQL query `ListEdgesBetweenNodes :many` returning all edges where both source and target are in a given node ID set.
- Add HTTP handler `GraphOverview` that combines these two queries: top-N nodes + all edges between them.
- Register route `POST /api/v1/graph/overview` on the `data` group (no CSRF, server-side rendering of read-only data).
- Update frontend `GraphPanel.tsx`: when focus is empty, call /graph/overview instead of showing empty state. When focus is non-empty, existing /graph/neighborhood behavior unchanged.
- Update frontend `useGraphNeighborhood.ts` (rename internally) or add sibling `useGraphOverview.ts` for the new endpoint.
- Tests: handler test for response shape, top-N ordering, mode filtering (code vs knowledge).

## Capabilities

### New Capabilities
- `graph-overview-endpoint`: Defines `POST /api/v1/graph/overview` request/response contract. Returns nodes and edges for the workspace's most-connected subgraph, scoped by edge types per UI mode.

### Modified Capabilities
None — existing /graph/neighborhood unchanged.

## Impact

- **Code:** `internal/storage/queries/graph.sql` (2 new queries), `internal/server/handlers/graph_overview.go` (new file ~100 lines), `routes.go` (1 line), `web/src/panels/GraphPanel.tsx` (~30 lines for auto-fetch logic), `web/src/api/types.ts` (new types).
- **Behavior:** /ui/graph shows graph on load. User can still focus a specific node by typing.
- **Risk:** Low — additive endpoint. Existing graph behavior preserved.
- **Performance:** Top-50 nodes query bounded by index `idx_graph_edges_source` + `idx_graph_edges_target`. Expected < 50ms on 33K edges.

## Out of Scope

- Real-time graph updates (websocket) — defer
- Graph layout caching at server level — frontend caches positions already
- Interactive expand/collapse — separate UX issue
