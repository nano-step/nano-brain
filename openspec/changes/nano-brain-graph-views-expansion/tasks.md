## 1. REST API Endpoints

- [ ] 1.1 Add GET /api/v1/graph/symbols endpoint to server.ts — query code_symbols + symbol_edges tables, return JSON with symbols, edges, clusters
- [ ] 1.2 Add GET /api/v1/graph/flows endpoint to server.ts — query execution_flows + flow_steps + code_symbols, return JSON with flows and step details
- [ ] 1.3 Add GET /api/v1/graph/connections endpoint to server.ts — query memory_connections + documents, return JSON with connection pairs and metadata
- [ ] 1.4 Add GET /api/v1/graph/infrastructure endpoint to server.ts — query symbols table, return JSON grouped by type with operations
- [ ] 1.5 Add store methods if missing: getSymbolsForProject(), getSymbolEdgesForProject(), getFlowsForProject(), getFlowSteps(), getConnectionsForProject(), getInfrastructureSymbols()
- [ ] 1.6 Add tests for all 4 new endpoints in test/rest-api.test.ts

## 2. Symbol Call Graph View

- [ ] 2.1 Create src/web/src/views/SymbolGraph.tsx — Sigma.js graph with cluster-first view
- [ ] 2.2 Add symbol graph adapter in src/web/src/lib/graph-adapter.ts — buildSymbolGraph() function
- [ ] 2.3 Add symbol kind color map in src/web/src/lib/colors.ts (function=blue, class=green, method=cyan, interface=purple)
- [ ] 2.4 Add cluster super-node logic — aggregate symbols by cluster_id, show as single large node when >500 symbols
- [ ] 2.5 Add click-to-expand cluster — replace super-node with individual symbols
- [ ] 2.6 Add symbol detail panel — name, kind, file:line, callers, callees
- [ ] 2.7 Add route /symbols to App.tsx and nav item to Layout.tsx
- [ ] 2.8 Wire SymbolGraph.tsx to /api/v1/graph/symbols endpoint

## 3. Execution Flows View

- [ ] 3.1 Create src/web/src/views/FlowsView.tsx — list + detail layout
- [ ] 3.2 Build flow list component — entry→terminal label, flow_type badge, step count
- [ ] 3.3 Build flow detail component — horizontal step chain with symbol boxes and arrows
- [ ] 3.4 Add filter controls — flow_type dropdown, search by symbol name, file path filter
- [ ] 3.5 Add pagination — 20 flows per page, lazy-load steps on expand
- [ ] 3.6 Add route /flows to App.tsx and nav item to Layout.tsx
- [ ] 3.7 Wire FlowsView.tsx to /api/v1/graph/flows endpoint

## 4. Document Connections View

- [ ] 4.1 Create src/web/src/views/ConnectionsView.tsx — Sigma.js force-directed graph
- [ ] 4.2 Add connection graph adapter in graph-adapter.ts — buildConnectionGraph() function
- [ ] 4.3 Add relationship type color map in colors.ts (supports=green, contradicts=red, extends=blue, etc.)
- [ ] 4.4 Add edge thickness by strength (0.0-1.0 → 1-5px)
- [ ] 4.5 Add connection detail panel — document title, path, relationship list
- [ ] 4.6 Add 500 node limit with pagination indicator
- [ ] 4.7 Add route /connections to App.tsx and nav item to Layout.tsx
- [ ] 4.8 Wire ConnectionsView.tsx to /api/v1/graph/connections endpoint

## 5. Infrastructure Symbols View

- [ ] 5.1 Create src/web/src/views/InfrastructureView.tsx — grouped table layout
- [ ] 5.2 Build symbol group component — collapsible sections by type
- [ ] 5.3 Build symbol detail row — pattern, operations badges, repo list, file count
- [ ] 5.4 Build expanded detail — file paths with line numbers
- [ ] 5.5 Add filter controls — type dropdown, repo filter, operation filter
- [ ] 5.6 Add virtual scrolling for large lists
- [ ] 5.7 Add route /infrastructure to App.tsx and nav item to Layout.tsx
- [ ] 5.8 Wire InfrastructureView.tsx to /api/v1/graph/infrastructure endpoint

## 6. Build & Integration

- [ ] 6.1 Rebuild web UI (npm run build:web)
- [ ] 6.2 Test all 4 new views in browser
- [ ] 6.3 Verify existing 4 views still work (Dashboard, Knowledge Graph, Code Dependencies, Search)
- [ ] 6.4 Run full test suite
- [ ] 6.5 Update README with new view documentation
