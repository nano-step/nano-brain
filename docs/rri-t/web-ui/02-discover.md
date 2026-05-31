# RRI-T Phase 2: DISCOVER — nano-brain Web UI

**Feature:** nano-brain-web-ui (Epic 9)
**Date:** 2026-05-31
**Interviewer:** OpenCode (automated RRI-T pipeline)

## Interview Summary
| Persona | Questions Generated | Key Concerns |
|---------|-------------------|--------------|
| End User | 15/15 | Search accuracy, navigation flow, wikilink UX, keyboard shortcuts |
| Business Analyst | 12/12 | Data integrity, filter correctness, URL state, workspace isolation |
| QA Destroyer | 15/15 | XSS, malformed input, rapid actions, large datasets, race conditions |
| DevOps Tester | 12/12 | SSE reliability, memory leaks, embed.FS fallback, bundle integrity |
| Security Auditor | 12/12 | CSRF, XSS, config secrets, CSP, bind safety, header injection |
| **Total** | **66/66** | |

---

## Persona 1: End User (Knowledge Worker)

### Context
As a developer using nano-brain daily to recall past decisions, search code symbols, and explore knowledge graphs, I need the Web UI to be fast, intuitive, and reliable. I switch between Dashboard, Memory, Graph, and Symbols panels frequently. I use keyboard shortcuts and the command palette. I care about search relevance, readable content, and not losing my place.

### Questions
1. What happens when I open the Dashboard and the embed queue has 10,000 pending items — does the counter update live via SSE?
2. What happens when I search for "authentication" in Memory and click a result — does the DocDrawer show the full markdown with rendered wikilinks?
3. What happens when I click a wikilink `[[server.go]]` in the DocDrawer — does it resolve to the correct document and swap the drawer content?
4. What happens when I use Cmd+K to search for a symbol name like "NewServer" — does it appear in results within 150ms?
5. What happens when I navigate to Graph, search for "handleQuery" in focus input, set depth=3, direction=both — does the graph render within 2 seconds?
6. What happens when I switch between Code mode and Knowledge mode in the Graph — does each mode preserve its own position state?
7. What happens when I double-click a node in the Code graph — does it navigate to the Symbols panel filtered to that symbol?
8. What happens when I use mnemonic shortcut `g m` to navigate to Memory — does it work within 800ms from any panel?
9. What happens when I filter Memory by tags `decision` AND `auth` via URL `?tags=decision,auth` — do I see only docs with BOTH tags?
10. What happens when I share a URL like `/ui/memory?tags=decision&doc=<id>` with a colleague — does it load the correct filtered view with the drawer open?
11. What happens when I open the Harvest panel and click "Trigger harvest" — does the SSE progress counter show real-time updates?
12. What happens when I edit a document via the DocDrawer Edit button — does it create a new superseding version without mutating the original?
13. What happens when I view a document with Vietnamese content like "Quyết định kiến trúc" — is the Unicode rendered correctly in title, body, and search results?
14. What happens when I use browser back/forward buttons after navigating between panels — does URL state (workspace, tags, doc) restore correctly?
15. What happens when I switch workspaces via the WorkspaceSelector — do all panels reload with the new workspace's data?

### Key Concerns
- Search relevance and speed across large workspaces (6000+ docs)
- Wikilink resolution correctness (ID vs title, ambiguous cases)
- Keyboard-first navigation (Cmd+K, mnemonic shortcuts, Tab/Enter/Esc)
- URL state synchronization for shareability
- Vietnamese/Unicode content handling

---

## Persona 2: Business Analyst (Success Criteria Validator)

### Context
As the person defining success criteria for the Web UI, I need to verify that each panel displays correct data, filters work as specified, workspace isolation is enforced, and the UI meets its acceptance criteria. I care about observable behavior matching the spec.

### Questions
1. What happens when the Dashboard shows doc counts — do they match the actual database counts from `/api/v1/stats`?
2. What happens when Memory tag filter uses AND logic — if I select tags `go` and `function`, do I see only docs tagged with BOTH?
3. What happens when I view backlinks for a document — are ALL documents that reference it via wikilinks listed?
4. What happens when the supersession chain shows doc A → doc B → doc C — is the chain correctly ordered chronologically?
5. What happens when a workspace has zero documents — do all panels show appropriate empty states?
6. What happens when the embed queue shows "failed" status for some chunks — does the Dashboard reflect the correct failed count?
7. What happens when Settings doctor checks show a warning (e.g., embedding model not found) — is it displayed as ⚠ with actionable hint?
8. What happens when I save a Settings config change (e.g., change `search.limit` from 20 to 50) — does it persist and take effect immediately?
9. What happens when the Symbols panel shows impact counts — do they correlate with actual graph edge counts?
10. What happens when I use the Graph "Show in graph" button from Symbols — does it navigate to Graph with the correct focus?
11. What happens when multiple workspaces exist — does workspace isolation prevent cross-workspace data leakage in Memory, Graph, and Symbols?
12. What happens when the SSE `embed_queue` event fires — does the Dashboard counter update without a full page reload?

### Key Concerns
- Data accuracy between API responses and UI display
- Filter logic correctness (AND vs OR for tags)
- Workspace isolation enforcement
- Empty state UX for new/empty workspaces
- Config change persistence and immediate effect

---

## Persona 3: QA Destroyer (Break Things)

### Context
My job is to break the Web UI. I will inject malicious input, stress boundaries, trigger race conditions, and exploit every edge case. I care about crashes, data corruption, XSS, and unhandled errors.

### Questions
1. What happens when I create a document with content `[[<script>alert('xss')</script>]]` and view it in DocDrawer — is the script tag sanitized?
2. What happens when I paste a 10MB markdown document into the edit form — does the UI handle it gracefully without freezing?
3. What happens when I rapidly click between 10 different documents in Memory panel — does the DocDrawer show the correct last-clicked document without race conditions?
4. What happens when I open the Graph with a symbol that has 5000+ connected nodes — does the 500-node cap apply and show truncation affordance?
5. What happens when I type `[[` followed by `]]` (empty wikilink) — does it render as broken link or crash the parser?
6. What happens when I search for a symbol with special characters like `func<T>` or `operator++` — does the search handle it without error?
7. What happens when I open the command palette and type 10000 characters — does the fuzzy search handle it without freezing?
8. What happens when I rapidly toggle between Code and Knowledge modes in Graph 50 times — does the state management break?
9. What happens when I open DocDrawer, then navigate away, then back, then open drawer again 100 times — does memory leak?
10. What happens when I set depth=5 in Graph for a heavily connected root node — does it timeout gracefully?
11. What happens when I inject `"; DROP TABLE documents; --` in the search input — does the API handle it safely?
12. What happens when I create a document with title containing null bytes `\x00` — does the UI handle it?
13. What happens when I open 9 SSE connections from the same IP — does the 9th get rejected with 429?
14. What happens when I send a POST to `/api/v1/config` with malformed JSON — does it return a proper error?
15. What happens when I trigger reindex while another reindex is already running — does it queue or reject?

### Key Concerns
- XSS through every input vector (wikilinks, markdown, symbol names, doc titles, tags)
- Race conditions in drawer state, graph mode switching
- Memory leaks from repeated drawer open/close
- SQL/NoSQL injection through search inputs
- Boundary conditions (empty, null, huge, special chars)
- Concurrent operation handling

---

## Persona 4: DevOps Tester (Operational Reliability)

### Context
As the person responsible for deploying and monitoring nano-brain, I need the Web UI to be operationally sound: fast startup, no memory leaks, correct SSE behavior, proper error handling on server restart, and bundle integrity. I care about reliability under adverse conditions.

### Questions
1. What happens when the server restarts while a browser has an active SSE connection — does EventSource auto-reconnect within 3 seconds?
2. What happens when the embed.FS is missing `dist/index.html` — does the fallback instructional page render?
3. What happens when I check the gzipped bundle size — is it within the 600 KB target (app) + 150 KB (graph lazy chunk)?
4. What happens when I open the UI for the first time (cold cache) — is first paint under 500ms?
5. What happens when the SSE subscriber buffer overflows (64 events queued) — does it emit a `lag` event and the client re-queries REST?
6. What happens when I deploy behind nginx with `proxy_buffering on` — do SSE events still flow (X-Accel-Buffering: no header)?
7. What happens when the PostgreSQL connection is temporarily lost — does the UI show an error state rather than hanging?
8. What happens when the Ollama embedding service goes down — does the Dashboard embed queue show appropriate status?
9. What happens when `dist/` directory contains stale assets from a previous build — does the cache-busting (hashed filenames) prevent serving stale JS?
10. What happens when I access `/ui/nonexistent-route` — does the SPA fallback serve index.html and the client router shows a 404?
11. What happens when the reindex SSE stream is interrupted by network blip — does the client show partial progress and recover?
12. What happens when server binds to 0.0.0.0 without `--unsafe-no-auth` — does it refuse to start or show the warning banner?

### Key Concerns
- SSE reliability (reconnect, backpressure, proxy compatibility)
- Bundle integrity and caching strategy
- embed.FS fallback behavior
- Memory/CPU under sustained use
- Error states for downstream service failures
- Bind safety enforcement

---

## Persona 5: Security Auditor (Threat Model)

### Context
As a security auditor, I need to verify that the Web UI does not expose the server to XSS, CSRF, clickjacking, information disclosure, or unauthorized access. I care about every attack vector in the OWASP Top 10 that applies to this local-first tool.

### Questions
1. What happens when I send a POST to `/api/v1/write` without `X-Requested-With: nano-brain-ui` header and without Origin/Referer — does CSRF middleware allow it (CLI/MCP path)?
2. What happens when I send a POST with `Origin: https://evil.com` — does the CSRF middleware reject it?
3. What happens when I send a POST with `Origin: null` (iframe sandbox) — does the CSRF middleware reject it?
4. What happens when I inspect the `GET /api/v1/config` response — are DATABASE_URL, VOYAGE_API_KEY, and SUMMARIZE_API_KEY redacted?
5. What happens when I check the Content-Security-Policy header on `/ui` responses — does it block inline scripts and external resources?
6. What happens when I try to iframe the UI from another origin — does X-Frame-Options: DENY prevent it?
7. What happens when I inject `<img src=x onerror=alert(1)>` into a document title and view it in Memory panel — is it sanitized?
8. What happens when I craft a wikilink `[[javascript:alert(1)]]` — does the resolver prevent protocol-based XSS?
9. What happens when I inspect localStorage for sensitive data — are there any auth tokens or API keys stored?
10. What happens when I check response headers for server version fingerprinting — is `X-Powered-By` suppressed?
11. What happens when I access `/api/v1/stats` without workspace parameter — does it leak cross-workspace aggregate data?
12. What happens when I manipulate the `X-Workspace-Hash` header to access a different workspace's data — does workspace isolation hold?

### Key Concerns
- CSRF middleware 7-step logic correctness
- XSS through all rendering paths (markdown, wikilinks, titles, tags, symbol names)
- Config secrets redaction completeness
- CSP header strictness
- localStorage security (no secrets)
- Workspace isolation enforcement
- Header security (X-Frame-Options, X-Content-Type-Options, Referrer-Policy)

---

## Raw Test Ideas (Consolidated)
| # | Idea | Source Persona | Potential Dimension | Priority |
|---|------|---------------|--------------------|----|
| 1 | XSS via wikilink `[[<script>]]` | QA Destroyer | D4: Security | P0 |
| 2 | CSRF on destructive POST without header | Security Auditor | D4: Security | P0 |
| 3 | Config secrets redaction | Security Auditor | D4: Security | P0 |
| 4 | Tag AND filter correctness | Business Analyst | D5: Data Integrity | P0 |
| 5 | SSE reconnect on server restart | DevOps Tester | D6: Infrastructure | P1 |
| 6 | Graph 500-node cap enforcement | QA Destroyer | D3: Performance | P1 |
| 7 | Wikilink resolution (ID + title) | End User | D5: Data Integrity | P1 |
| 8 | DocDrawer race condition on rapid clicks | QA Destroyer | D7: Edge Cases | P1 |
| 9 | Empty state for all panels | Business Analyst | D1: UI/UX | P1 |
| 10 | Bundle size within target | DevOps Tester | D3: Performance | P1 |
| 11 | CSP header on /ui responses | Security Auditor | D4: Security | P1 |
| 12 | Workspace isolation in Memory/Graph | Security Auditor | D5: Data Integrity | P0 |
| 13 | Mnemonic shortcuts work | End User | D1: UI/UX | P2 |
| 14 | Vietnamese content rendering | End User | D7: Edge Cases | P2 |
| 15 | Browser back/forward URL state | End User | D1: UI/UX | P2 |
| 16 | SQL injection in search | QA Destroyer | D4: Security | P0 |
| 17 | 10MB document in DocDrawer | QA Destroyer | D3: Performance | P2 |
| 18 | embed.FS fallback page | DevOps Tester | D6: Infrastructure | P2 |
| 19 | SSE subscriber cap (8 per IP) | DevOps Tester | D6: Infrastructure | P2 |
| 20 | localStorage quota handling | DevOps Tester | D6: Infrastructure | P2 |
