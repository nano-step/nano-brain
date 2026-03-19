# Tasks: nano-brain Web Dashboard

## Phase 1: REST API (2-4h)
- [ ] Add REST API endpoints to src/server.ts (/api/v1/status, /api/v1/workspaces, /api/v1/graph/entities, /api/v1/graph/stats, /api/v1/code/dependencies, /api/v1/search, /api/v1/telemetry)
- [ ] Add CORS middleware for localhost origins
- [ ] Add static file serving for /web/* route
- [ ] Add tests for REST API endpoints

## Phase 2: Web UI Scaffold (1-2h)
- [ ] Initialize Vite + React + TypeScript project in src/web/
- [ ] Configure Tailwind v4 with dark theme
- [ ] Set up routing (react-router-dom)
- [ ] Create API client service (fetch wrapper with React Query)
- [ ] Create Zustand store for app state

## Phase 3: Status Dashboard (2-4h)
- [ ] Build StatusDashboard component with health metrics
- [ ] Add learning metrics charts (Recharts): bandit stats, preference weights, expand rate
- [ ] Add workspace selector
- [ ] Wire to /api/v1/status and /api/v1/telemetry

## Phase 4: Knowledge Graph Explorer (1-2d)
- [ ] Install Sigma.js + Graphology
- [ ] Build GraphCanvas component (Sigma.js WebGL wrapper)
- [ ] Build useSigma hook (ForceAtlas2 Web Worker, adaptive params)
- [ ] Build graph adapter (entities → Graphology nodes/edges)
- [ ] Add node type coloring, click-to-expand, detail panel
- [ ] Add cluster-first view for large graphs
- [ ] Wire to /api/v1/graph/entities

## Phase 5: Code Dependency Graph (1d)
- [ ] Reuse GraphCanvas component with code dependency data
- [ ] Add centrality-based node sizing
- [ ] Add cluster coloring
- [ ] Wire to /api/v1/code/dependencies

## Phase 6: Search Interface (2-4h)
- [ ] Build SearchInterface component with input + results list
- [ ] Add result cards with score, snippet, tags
- [ ] Add document detail view
- [ ] Wire to /api/v1/search

## Phase 7: Build & Integration (1-2h)
- [ ] Configure Vite build output to dist/web/
- [ ] Add npm script: "build:web" 
- [ ] Serve built files from HTTP server at /web/*
- [ ] Add to package.json files list for npm publish
- [ ] Test end-to-end: start server, open browser, verify all views
