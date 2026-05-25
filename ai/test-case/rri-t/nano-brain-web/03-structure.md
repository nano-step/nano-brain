# RRI-T Phase 3: STRUCTURE — nano-brain-web

## Test Cases (Q-A-R-P-T)

### D1: UI/UX (8 TCs)

#### WEB-001: All 8 routes render without crash
- **P:** P0 | **Persona:** End User
- **Steps:** Navigate to each of /dashboard, /graph, /code, /symbols, /flows, /connections, /infrastructure, /search
- **Expected:** Each view renders, no React error

#### WEB-002: Search debounce and result display
- **P:** P1 | **Persona:** End User
- **Steps:** Type "test" in search, wait 300ms
- **Expected:** Results appear with score bars, execution time shown

#### WEB-003: Graph node click shows NodeDetail
- **P:** P1 | **Persona:** End User
- **Steps:** Click a node on Knowledge Graph
- **Expected:** NodeDetail card appears with name, type, metadata

#### WEB-004: Flow list filter by type and search
- **P:** P1 | **Persona:** End User
- **Steps:** Select "intra_community" filter, type name substring
- **Expected:** Flow list filters correctly

#### WEB-005: Infrastructure tree expand/collapse
- **P:** P1 | **Persona:** End User
- **Steps:** Click type header → click pattern → view operations
- **Expected:** Sections expand/collapse, operations listed

#### WEB-006: Workspace selector changes all views
- **P:** P1 | **Persona:** End User
- **Steps:** Select different workspace from dropdown
- **Expected:** All queries refetch with workspace param

#### WEB-007: Sidebar navigation highlights active route
- **P:** P2 | **Persona:** End User
- **Steps:** Click each nav item
- **Expected:** Active item visually highlighted

#### WEB-008: Symbol graph cluster mode toggle
- **P:** P1 | **Persona:** End User
- **Steps:** Toggle cluster mode on/off
- **Expected:** Graph switches between cluster and individual views

### D2: API Contract (8 TCs)

#### WEB-009: /api/v1/status returns valid schema
- **P:** P0 | **Persona:** BA
- **Steps:** GET /api/v1/status
- **Expected:** { version, uptime, documents, embeddings, workspaces, primaryWorkspace }

#### WEB-010: /api/v1/search?q= validates query param
- **P:** P0 | **Persona:** QA
- **Steps:** 1) GET /api/v1/search (no q) 2) GET /api/v1/search?q=test
- **Expected:** 1) 400 + error 2) 200 + results array

#### WEB-011: /api/v1/connections validates docId
- **P:** P0 | **Persona:** QA
- **Steps:** 1) No docId → 400 2) Invalid docId → 404 3) Valid → 200
- **Expected:** Correct HTTP status codes

#### WEB-012: All graph endpoints return valid schemas
- **P:** P0 | **Persona:** BA
- **Steps:** GET each graph endpoint
- **Expected:** Each returns typed JSON matching response interfaces

#### WEB-013: Workspace param filters API responses
- **P:** P1 | **Persona:** End User
- **Steps:** GET /api/v1/search?q=test&workspace=hash1
- **Expected:** Results scoped to workspace

#### WEB-014: Telemetry endpoint returns stats
- **P:** P1 | **Persona:** BA
- **Steps:** GET /api/v1/telemetry
- **Expected:** { queryCount, banditStats, preferenceWeights, expandRate, importanceStats }

#### WEB-015: CORS preflight returns 204
- **P:** P0 | **Persona:** DevOps
- **Steps:** OPTIONS /api/v1/status with Origin header
- **Expected:** 204 with correct CORS headers

#### WEB-016: 404 for unknown API routes
- **P:** P1 | **Persona:** QA
- **Steps:** GET /api/v1/nonexistent
- **Expected:** 404

### D3: Performance (4 TCs)

#### WEB-017: Graph rendering with 500+ nodes
- **P:** P1 | **Persona:** QA
- **Steps:** Build entity graph with 500 nodes
- **Expected:** ForceAtlas2 completes within 5s, no freeze

#### WEB-018: Search response time < 500ms
- **P:** P1 | **Persona:** DevOps
- **Steps:** Search with common keyword on 500-doc index
- **Expected:** executionMs < 500ms

#### WEB-019: Connection graph node limit (500)
- **P:** P2 | **Persona:** QA
- **Steps:** Pass 1000 connections to buildConnectionGraph
- **Expected:** Graph limited to 500 nodes (greedy selection)

#### WEB-020: React Query caching (5min staleTime)
- **P:** P2 | **Persona:** DevOps
- **Steps:** Fetch status, navigate away, return within 5min
- **Expected:** No refetch, cached data shown

### D4: Security (5 TCs)

#### WEB-021: XSS in search query is escaped
- **P:** P0 | **Persona:** Security
- **Steps:** Search for `<script>alert(1)</script>`
- **Expected:** Rendered as text, no script execution

#### WEB-022: XSS in graph node labels
- **P:** P0 | **Persona:** Security
- **Steps:** Entity with name `<img onerror=alert(1)>` in graph
- **Expected:** Rendered as text in Sigma canvas (safe by default)

#### WEB-023: CORS rejects non-localhost origins
- **P:** P0 | **Persona:** Security
- **Steps:** Request with Origin: https://evil.com
- **Expected:** No Access-Control-Allow-Origin header

#### WEB-024: API errors don't leak stack traces
- **P:** P1 | **Persona:** Security
- **Steps:** Trigger 500 error
- **Expected:** Generic error message, no stack trace in response

#### WEB-025: API request logging doesn't expose sensitive data
- **P:** P1 | **Persona:** Security
- **Steps:** Review client.ts requestJson logging
- **Expected:** Only logs URL, status, timing — no body content

### D5: Data Integrity (4 TCs)

#### WEB-026: Dashboard counts match actual index
- **P:** P0 | **Persona:** BA
- **Steps:** Compare dashboard doc count with getIndexHealth()
- **Expected:** Counts match exactly

#### WEB-027: Graph entity data matches API response
- **P:** P1 | **Persona:** BA
- **Steps:** Compare graph nodes with API entity data
- **Expected:** Node count, names, types all match

#### WEB-028: Search results match backend FTS
- **P:** P1 | **Persona:** BA
- **Steps:** Same query via API and direct store.searchFTS
- **Expected:** Same results in same order

#### WEB-029: Telemetry bandit stats match store
- **P:** P2 | **Persona:** BA
- **Steps:** Compare telemetry endpoint with store.loadBanditStats
- **Expected:** Stats match

### D6: Infrastructure (5 TCs)

#### WEB-030: Web build produces valid static assets
- **P:** P0 | **Persona:** DevOps
- **Steps:** Run npm run build:web, check dist/web/
- **Expected:** index.html + assets/ exist, no build errors

#### WEB-031: SPA fallback serves index.html for all routes
- **P:** P1 | **Persona:** DevOps
- **Steps:** Direct GET /web/graph (not from SPA navigation)
- **Expected:** Returns index.html, React Router handles route

#### WEB-032: API server handles concurrent requests
- **P:** P1 | **Persona:** DevOps
- **Steps:** 10 concurrent API requests
- **Expected:** All return valid JSON, no crashes

#### WEB-033: Server graceful shutdown
- **P:** P2 | **Persona:** DevOps
- **Steps:** Stop server while requests in-flight
- **Expected:** In-flight requests complete or get clean error

#### WEB-034: React Query retries on failure
- **P:** P2 | **Persona:** DevOps
- **Steps:** API returns 500 once
- **Expected:** React Query retries 1 time (config: retry: 1)

### D7: Edge Cases (6 TCs)

#### WEB-035: Empty data renders gracefully
- **P:** P0 | **Persona:** QA
- **Steps:** API returns empty arrays for all endpoints
- **Expected:** "No data" messages, no crashes

#### WEB-036: Graph with 0 or 1 node
- **P:** P1 | **Persona:** QA
- **Steps:** Build graph with 0 nodes, then 1 node
- **Expected:** Shows empty state or single node, no layout error

#### WEB-037: Search with <2 characters
- **P:** P1 | **Persona:** QA
- **Steps:** Type "a" in search
- **Expected:** Query disabled, shows "Type at least 2 characters"

#### WEB-038: Flow with 0 steps
- **P:** P1 | **Persona:** QA
- **Steps:** API returns flow with empty steps array
- **Expected:** Flow card shows, detail shows empty chain

#### WEB-039: Connection strength edge cases (0, negative, very large)
- **P:** P2 | **Persona:** QA
- **Steps:** Connections with strength 0, -1, 999
- **Expected:** Edge rendered (clamped sizing), no crash

#### WEB-040: Very long node labels in graph
- **P:** P2 | **Persona:** QA
- **Steps:** Entity with 500-char name
- **Expected:** Label truncated or wrapped, no overflow
