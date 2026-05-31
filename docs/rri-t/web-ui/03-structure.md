# RRI-T Phase 3: STRUCTURE — nano-brain Web UI

**Feature:** nano-brain-web-ui (Epic 9)
**Generated from:** Persona Interviews (2026-05-31)
**Total Test Cases:** 100

## Priority Distribution
| Priority | Count | Description |
|----------|-------|-------------|
| P0 | 18 | Critical — blocks release |
| P1 | 38 | Major — fix before release |
| P2 | 44 | Minor — next sprint |

## Dimension Distribution
| Dimension | Count | Target Coverage |
|-----------|-------|----------------|
| D1: UI/UX | 16 | >= 85% |
| D2: API | 14 | >= 85% |
| D3: Performance | 10 | >= 70% |
| D4: Security | 20 | >= 85% |
| D5: Data Integrity | 14 | >= 85% |
| D6: Infrastructure | 12 | >= 70% |
| D7: Edge Cases | 14 | >= 85% |

---

## D4: Security (20 test cases)

### TC-WUI-001
- **Q:** What happens when a document contains `[[<script>alert('xss')</script>]]` and is viewed in DocDrawer?
- **A:** The script tag must be sanitized by rehype-sanitize. No JS execution.
- **R:** XSS defense via SafeMarkdown allow-list
- **P:** P0
- **T:**
  - **Preconditions:** Document exists with wikilink containing script tag
  - **Steps:**
    1. `POST /api/v1/write` with content containing `[[<script>alert('xss')</script>]]`
    2. Fetch the document via `POST /api/v1/get`
    3. Verify raw content is stored as-is
    4. (MANUAL) Open document in DocDrawer, inspect rendered HTML
  - **Expected Result:** Script tag is stripped/escaped in rendered output. No alert dialog.
  - **Dimension:** D4: Security
  - **Source Persona:** QA Destroyer

---

## D2: API (14 test cases)

### TC-WUI-021
- **Q:** Does `GET /api/v1/stats?workspace=<hash>` return correct aggregation?
- **A:** Must include collections, chunks by embed_status, graph_edges by type, top_tags, recent_docs.
- **R:** Stats endpoint contract
- **P:** P0
- **T:**
  - **Steps:** `curl -s /api/v1/stats?workspace=<hash>` and validate JSON shape
  - **Expected Result:** JSON with `collections[]`, `chunks[]`, `graph_edges[]`, `top_tags[]`, `recent_docs[]`; query_ms < 500
  - **Dimension:** D2: API
  - **Source Persona:** Business Analyst

### TC-WUI-022
- **Q:** Does `GET /api/v1/doctor` return health checks?
- **A:** Must return array of `{name, status, detail}` for PG, pgvector, embedding provider, model.
- **R:** Doctor endpoint contract
- **P:** P1
- **T:**
  - **Steps:** `curl -s /api/v1/doctor` and validate JSON
  - **Expected Result:** `all_passed: true/false`, `checks[]` with name/status/detail fields
  - **Dimension:** D2: API
  - **Source Persona:** Business Analyst

### TC-WUI-023
- **Q:** Does `GET /api/v1/workspaces` return workspace list with doc counts?
- **A:** Array of objects with workspace_hash, root_path, name, document_count, timestamps.
- **R:** Workspace list endpoint
- **P:** P0
- **T:**
  - **Steps:** `curl -s /api/v1/workspaces` and validate shape
  - **Expected Result:** JSON array with workspace objects containing all required fields
  - **Dimension:** D2: API
  - **Source Persona:** End User

### TC-WUI-024
- **Q:** Does `GET /api/v1/tags?workspace=<hash>` return tags with counts?
- **A:** Array of `{tag, count}` sorted by count desc.
- **R:** Tags endpoint contract
- **P:** P1
- **T:**
  - **Steps:** `curl -s /api/v1/tags?workspace=<hash>`
  - **Expected Result:** JSON array of `{tag: string, count: number}` objects
  - **Dimension:** D2: API
  - **Source Persona:** End User

### TC-WUI-025
- **Q:** Does `GET /api/v1/symbols?workspace=<hash>&query=<q>` return symbol results?
- **A:** Object with count and symbols array with name, kind, language, signature, source_path.
- **R:** Symbols endpoint contract
- **P:** P1
- **T:**
  - **Steps:** `curl -s "/api/v1/symbols?workspace=<hash>&query=serve&limit=5"`
  - **Expected Result:** `{count: N, symbols: [{name, kind, language, signature, source_path}]}`
  - **Dimension:** D2: API
  - **Source Persona:** End User

### TC-WUI-026
- **Q:** Does `POST /api/v1/graph/neighborhood` return neighborhood data?
- **A:** Must return nodes[], edges[], truncated flag, frontier_nodes.
- **R:** Graph neighborhood endpoint
- **P:** P1
- **T:**
  - **Steps:** `curl -X POST /api/v1/graph/neighborhood -H "X-Requested-With: nano-brain-ui" -H "Content-Type: application/json" -d '{"workspace":"<hash>","focus":"<symbol>","depth":2,"direction":"both","node_kind":"symbol"}'`
  - **Expected Result:** `{nodes: [], edges: [], truncated: bool, frontier_nodes: []}`
  - **Dimension:** D2: API
  - **Source Persona:** End User

### TC-WUI-027
- **Q:** Does `GET /api/v1/links/<doc_id>/backlinks` return backlink list?
- **A:** Must return `{doc_id, items[], total}`.
- **R:** Backlinks endpoint contract
- **P:** P1
- **T:**
  - **Steps:** `curl -s /api/v1/links/<doc_id>/backlinks?workspace=<hash>&limit=10`
  - **Expected Result:** JSON with `doc_id`, `items[]`, and `total`
  - **Dimension:** D2: API
  - **Source Persona:** End User

### TC-WUI-028
- **Q:** Does `GET /api/v1/links/resolve?workspace=<hash>&query=<title>` resolve wikilinks?
- **A:** Must return `{matched: [uuid], ambiguous: bool, kind: "id"|"title"}`.
- **R:** Link resolution endpoint
- **P:** P1
- **T:**
  - **Steps:** `curl -s "/api/v1/links/resolve?workspace=<hash>&query=server.go"`
  - **Expected Result:** JSON with matched UUIDs, ambiguous flag, and kind
  - **Dimension:** D2: API
  - **Source Persona:** End User

### TC-WUI-029
- **Q:** Does `POST /api/v1/config` accept partial patch?
- **A:** Must validate against safe-patch allowlist, persist, trigger reload.
- **R:** Config patch endpoint
- **P:** P1
- **T:**
  - **Steps:** `curl -X POST /api/v1/config -H "X-Requested-With: nano-brain-ui" -H "Content-Type: application/json" -d '{"search":{"limit":30}}'`
  - **Expected Result:** Returns updated config with `search.limit: 30`
  - **Dimension:** D2: API
  - **Source Persona:** Business Analyst

### TC-WUI-030
- **Q:** Does `POST /api/v1/get` return single document by source_path?
- **A:** Full document with id, title, content, tags, collection, metadata, timestamps.
- **R:** Document get endpoint
- **P:** P0
- **T:**
  - **Steps:** `curl -X POST /api/v1/get -H "Content-Type: application/json" -d '{"workspace":"<hash>","source_path":"<path>"}'`
  - **Expected Result:** Full document JSON with all fields populated
  - **Dimension:** D2: API
  - **Source Persona:** End User

### TC-WUI-031
- **Q:** Does `GET /api/v1/events?workspace=<hash>` send initial `hello` event within 100ms?
- **A:** First SSE event must be `hello` type with server_version and workspace.
- **R:** SSE streaming spec
- **P:** P0
- **T:**
  - **Steps:** `timeout 2 curl -sN /api/v1/events?workspace=<hash>`
  - **Expected Result:** Receives `event: hello\ndata: {...}` within first 100ms
  - **Dimension:** D2: API
  - **Source Persona:** DevOps Tester

### TC-WUI-032
- **Q:** Does SSE emit heartbeat comments every 30 seconds?
- **A:** `:` comment line for proxy keep-alive.
- **R:** SSE streaming spec, proxy compatibility
- **P:** P2
- **T:**
  - **Steps:** `timeout 35 curl -sN /api/v1/events?workspace=<hash>` and look for `:` line
  - **Expected Result:** At least one `:` heartbeat comment within 35 seconds
  - **Dimension:** D2: API
  - **Source Persona:** DevOps Tester

### TC-WUI-033
- **Q:** Does `/api/v1/stats` respond within 500ms?
- **A:** Stats query must be fast.
- **R:** Performance acceptance criteria
- **P:** P1
- **T:**
  - **Steps:** `curl -o /dev/null -s -w "%{time_total}" /api/v1/stats?workspace=<hash>`
  - **Expected Result:** Response time < 0.5 seconds
  - **Dimension:** D2: API
  - **Source Persona:** Business Analyst

### TC-WUI-034
- **Q:** Does `POST /api/v1/search` handle empty query gracefully?
- **A:** Should return empty results or all docs, not error.
- **R:** Search robustness
- **P:** P2
- **T:**
  - **Steps:** `curl -X POST /api/v1/search -H "Content-Type: application/json" -d '{"workspace":"<hash>","query":""}'`
  - **Expected Result:** Valid JSON response (empty results or recent docs), not 500 error
  - **Dimension:** D2: API
  - **Source Persona:** QA Destroyer

---

## D5: Data Integrity (14 test cases)

### TC-WUI-035
- **Q:** Does tag filter implement AND logic correctly?
- **A:** `tags=["go","function"]` must show only docs with BOTH tags.
- **R:** Tag filter AND logic
- **P:** P0
- **T:**
  - **Steps:** `curl -X POST /api/v1/search -H "Content-Type: application/json" -d '{"workspace":"<hash>","query":"*","tags":["go","function"]}'`
  - **Expected Result:** All returned documents contain both `go` and `function` in their tags
  - **Dimension:** D5: Data Integrity
  - **Source Persona:** Business Analyst

### TC-WUI-036
- **Q:** Does workspace isolation prevent cross-workspace data in search?
- **A:** Search in workspace A must never return workspace B documents.
- **R:** Workspace scoping
- **P:** P0
- **T:**
  - **Steps:**
    1. Search workspace A for a known term
    2. Search workspace B for same term
    3. Verify results are workspace-scoped
  - **Expected Result:** Each result set only contains docs from the queried workspace
  - **Dimension:** D5: Data Integrity
  - **Source Persona:** Security Auditor

### TC-WUI-037
- **Q:** Does the Dashboard doc count match actual database count?
- **A:** Stats doc counts must be accurate.
- **R:** Data accuracy
- **P:** P1
- **T:**
  - **Steps:**
    1. `curl /api/v1/stats?workspace=<hash>` → sum collection doc_counts
    2. `curl /api/v1/workspaces` → find workspace's document_count
    3. Compare
  - **Expected Result:** Counts match or are very close
  - **Dimension:** D5: Data Integrity
  - **Source Persona:** Business Analyst

### TC-WUI-038
- **Q:** Does backlinks list correctly show referencing documents?
- **A:** Every doc containing `[[target]]` wikilink must appear in target's backlinks.
- **R:** Backlinks accuracy
- **P:** P1
- **T:**
  - **Steps:** Query known doc's backlinks
  - **Expected Result:** Referencing docs appear with snippet showing the wikilink
  - **Dimension:** D5: Data Integrity
  - **Source Persona:** End User

### TC-WUI-039
- **Q:** Does the supersession chain show correct ordering?
- **A:** If doc B supersedes A, chain must be correctly linked.
- **R:** Supersession chain integrity
- **P:** P1
- **T:**
  - **Steps:** Fetch a document that has a supersedes field, verify chain
  - **Expected Result:** Supersedes chain is correctly linked
  - **Dimension:** D5: Data Integrity
  - **Source Persona:** Business Analyst

### TC-WUI-040
- **Q:** Does wikilink resolve by title (case-insensitive)?
- **A:** `[[server.go]]` should match doc titled "server.go" or "Server.go".
- **R:** Wikilink title resolution
- **P:** P1
- **T:**
  - **Steps:** `curl "/api/v1/links/resolve?workspace=<hash>&query=server.go"`
  - **Expected Result:** Returns matched UUID(s)
  - **Dimension:** D5: Data Integrity
  - **Source Persona:** End User

### TC-WUI-041
- **Q:** Does wikilink resolve by UUID?
- **A:** ID lookup must be exact match.
- **R:** Wikilink ID resolution
- **P:** P1
- **T:**
  - **Steps:**
    1. Get a known doc UUID
    2. `curl "/api/v1/links/resolve?workspace=<hash>&query=<uuid>"`
  - **Expected Result:** Returns that UUID in matched with `kind: "id"`
  - **Dimension:** D5: Data Integrity
  - **Source Persona:** End User

### TC-WUI-042
- **Q:** Does ambiguous wikilink (title matches multiple docs) report correctly?
- **A:** Should return `ambiguous: true`.
- **R:** Ambiguous wikilink handling
- **P:** P2
- **T:**
  - **Steps:** Resolve a title that matches multiple docs
  - **Expected Result:** `ambiguous: true` with 2+ matched UUIDs
  - **Dimension:** D5: Data Integrity
  - **Source Persona:** End User

### TC-WUI-043
- **Q:** Does Graph node count respect 500-node cap?
- **A:** Neighborhood query with large fan-out must cap at 500 nodes.
- **R:** Graph cap enforcement
- **P:** P1
- **T:**
  - **Steps:** Query neighborhood of highly-connected symbol at depth=5
  - **Expected Result:** `nodes.length <= 500` and `truncated: true`
  - **Dimension:** D5: Data Integrity
  - **Source Persona:** QA Destroyer

### TC-WUI-044
- **Q:** Are graph edges correctly typed?
- **A:** Each edge must have valid edge_type.
- **R:** Graph edge accuracy
- **P:** P1
- **T:**
  - **Steps:** Query neighborhood, check each edge type
  - **Expected Result:** All edges have `edge_type` in valid set
  - **Dimension:** D5: Data Integrity
  - **Source Persona:** Business Analyst

### TC-WUI-045
- **Q:** Does Symbols search return results sorted by impact?
- **A:** Symbols with more graph edges should appear first.
- **R:** Symbols sort order
- **P:** P2
- **T:**
  - **Steps:** `curl "/api/v1/symbols?workspace=<hash>&query=handle&limit=10"`
  - **Expected Result:** Results ordered reasonably (most impactful first)
  - **Dimension:** D5: Data Integrity
  - **Source Persona:** Business Analyst

### TC-WUI-046
- **Q:** Does search work with Vietnamese text?
- **A:** Unicode must be handled correctly.
- **R:** Unicode/i18n support
- **P:** P2
- **T:**
  - **Steps:** Search for Vietnamese term if any Vietnamese docs exist
  - **Expected Result:** Correct results returned
  - **Dimension:** D5: Data Integrity
  - **Source Persona:** End User

### TC-WUI-047
- **Q:** Does collection filter scope results correctly?
- **A:** Filtering by `code` should only show code docs.
- **R:** Collection filtering
- **P:** P1
- **T:**
  - **Steps:** Search with collection filter
  - **Expected Result:** All results from specified collection
  - **Dimension:** D5: Data Integrity
  - **Source Persona:** Business Analyst

### TC-WUI-048
- **Q:** Does workspace isolation hold for graph queries?
- **A:** Graph query must not return cross-workspace nodes.
- **R:** Workspace scoping in graph
- **P:** P0
- **T:**
  - **Steps:** Query graph in workspace A, verify no cross-workspace data
  - **Expected Result:** All nodes belong to queried workspace
  - **Dimension:** D5: Data Integrity
  - **Source Persona:** Security Auditor

---

## D1: UI/UX (16 test cases)

### TC-WUI-049
- **Q:** Does `/ui` load and return HTML?
- **A:** Should render with React app HTML.
- **R:** UI availability
- **P:** P0
- **T:**
  - **Steps:** `curl -s http://localhost:3100/ui` and check for HTML response
  - **Expected Result:** HTTP 200 with `text/html`, contains `<div id="root">`
  - **Dimension:** D1: UI/UX
  - **Source Persona:** End User

### TC-WUI-050
- **Q:** Does `/ui/memory` return SPA fallback?
- **A:** Client-side route must serve index.html.
- **R:** SPA fallback
- **P:** P0
- **T:**
  - **Steps:** `curl -s http://localhost:3100/ui/memory`
  - **Expected Result:** HTTP 200 with same index.html content
  - **Dimension:** D1: UI/UX
  - **Source Persona:** End User

### TC-WUI-051
- **Q:** Does `/ui/nonexistent-route` serve SPA fallback?
- **A:** Unknown routes under /ui/* serve index.html.
- **R:** SPA fallback for all client routes
- **P:** P1
- **T:**
  - **Steps:** `curl -s http://localhost:3100/ui/some-unknown-route`
  - **Expected Result:** HTTP 200 with index.html
  - **Dimension:** D1: UI/UX
  - **Source Persona:** DevOps Tester

### TC-WUI-052
- **Q:** Do hashed assets have immutable cache headers?
- **A:** `max-age=31536000, immutable` for assets with hash in filename.
- **R:** Cache strategy
- **P:** P2
- **T:**
  - **Steps:** Extract a JS asset URL from index.html, check its Cache-Control
  - **Expected Result:** Long-lived cache headers for hashed assets
  - **Dimension:** D1: UI/UX
  - **Source Persona:** DevOps Tester

### TC-WUI-053
- **Q:** Does index.html have `no-cache` header?
- **A:** HTML must always revalidate.
- **R:** Cache strategy
- **P:** P2
- **T:**
  - **Steps:** `curl -sI http://localhost:3100/ui` and check Cache-Control
  - **Expected Result:** `no-cache` or `no-store`
  - **Dimension:** D1: UI/UX
  - **Source Persona:** DevOps Tester

### TC-WUI-054
- **Q:** Do all 6 panel routes return 200?
- **A:** /ui/dashboard, /ui/memory, /ui/graph, /ui/symbols, /ui/harvest, /ui/settings
- **R:** Route coverage
- **P:** P1
- **T:**
  - **Steps:** Curl each of the 6 routes
  - **Expected Result:** All return HTTP 200
  - **Dimension:** D1: UI/UX
  - **Source Persona:** End User

### TC-WUI-055 — TC-WUI-064
- **Note:** TC-WUI-055 through TC-WUI-064 are MANUAL-REQUIRED UI tests covering: WorkspaceSelector dropdown, CommandPalette (Cmd+K), mnemonic shortcuts, DocDrawer Esc close, ConfirmDialog typed confirm, NonLoopbackBindBanner, focus-visible rings, Graph mode toggle, Settings doctor checks, and Memory empty state. These require browser interaction.
- **Dimension:** D1: UI/UX
- **Priority:** P1-P2 (see individual test case descriptions in Phase 4)

---

## D3: Performance (10 test cases)

### TC-WUI-065
- **Q:** Is the gzipped bundle size within target?
- **A:** Initial bundle < 600 KB gzipped.
- **R:** Bundle size target
- **P:** P1
- **T:**
  - **Steps:** Check dist/ asset sizes, calculate gzipped total
  - **Expected Result:** < 600 KB gzipped main bundle
  - **Dimension:** D3: Performance
  - **Source Persona:** DevOps Tester

### TC-WUI-066 through TC-WUI-074
- Performance tests for: graph chunk size, stats latency, symbol search latency, hybrid query latency, graph neighborhood latency, backlinks latency, UI page load latency, SSE hello latency, doctor check latency.
- See Phase 4 for execution details and measured results.

---

## D6: Infrastructure (12 test cases)

### TC-WUI-075 through TC-WUI-086
- Infrastructure tests for: SSE hello event, X-Accel-Buffering header, SSE Content-Type, SSE Cache-Control, dist/index.html existence, JS asset existence, MIME types, /mcp compatibility, /health stability, /api/status stability, SSE subscriber cap, server stability with UI.
- See Phase 4 for execution details.

---

## D7: Edge Cases (14 test cases)

### TC-WUI-087 through TC-WUI-100
- Edge case tests for: special regex chars in search, nonexistent workspace, empty wikilink, long title, many tags, special chars in workspace name, nonexistent graph focus, nonexistent doc backlinks, empty resolve query, null bytes, zero depth, excessive depth, SSE without workspace, concurrent reindex.
- See Phase 4 for execution details.


### TC-WUI-011
- **Q:** What happens when I craft wikilink `[[javascript:alert(1)]]`?
- **A:** Resolver must not create a `javascript:` protocol link.
- **R:** XSS prevention in wikilink resolver
- **P:** P0
- **T:**
  - **Preconditions:** Document with `[[javascript:alert(1)]]` content
  - **Steps:**
    1. Write document with that content
    2. (MANUAL) View in DocDrawer
    3. Inspect rendered anchor href
  - **Expected Result:** Link is rendered as broken/unresolved, not as `javascript:` href
  - **Dimension:** D4: Security
  - **Source Persona:** Security Auditor

### TC-WUI-012
- **Q:** What happens when I check Referrer-Policy header?
- **A:** Must be `same-origin`.
- **R:** Security headers
- **P:** P2
- **T:**
  - **Preconditions:** Server running
  - **Steps:** `curl -sI http://localhost:3100/ui` and check header
  - **Expected Result:** `Referrer-Policy: same-origin`
  - **Dimension:** D4: Security
  - **Source Persona:** Security Auditor

### TC-WUI-013
- **Q:** Is `X-Powered-By` header suppressed?
- **A:** Should not reveal server technology.
- **R:** Security hardening
- **P:** P2
- **T:**
  - **Steps:** `curl -sI http://localhost:3100/ui` and check for X-Powered-By
  - **Expected Result:** No `X-Powered-By` header present
  - **Dimension:** D4: Security
  - **Source Persona:** Security Auditor

### TC-WUI-014
- **Q:** Does localStorage contain any API keys or secrets?
- **A:** Only workspace preference, position cache, recent searches should be stored.
- **R:** No secrets in client storage
- **P:** P1
- **T:**
  - **Steps:** (MANUAL) Open DevTools → Application → LocalStorage, inspect all keys
  - **Expected Result:** No API keys, tokens, or passwords. Only UI state (workspace hash, position cache, recent searches).
  - **Dimension:** D4: Security
  - **Source Persona:** Security Auditor

### TC-WUI-015
- **Q:** What happens when I send POST to mutating endpoint with `X-Requested-With: nano-brain-ui` header?
- **A:** CSRF middleware allows it (step 1).
- **R:** CSRF bypass for legitimate UI requests
- **P:** P0
- **T:**
  - **Steps:**
    1. `curl -X POST /api/v1/write -H "X-Requested-With: nano-brain-ui" -H "Content-Type: application/json" -d '{"workspace":"...","source_path":"csrf-test","content":"test"}'`
  - **Expected Result:** Request proceeds past CSRF check (success or other validation error)
  - **Dimension:** D4: Security
  - **Source Persona:** Security Auditor

### TC-WUI-016
- **Q:** Does `/api/v1/stats` without workspace parameter leak cross-workspace data?
- **A:** Should require workspace parameter or return error.
- **R:** Workspace isolation
- **P:** P1
- **T:**
  - **Steps:** `curl -s http://localhost:3100/api/v1/stats` (no workspace param)
  - **Expected Result:** Error response or empty results, not aggregate cross-workspace data
  - **Dimension:** D4: Security
  - **Source Persona:** Security Auditor

### TC-WUI-017
- **Q:** Can I access another workspace's data by manipulating X-Workspace-Hash?
- **A:** Should only return data for the specified workspace (no privilege escalation since no auth).
- **R:** Workspace isolation (data scoping, not auth)
- **P:** P1
- **T:**
  - **Steps:**
    1. Query workspace A docs
    2. Switch X-Workspace-Hash to workspace B
    3. Query docs — should see B's data only
  - **Expected Result:** Each request returns only the specified workspace's data
  - **Dimension:** D4: Security
  - **Source Persona:** Security Auditor

### TC-WUI-018
- **Q:** Does CSRF middleware reject POST with Origin from different host but same port?
- **A:** Yes, step 5 rejects different-host Origin.
- **R:** CSRF 7-step middleware
- **P:** P0
- **T:**
  - **Steps:** `curl -X POST /api/v1/write -H "Origin: http://attacker.com:3100" -H "Content-Type: application/json" -d '{...}'`
  - **Expected Result:** HTTP 403
  - **Dimension:** D4: Security
  - **Source Persona:** Security Auditor

### TC-WUI-019
- **Q:** Does the UI make any third-party CDN requests?
- **A:** No. All fonts, icons, scripts, styles must be bundled. Offline + self-contained.
- **R:** Self-contained bundle, no external requests
- **P:** P1
- **T:**
  - **Steps:** (MANUAL) Open DevTools → Network, load /ui, filter for non-localhost domains
  - **Expected Result:** Zero requests to external domains
  - **Dimension:** D4: Security
  - **Source Persona:** Security Auditor

### TC-WUI-020
- **Q:** Does `POST /api/v1/config` with malformed JSON return proper error?
- **A:** Should return 400 with error message, not 500 or crash.
- **R:** Input validation
- **P:** P2
- **T:**
  - **Steps:** `curl -X POST /api/v1/config -H "X-Requested-With: nano-brain-ui" -H "Content-Type: application/json" -d 'not-json'`
  - **Expected Result:** HTTP 400 with descriptive error
  - **Dimension:** D4: Security
  - **Source Persona:** QA Destroyer
