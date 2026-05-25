## Why

nano-brain stores 8 types of graph data but the web dashboard only exposes 2 (memory entities + file dependencies). 6 graph types are stored but invisible — code symbol call graphs, execution flows, document connections, infrastructure symbols, query behavior chains, and contradiction/lineage graphs. Users cannot visualize or explore the rich relationship data that nano-brain already computes and stores.

## What Changes

- Add 4 new REST API endpoints to expose hidden graph data:
  - `GET /api/v1/graph/symbols` — code symbol call graph with clusters
  - `GET /api/v1/graph/flows` — execution flow chains
  - `GET /api/v1/graph/connections` — document relationship network
  - `GET /api/v1/graph/infrastructure` — infrastructure symbols grouped by type
- Add 4 new web dashboard views:
  - `/web/symbols` — interactive symbol call graph with cluster-first rendering
  - `/web/flows` — execution flow list with step-by-step visualization
  - `/web/connections` — force-directed document connection graph
  - `/web/infrastructure` — grouped table of infrastructure symbols

## Capabilities

### New Capabilities

- `graph-symbols-api`: REST endpoint returning code symbols, edges, and Louvain clusters for a workspace
- `graph-flows-api`: REST endpoint returning execution flows with step details
- `graph-connections-api`: REST endpoint returning document connections with relationship metadata
- `graph-infrastructure-api`: REST endpoint returning infrastructure symbols grouped by type
- `symbol-graph-view`: Web view rendering symbol call graph with cluster-first approach for large graphs
- `flows-view`: Web view displaying execution flows as expandable step chains
- `connections-view`: Web view rendering document connections as force-directed graph
- `infrastructure-view`: Web view displaying infrastructure symbols as grouped filterable table

### Modified Capabilities

## Impact

- **Server**: 4 new route handlers in `src/server.ts`, potential new store methods in `src/store.ts`
- **Web UI**: 4 new view components, updates to `App.tsx` routing and `Layout.tsx` navigation
- **Dependencies**: No new dependencies — reuses existing Sigma.js, Graphology, Recharts
- **Performance**: Symbol graph requires cluster-first rendering for >500 nodes; flows/connections need pagination
- **Database**: Read-only queries against existing tables (code_symbols, symbol_edges, execution_flows, flow_steps, memory_connections, symbols)
