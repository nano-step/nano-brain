# Browser UX/UI Analysis — nano-brain Web Dashboard

## Test Environment
- Browser: Chrome (via Claude in Chrome)
- URL: http://localhost:3100/web/
- Server: nano-brain v2026.7.0-rc.19
- Workspace: zengamingx (real production data)

---

## View-by-View Analysis

### 1. Dashboard (`/web/dashboard`) ✅ GOOD
**Screenshot:** 4 stat cards (Version, Uptime 143m, Documents 7180, Embeddings 6516) + Bandit Stats chart + Expand Rate 0% + Preference Weights

**UX Strengths:**
- Clean 4-column stat cards with clear labels
- Bandit chart and expand rate side-by-side — good information density
- Version displayed in both header and card (redundant but harmless)

**UX Issues Found:**
- ⚠️ Hardcoded "+2.4% vs last window" text — this is fake data, not computed (REMOVED in fix)
- ⚠️ "0 variants" and "0 categories" — empty chart areas take up space. Should show empty state message instead of blank chart
- Minor: Uptime shows "143m" — could show "2h 23m" for better readability

### 2. Knowledge Graph (`/web/graph`) ⚠️ BUG
**Screenshot:** Shows "Nodes: 25, Edges: 21" in header but graph area shows "No graph data."

**Bug:** Graph build fails silently — `buildEntityGraph()` returns null even though API returns 25 nodes. Likely issue: nodes have data but Sigma/Graphology can't render (possibly canvas init issue with the new QueryStatus empty state wrapping).

**UX Issues:**
- 🔴 Graph not rendering despite having 25 nodes — critical visual bug
- Type distribution sidebar shows correct data (tool:3, concept:6, file:9, etc.)
- Empty state message is misleading — there IS data, rendering just fails

### 3. Code Dependencies (`/web/code`) ✅ EXCELLENT
**Screenshot:** Beautiful force-directed graph with 1055 files, 2746 edges. Cluster coloring with 25+ clusters visible.

**UX Strengths:**
- ForceAtlas2 layout creates clear visual clusters
- Color-coded clusters with scrollable legend
- Node click shows detail panel: file path, centrality, cluster, imports/dependents count
- Responsive — graph fills available space

**UX Issues:**
- Cluster legend has 25+ entries — could truncate with "show more"
- Label density too low — file names only visible on hover/zoom

### 4. Symbol Graph (`/web/symbols`) ✅ OK (empty)
**Screenshot:** 0 symbols — empty state shows correctly with "Symbol kinds" legend (all zeros)

**UX:** Clean empty state. Legend shows even when empty — acceptable.

### 5. Execution Flows (`/web/flows`) ✅ OK (empty)
**Screenshot:** "No flows found." with filter controls (type dropdown + search input)

**UX:** Proper empty state. Filter controls visible even when empty — good for discoverability.

### 6. Document Connections (`/web/connections`) ✅ OK (empty)
**Screenshot:** "No connection data." with relationship distribution, legend, and selected connections panels

**UX:** Clean 2-column layout with graph area + sidebar. Legend shows all 8 relationship types.

### 7. Infrastructure Symbols (`/web/infrastructure`) ✅ EXCELLENT
**Screenshot:** 722 patterns loaded. API_ENDPOINT section with 301 patterns showing expandable tree.

**UX Strengths:**
- 3-filter bar (Type, Repository, Operation) — very discoverable
- Expandable type → pattern → operations hierarchy
- Operation badges (define, 1 repos) — quick info scanning
- Color dots per infra type

**UX Issues:**
- Long list could benefit from pagination or virtual scrolling for 722 patterns
- No total count per infra type visible (only pattern count)

### 8. Search (`/web/search`) ⚠️ BUG
**Screenshot:** "Searching..." stuck indefinitely after typing "kubernetes"

**Bug:** The search API call hangs — confirmed via network tab showing pending request. Root cause: The server's hybrid search endpoint calls the embedding provider (Ollama/VoyageAI) for vector search, which times out. The FTS-only API (`curl` direct) works fine (1079ms response).

**UX Issues:**
- 🔴 No timeout handling — "Searching..." hangs forever with no way to cancel
- No fallback to FTS-only when vector search is unavailable
- Debounce 300ms works correctly
- Input styling good — placeholder text, search label, result count area

---

## Global UX Assessment

### ✅ What Works Well
1. **Dark theme** — consistent, professional, easy on eyes
2. **Sidebar navigation** — clear icons + labels, active state highlighted
3. **Header** — version display, workspace selector
4. **Card design** — consistent border-radius, spacing, border colors
5. **Empty states** — every view shows appropriate message when no data
6. **Graph interactivity** — click-to-select with detail panel (Code Dependencies)
7. **Typography** — Space Grotesk font, clear hierarchy (2xl → lg → sm → xs)
8. **Color palette** — muted grays (#8888a0) + accent colors per data type

### ⚠️ Issues to Fix

| # | Issue | Severity | View |
|---|-------|----------|------|
| 1 | Knowledge Graph not rendering (25 nodes) | P0 | /graph |
| 2 | Search hangs on embedding timeout | P1 | /search |
| 3 | No search timeout/cancel UX | P1 | /search |
| 4 | Fake "+2.4% vs last window" text | P2 | /dashboard |
| 5 | Empty charts take space without message | P2 | /dashboard |
| 6 | Cluster legend too long (25+ items) | P3 | /code |
| 7 | No pagination for 722 infra patterns | P3 | /infrastructure |

### Improvements Applied in This Session
1. ✅ **ErrorBoundary** — wraps every route, catches crashes gracefully
2. ✅ **QueryStatus** — shared loading spinner + error + retry button for all views
3. ✅ **Skeleton** — animated loading placeholders (cards, graphs, lists)
4. ✅ **Responsive** — grid-dense collapses on <1280px, single-column on <768px
5. ✅ **Console cleanup** — removed debug console.logs from Dashboard, GraphExplorer
6. ✅ **TypeScript fix** — `edge.edge_type` → `edge.edgeType` in graph-adapter
7. ✅ **GraphCanvas fix** — added `any` type annotations for Sigma event params
8. ✅ **Dashboard fix** — removed hardcoded "+2.4% vs last window" fake stat
