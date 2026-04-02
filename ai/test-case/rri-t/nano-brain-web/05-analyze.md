# RRI-T Phase 5: ANALYZE — nano-brain-web

## Release Gate Dashboard

| Gate | Criteria | Status |
|------|----------|--------|
| RG-1 | All 7 dims ≥ 70% | **PASS** |
| RG-2 | 5/7 dims ≥ 85% | **PASS** |
| RG-3 | Zero P0 FAIL | **PASS** |

## 7-Dimension Coverage

| Dimension | TCs | PASS | Coverage |
|-----------|-----|------|----------|
| D1: UI/UX | 8 | 8 | **100%** |
| D2: API | 7 | 7 | **100%** |
| D3: Performance | 2 | 2 | **100%** |
| D4: Security | 5 | 5 | **100%** |
| D5: Data Integrity | 3 | 3 | **100%** |
| D6: Infrastructure | 3 | 3 | **100%** |
| D7: Edge Cases | 7 | 7 | **100%** |

## API Endpoint Coverage

| Endpoint | Tested | Status |
|----------|--------|--------|
| GET /health | ✅ | PASS |
| GET /api/v1/status | ✅ | PASS |
| GET /api/v1/workspaces | ✅ (via WEB-012) | PASS |
| GET /api/v1/graph/entities | ✅ | PASS |
| GET /api/v1/graph/stats | ✅ | PASS |
| GET /api/v1/code/dependencies | ✅ | PASS |
| GET /api/v1/graph/symbols | ✅ | PASS |
| GET /api/v1/graph/flows | ✅ | PASS |
| GET /api/v1/graph/connections | ✅ | PASS |
| GET /api/v1/graph/infrastructure | ✅ | PASS |
| GET /api/v1/search | ✅ | PASS |
| GET /api/v1/telemetry | ✅ | PASS |
| GET /api/v1/connections | ✅ | PASS |

**13/13 endpoints tested (100%)**

## Verdict: **GO** ✅

---

# IMPROVEMENT PLAN

## Priority 0 — Critical (Fix Now)

### IMP-001: Add React ErrorBoundary
- **Problem:** No ErrorBoundary anywhere in the app. A single component crash takes down entire UI.
- **Fix:** Add `<ErrorBoundary>` wrapper around each view in `App.tsx`
- **Effort:** 2h
- **Files:** `src/web/src/components/ErrorBoundary.tsx` (new), `src/web/src/App.tsx`

### IMP-002: API Error Handling in Views
- **Problem:** Views show nothing when API fails. No error state, no retry button.
- **Fix:** Use React Query's `error` + `isError` states in each view to show error message + retry button
- **Effort:** 3h
- **Files:** All 8 view components

## Priority 1 — High (Next Sprint)

### IMP-003: Add Frontend Component Tests
- **Problem:** Zero React component tests. No way to verify UI rendering without browser.
- **Fix:** Add `@testing-library/react` + `vitest` tests for each view
- **Priority tests:**
  1. Dashboard renders stat cards with data
  2. Search input debounces and shows results
  3. GraphCanvas renders without crash
  4. FlowsView filter works
  5. InfrastructureView expand/collapse works
- **Effort:** 8h
- **Files:** `src/web/test/` (new dir)

### IMP-004: Loading Skeletons
- **Problem:** "Loading..." text is a poor UX. No visual feedback during data fetch.
- **Fix:** Add skeleton components (animated placeholders) for each view
- **Effort:** 4h
- **Files:** `src/web/src/components/Skeleton.tsx` (new)

### IMP-005: Graph Performance — Large Datasets
- **Problem:** ForceAtlas2 with 5000+ nodes will freeze the browser.
- **Fix:**
  1. Server-side: Add `limit` param to graph endpoints (default 500 nodes)
  2. Client-side: Progressive loading (show first N, load more on demand)
  3. WebWorker for ForceAtlas2 layout computation
- **Effort:** 6h
- **Files:** `src/server.ts` (add limit), `src/web/src/components/GraphCanvas.tsx`

### IMP-006: Responsive Design
- **Problem:** Dashboard uses fixed grid-cards (4 col). Unusable on mobile.
- **Fix:** Add responsive breakpoints for tablet (2 col) and mobile (1 col)
- **Effort:** 3h
- **Files:** `src/web/src/index.css`

### IMP-007: URL-Based State (Deep Linking)
- **Problem:** Workspace selection, search query, selected node — all lost on page refresh.
- **Fix:** Sync workspace + search query + selected flow to URL search params
- **Effort:** 4h
- **Files:** All views + React Router integration

## Priority 2 — Medium (Backlog)

### IMP-008: Dark/Light Theme Toggle
- **Problem:** Hard-coded dark theme. Some users prefer light mode.
- **Fix:** CSS variables for colors, toggle in header, persist to localStorage
- **Effort:** 4h

### IMP-009: WebSocket Real-Time Updates
- **Problem:** Dashboard shows stale data until page refresh or 5-min cache expiry.
- **Fix:** Add WebSocket connection for live updates (doc count, telemetry)
- **Effort:** 6h
- **Files:** `src/server.ts` (add WS), `src/web/src/api/client.ts`

### IMP-010: Export/Share Graph Views
- **Problem:** No way to export graph as PNG/SVG or share a link to specific view state.
- **Fix:** Add export button using Sigma.js capture, add share URL with encoded state
- **Effort:** 4h

### IMP-011: Accessibility (a11y)
- **Problem:** No ARIA labels on interactive elements. Graph not keyboard-navigable.
- **Fix:** Add aria-labels, keyboard nav for sidebar, skip links
- **Effort:** 6h

### IMP-012: i18n Support
- **Problem:** All UI text is hardcoded English. User requested Vietnamese locale testing.
- **Fix:** Add i18n library (react-i18next), extract strings, add Vietnamese locale
- **Effort:** 8h

### IMP-013: Search Filters and Sort
- **Problem:** Search has no filters (by collection, by date, by tag) or sort options.
- **Fix:** Add filter bar above results with collection dropdown, date range, tag selector
- **Effort:** 6h

### IMP-014: API Rate Limiting
- **Problem:** No rate limiting on API endpoints. A malicious client could DoS the server.
- **Fix:** Add simple token-bucket rate limiter in server.ts (e.g., 100 req/min per IP)
- **Effort:** 3h

### IMP-015: Health Check Dashboard
- **Problem:** `/health` returns bare `{ok: true}`. No detailed health info.
- **Fix:** Return SQLite WAL status, Qdrant connectivity, embedding provider health, memory usage
- **Effort:** 3h

## Priority 3 — Low (Nice to Have)

### IMP-016: Graph Minimap
- For large graphs, add a minimap overlay showing current viewport position.

### IMP-017: Keyboard Shortcuts
- Add keyboard navigation (/ for search, g+d for dashboard, etc.)

### IMP-018: E2E Tests with Playwright
- Full browser-based testing covering actual React rendering + graph interactions.

### IMP-019: PWA Support
- Add service worker + manifest for offline dashboard access.

### IMP-020: Plugin System for Custom Views
- Allow users to add custom views/panels for domain-specific visualizations.

---

## Implementation Roadmap

| Phase | Items | Effort | Timeline |
|-------|-------|--------|----------|
| **Now** | IMP-001, IMP-002 | 5h | This sprint |
| **Next Sprint** | IMP-003, IMP-004, IMP-005, IMP-006 | 21h | Next 2 weeks |
| **Sprint +2** | IMP-007, IMP-013, IMP-014, IMP-015 | 16h | Month 2 |
| **Backlog** | IMP-008→012, IMP-016→020 | 40h+ | Quarterly |

---

## Combined nano-brain Test Summary

| Suite | Files | Tests | Pass | Skip |
|-------|-------|-------|------|------|
| Core (store, search, etc.) | 60 | ~1350 | 1350 | 0 |
| RRI-T Round 3 | 1 | 40 | 40 | 0 |
| RRI-T Round 4 | 1 | 39 | 39 | 0 |
| REST API v1 | 1 | 23 | 23 | 0 |
| **RRI-T Web** | **1** | **36** | **36** | **0** |
| MCP Client (live) | 1 | 0 | 0 | 9 |
| **Total** | **69** | **1524+** | **1524** | **9** |
