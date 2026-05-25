# Proposal: nano-brain Web Dashboard

## Why

nano-brain has rich data (7000+ documents, 1000+ entities, 10000+ edges, learning telemetry, code intelligence) but no way to visualize it. Users can only interact via MCP tools that return text, with no way to see the knowledge graph structure, explore entity relationships visually, or monitor learning system health.

## What Changes

- Add REST API endpoints returning JSON (alongside existing MCP tools)
- Add knowledge graph explorer (entities + edges, cluster coloring, click-to-expand)
- Add status/learning dashboard (health, telemetry, bandit stats, preference weights)
- Add code dependency graph (files + imports, centrality highlighting)
- Add search interface (hybrid search with visual results)
- Add static file serving from existing HTTP server at /web/*
- Dark mode UI throughout

**Out of Scope (v2):** Execution flow visualization, document connections graph, cross-repo symbols view, real-time WebSocket updates, AI chat integration, PWA/offline support.

## Capabilities

### New Capabilities
- `rest-api`: REST API endpoints for status, workspaces, graph entities, graph stats, code dependencies, search, and telemetry
- `web-ui`: Interactive web dashboard with status dashboard, knowledge graph explorer, code dependency graph, and search interface

### Modified Capabilities
(none)

## Impact

- **Code**: New `src/web/` directory for React frontend, new REST routes in `src/server.ts`
- **APIs**: New REST endpoints at `/api/v1/*`, static file serving at `/web/*`
- **Dependencies**: React, Vite, Tailwind, Sigma.js, Graphology, Recharts, Zustand, React Query (dev dependencies for web build)
- **Systems**: HTTP server extended to serve static files and REST API

## Success Criteria
- Dashboard accessible at http://localhost:3100/web/
- Knowledge graph renders 1000+ entities at 30+ FPS
- Status dashboard shows all learning metrics
- Search returns results in <500ms
- Zero impact on existing MCP tool performance
