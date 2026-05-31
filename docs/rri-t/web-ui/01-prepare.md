# RRI-T Phase 1: PREPARE — nano-brain Web UI

**Feature:** nano-brain-web-ui (Epic 9)
**Date:** 2026-05-31
**Spec Sources:** `openspec/changes/archive/2026-05-30-web-ui/` (3 specs + design + tasks)
**Server URL:** `http://localhost:3100` (host) / `http://host.docker.internal:3100` (container)
**API Base:** `/api/v1`

---

## 1. Feature Inventory

### Routes / Pages
| Route | Panel | Key Features |
|-------|-------|-------------|
| `/ui/dashboard` | DashboardPanel | Server version, uptime, embedding provider, doc/chunk/edge counts, recent docs (10), live embed queue via SSE |
| `/ui/memory` | MemoryPanel | BM25 text filter, multi-select AND tag chips (URL-synced `?tags=a,b`), row click → DocDrawer (URL-synced `?doc=<id>`) |
| `/ui/graph` | GraphPanel | Dual-mode (Code/Knowledge), Sigma.js + Graphology + ForceAtlas2, focus input, depth [1-5], direction [in/out/both], edge-type chips, position cache in localStorage, hover tooltip, double-click navigation |
| `/ui/symbols` | SymbolsPanel | Type-ahead search, kind/lang chips, sorted by impact desc, "Show in graph" per-row button |
| `/ui/harvest` | HarvestPanel | Session list, Trigger harvest button → POST /api/v1/reindex with SSE progress, AbortController-based fetch |
| `/ui/settings` | SettingsPanel | react-hook-form + zod, GET/POST /api/v1/config, doctor checks (✅/⚠/❌), destructive actions gated by ConfirmDialog |

### Shared Components
| Component | Purpose |
|-----------|---------|
| CommandPalette | Cmd+K, cmdk + fuse.js fuzzy search: Navigation/Actions/Workspaces/Symbols/Recent |
| DocDrawer | Slide-in: metadata kv, supersession chain, SafeMarkdown + WikilinkRewriter, BacklinksList, Edit/Copy/Delete |
| ConfirmDialog | Focus trap, Esc cancel, typed-confirm for destructive ops |
| SafeMarkdown | react-markdown + rehype-sanitize, strict allow-list |
| WikilinkRewriter | `[[target]]` → anchor, 4 states: resolved/ambiguous/broken/escaped |
| BacklinksList | "Referenced by" list, clickable rows swap drawer |
| WorkspaceSelector | Dropdown, persists to localStorage, URL query param |
| NonLoopbackBindBanner | Persistent red banner when server.host ∉ {localhost, 127.0.0.1, ::1} |
| Mnemonic shortcuts | `g d/m/g/s/h/,` two-keystroke nav within 800ms |

### API Endpoints Used by UI
| Method | Endpoint | UI Consumer |
|--------|----------|-------------|
| GET | `/api/v1/workspaces` | WorkspaceSelector |
| GET | `/api/v1/stats?workspace=<h>` | DashboardPanel |
| GET | `/api/v1/doctor` | SettingsPanel |
| GET | `/api/v1/config` | SettingsPanel |
| POST | `/api/v1/config` | SettingsPanel (patch) |
| GET | `/api/v1/tags?workspace=<h>` | MemoryPanel tag filter |
| GET | `/api/v1/symbols?workspace=<h>&query=...` | SymbolsPanel, CommandPalette |
| POST | `/api/v1/query` | MemoryPanel search |
| POST | `/api/v1/search` | MemoryPanel BM25 |
| POST | `/api/v1/get` | DocDrawer |
| POST | `/api/v1/write` | DocDrawer edit |
| POST | `/api/v1/graph/neighborhood` | GraphPanel |
| GET | `/api/v1/links/:doc_id/backlinks` | BacklinksList |
| GET | `/api/v1/links/resolve?query=<t>` | WikilinkRewriter |
| GET | `/api/v1/events?workspace=<h>` | SSE: embed_queue, reindex, harvest, watcher |
| POST | `/api/v1/reindex` | HarvestPanel trigger |
| POST | `/api/harvest` | HarvestPanel trigger |
| DELETE | `/api/v1/workspaces/:hash` | SettingsPanel |
| POST | `/api/v1/reset-workspace` | SettingsPanel |

### Security Surface
| Mechanism | Implementation |
|-----------|---------------|
| CSRF | 7-step decision middleware on POST/PUT/DELETE; `X-Requested-With: nano-brain-ui` header bypass |
| XSS | rehype-sanitize strict allow-list for all markdown content |
| CSP | `default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'` |
| X-Frame-Options | DENY |
| X-Content-Type-Options | nosniff |
| Referrer-Policy | same-origin |
| Bind safety | Startup refuses non-loopback without `--unsafe-no-auth`; red banner if violated |
| Config redaction | Database URL, API keys returned as `<redacted>` |

---

## 2. Risk Assessment

### High Risk (P0)
1. **XSS via wikilinks/markdown**: User-controlled content (`[[<script>alert(1)</script>]]`) rendered in DocDrawer
2. **CSRF on destructive endpoints**: reset-workspace, delete workspace, write doc — must require `X-Requested-With`
3. **Config secrets exposure**: `GET /api/v1/config` must redact DATABASE_URL, API keys
4. **Bind safety enforcement**: Non-loopback binding without `--unsafe-no-auth` must be blocked or warned

### Medium Risk (P1)
5. **SSE reconnect on server restart**: Browser must auto-reconnect within 3s
6. **Graph rendering with large neighborhoods**: ≤500 node cap, truncation affordance
7. **Tag filter AND logic correctness**: `?tags=a,b` must show only docs with BOTH tags
8. **localStorage quota exhaustion**: Position cache, recent searches, workspace state
9. **Concurrent SSE subscriber cap**: 8 per-IP limit, 429 on 9th
10. **Backlinks accuracy**: Wikilink extraction idempotent, cache invalidation on write

### Lower Risk (P2)
11. **Bundle size**: Target < 600 KB gzip (app) + ~150 KB lazy (graph)
12. **First paint time**: Dashboard < 500ms warm cache
13. **Vietnamese locale in content**: Unicode handling in search, display, wikilinks
14. **Browser back/forward with URL-synced state**: tags, doc, workspace params
15. **Very long doc titles/tags**: Layout overflow handling
16. **Empty states**: No docs, no tags, no symbols, no workspaces
17. **Network throttling**: Slow 3G behavior for all panels

---

## 3. Test Environment Setup

### Server Status (Verified)
- **Server**: Running on host port 3100 ✅
- **PostgreSQL**: Healthy (migration v12) ✅
- **pgvector**: 0.8.2 ✅
- **Embedding**: Ollama (nomic-embed-text) ✅
- **Workspace count**: 18 workspaces registered ✅
- **Test workspace**: `nano-brain` hash `7f443561795a6fea64b6e8d35a9b06ed4d216b8a27af5e10e7137b261ade061f` (6372 docs)

### Test Data Available
- 6372 documents in `nano-brain` workspace
- 3427 symbols (go, python, typescript, javascript)
- 18859 embedded chunks, 424 pending, 2 failed
- 14789 graph edges (calls: 10336, contains: 2685, imports: 1768)
- 110 session summaries, 5 memory notes
- Tags: symbol (3427), go (2582), function (1846), python (686), etc.

### Testing Tools
- **curl**: API endpoint testing (primary) ✅
- **Browser automation**: Not available in container (Playwright deps missing) ❌
- **UI testing**: Manual via host browser at `http://localhost:3100/ui` (MANUAL-REQUIRED)
- **SSE testing**: `curl -N` with timeout ✅
- **File integrity**: `ls` / `stat` on embedded dist/ ✅

### CSRF Headers for Testing
```bash
# Mutating requests need:
-H "X-Requested-With: nano-brain-ui"
-H "X-Workspace-Hash: <hash>"
-H "Content-Type: application/json"
```

### SSE Verified
- `GET /api/v1/events?workspace=<hash>` → receives `hello` event within 100ms ✅
- Heartbeat comments every 30s (expected)

---

## 4. Spec Coverage Map

| Spec File | Covers |
|-----------|--------|
| `web-ui-app/spec.md` | All 6 panels, components, routing, accessibility, keyboard nav, wikilinks, backlinks, graph modes, command palette |
| `web-ui-server/spec.md` | /ui SPA serving, /api/v1/config, /api/v1/doctor, /api/v1/stats, /api/v1/graph/neighborhood, /api/v1/links/*, security headers, CSRF middleware, bind safety |
| `web-ui-streaming/spec.md` | SSE endpoint, event types (hello/embed_queue/reindex/harvest/watcher/lag), backpressure, subscriber cap, heartbeat, auto-reconnect |
| `design.md` | Architecture decisions, tech stack, component hierarchy, data flow |
| `tasks.md` | Implementation order, story breakdown, acceptance criteria |
