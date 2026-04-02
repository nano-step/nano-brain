# RRI-T Phase 1: PREPARE — nano-brain-web

| Field | Value |
|-------|-------|
| Feature | nano-brain Web Dashboard + REST API v1 |
| Date | 2026-03-29 |
| Stack | React 18 + TypeScript + Vite + TailwindCSS + Sigma/Graphology + Recharts |
| Dimensions | All 7 |
| Personas | All 5 |

## System Under Test

### 8 Web Views
| View | Route | Key Components |
|------|-------|---------------|
| Dashboard | `/dashboard` | Stat cards, bandit chart, expand rate, preference weights |
| Knowledge Graph | `/graph` | Sigma canvas, entity nodes, relationship edges |
| Code Dependencies | `/code` | File-level graph, centrality sizing, cluster coloring |
| Symbol Graph | `/symbols` | Function/class call graph, cluster mode toggle |
| Execution Flows | `/flows` | Flow list + detail, type/name filters |
| Connections | `/connections` | Document relationship network, strength edges |
| Infrastructure | `/infrastructure` | Expandable type→pattern→operation tree |
| Search | `/search` | Debounced input, result cards, score bars |

### 13 REST API v1 Endpoints
| # | Endpoint | Method |
|---|----------|--------|
| 1 | `/api/v1/status` | GET |
| 2 | `/api/v1/workspaces` | GET |
| 3 | `/api/v1/graph/entities` | GET |
| 4 | `/api/v1/graph/stats` | GET |
| 5 | `/api/v1/code/dependencies` | GET |
| 6 | `/api/v1/graph/symbols` | GET |
| 7 | `/api/v1/graph/flows` | GET |
| 8 | `/api/v1/graph/connections` | GET |
| 9 | `/api/v1/graph/infrastructure` | GET |
| 10 | `/api/v1/search?q=` | GET |
| 11 | `/api/v1/telemetry` | GET |
| 12 | `/api/v1/connections?docId=` | GET |
| 13 | `/health` | GET |

### Shared Components
| Component | Purpose |
|-----------|---------|
| Layout | Sidebar nav + header + workspace selector |
| GraphCanvas | Sigma + ForceAtlas2 graph renderer |
| NodeDetail | Selected node info card |
| SearchResult | Expandable search result with score bar |

### Existing Test Baseline
- `test/rest-api.test.ts`: 23 tests covering all 13 API endpoints
- No frontend component tests exist
