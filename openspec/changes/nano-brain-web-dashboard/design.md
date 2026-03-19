# Design: nano-brain Web Dashboard

## Architecture
Embedded in nano-brain as `src/web/` directory. Shares TypeScript types. Built with Vite, served as static files from the existing HTTP server.

### Tech Stack
- React 18 + TypeScript
- Vite 6 (build tool)
- Tailwind CSS v4 (styling, dark mode)
- Sigma.js + Graphology (graph visualization, WebGL)
- Recharts (dashboard charts)
- Zustand (state management)
- @tanstack/react-query (data fetching + caching)

### Data Access
REST API endpoints alongside MCP (same store methods, JSON output):
- GET /api/v1/graph/entities — all entities with edges
- GET /api/v1/graph/stats — graph statistics
- GET /api/v1/code/dependencies — file dependency graph
- GET /api/v1/code/symbols — symbol graph with edges
- GET /api/v1/search — hybrid search (JSON results)
- GET /api/v1/telemetry — learning stats (bandits, preferences, importance)
- GET /api/v1/status — system health
- GET /api/v1/workspaces — list workspaces

### Graph Rendering
- Sigma.js WebGL renderer for 10k+ node graphs
- ForceAtlas2 layout via Web Worker (non-blocking)
- Adaptive parameters by graph size (small/medium/large)
- Cluster-first view: show Louvain clusters, expand on click
- Node reducer pattern for dynamic styling per frame

### Component Architecture
```
App
├── Header (workspace selector, nav)
├── Sidebar (navigation)
└── Main Content (route-based)
    ├── /dashboard — StatusDashboard
    ├── /graph — KnowledgeGraphExplorer
    ├── /code — CodeDependencyGraph
    └── /search — SearchInterface
```

### Performance
- < 500 nodes: render all, force-directed layout
- 500-2000: show clusters, expand on click
- > 2000: paginate, show top clusters
- Web Workers for layout computation
- React Query caching (5min stale time)

### Risks & Mitigations
- CORS: serve from same origin (localhost:3100/web/)
- Bundle size: ~200KB gzipped (acceptable for dev tool)
- Memory leaks: proper Sigma.js disposal on unmount
- Large graphs: cluster-first + lazy edge loading
