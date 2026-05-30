# Tasks — Web UI

## Pre-implementation
- [ ] Run deep-design (Metis + Oracle) on `proposal.md` + `design.md`. Revise until clean pass. Capture report in `docs/evidence/deep-design-web-ui.md`.
- [ ] Confirm `huashu-design` hi-fi mockup is approved by the user (linked from this change folder under `mockup/`).
- [ ] Create GitHub issue per `docs/FEATURE_INTAKE.md`. Label: `lane:high-risk`, `change-type:user-feature`, `area:web-ui`.
- [ ] Confirm binary size baseline (`ls -l ./nano-brain`) so the +8 MB budget can be enforced post-merge.

## Implementation

### Backend — schema migration

- [ ] `migrations/0001N_graph_edge_references.sql` (next free goose number):
  - [ ] Up: drop existing `graph_edges_edge_type_check` CHECK; add new CHECK allowing `('contains','imports','calls','references')`.
  - [ ] Down: drop new CHECK; restore old `('contains','imports','calls')`. Comment in Down: "this will fail if 'references' rows exist; operator must run `DELETE FROM graph_edges WHERE edge_type='references'` first".
- [ ] Verify `internal/storage/sqlc/graph.sql.go` queries continue to work unchanged (no enum-coupling on the Go side).
- [ ] Update sqlc.yaml / migration regen if needed (probably no — CHECK changes don't affect generated code).
- [ ] Add integration test: insert all four edge types, query each by edge_type, verify counts.

### Backend — link extractor package

- [ ] Create `internal/links/` package, zero internal deps (mirroring `internal/eventbus/` pattern):
  - [ ] `internal/links/parse.go` — `Parse(content string) []Link`. Single-pass scanner. Handles escape sequences (`\[[...]\]` → literal). Rejects multi-line / nested / >200-char content inside brackets.
  - [ ] `internal/links/parse_test.go` — table-driven: simple ID, simple title, mixed, escaped, malformed, empty, edge cases (`[[` without close, `[[[[a]]`, unicode titles).
  - [ ] `internal/links/extract.go` — `Extractor{queries, bus}` with `Extract(ctx, doc)`. Steps: parse → resolve each unique target → diff against existing `references` edges → delete stale + upsert new in a single transaction → publish `links_changed` event on bus.
  - [ ] `internal/links/resolve.go` — title→IDs cache (LRU 10k titles per workspace) invalidated on doc write/update/delete.
- [ ] sqlc queries:
  - [ ] `ListDocIDsByTitle(workspace, lower_title)` — case-insensitive title lookup.
  - [ ] `ExistsDocByID(workspace, id)` — fast existence check.
  - [ ] `ListReferenceEdgesBySource(workspace, source_node)` — for idempotent re-extract.
  - [ ] `UpsertReferenceEdge(workspace, source, target, source_file, metadata)` — uses existing unique constraint.
  - [ ] `DeleteReferenceEdgesBySource(workspace, source_node)`.

### Backend — wire link extractor

- [ ] `internal/server/handlers/write.go` — after a successful write, call `resolver.FlushWorkspace(wh)` THEN `Extractor.Extract`. Capture extractor error as warn (don't fail the write if extraction has an issue).
- [ ] `internal/server/handlers/reindex.go` — after each upserted memory/summary document, call `resolver.FlushWorkspace(wh)` THEN run extraction.
- [ ] `internal/harvest/harvest.go` — after each session summary insert, call `resolver.FlushWorkspace(wh)` THEN run extraction.
- [ ] Watcher path: do NOT run extraction on source-code file changes (extraction is scoped to `collection IN ('memory','session-summary:%')`).
- [ ] **Test (Oracle Q4): cache-staleness scenario.** Write doc A with title "Foo". Write doc B containing `[[Foo]]`. Update doc A's title to "Bar". Verify the next write of doc C containing `[[Bar]]` resolves correctly (i.e., the rename took effect — the cache was flushed before extraction). Currently, doc B's existing `references` edge to A is NOT updated by A's rename; that's by design (edges are over doc IDs, not titles). Test asserts: B → A edge persists; C → A edge appears.

### Backend — new endpoints (extended)

#### Server scaffolding
- [ ] Add `internal/server/webui/` package.
  - [ ] `webui.go` — `RegisterUIRoutes(e *echo.Echo, embedFS embed.FS, bus eventbus.Publisher)` mounts `/ui` and `/ui/*` with SPA fallback and applies the **security headers middleware** (CSP, X-Content-Type-Options, X-Frame-Options, Referrer-Policy) scoped to `/ui` only.
  - [ ] `dist/` directory with `.gitkeep` so `//go:embed dist/*` compiles even when frontend is not built (CI must build first).
  - [ ] **Missing-UI fallback** (Oracle): at startup, check `embedFS` for `dist/index.html`. If absent, register `/ui` route to serve a small hard-coded instructional HTML page (see design.md). The REST API SHALL keep functioning.
- [ ] Add the bind-safety check in `cmd/nano-brain/main.go serve` path: if `server.host` is non-loopback and `--unsafe-no-auth` is not set, refuse to start with explanatory error.

#### Event bus (REVISED per Oracle)
- [ ] **Create new standalone package `internal/eventbus/`** with zero internal dependencies (avoids import cycle):
  - [ ] `internal/eventbus/bus.go` — `Bus`, `Event`, `Publisher` interface, fan-out goroutine pattern.
  - [ ] **Drop-newest (non-blocking send), not drop-oldest** — race-free; periodic `lag` event ticker (5s) emits `{dropped: N}` to subscribers with non-zero drop counter and resets.
  - [ ] `Subscribe(workspace string)` performs workspace filtering at SSE read site, not at publish.
  - [ ] `Close()` performs graceful shutdown: cancel ctx → drain incoming → close all subscriber channels.
  - [ ] Unit tests covering: Publish under contention, slow subscriber drops, lag event emission, Subscribe/unsubscribe lifecycle, graceful Close.

#### SSE handler
- [ ] `internal/server/handlers/events.go` — `GET /api/v1/events`. Sets SSE headers, subscribes to bus with workspace filter, writes `data: <json>\n\n` + flushes, emits `:` heartbeat comment every 30 s, exits on `c.Request().Context().Done()`.
- [ ] Per-IP subscriber cap (8) + idle reaper (5 min, reset by heartbeat).

#### Wire producers to bus (constructor injection of `eventbus.Publisher`)
- [ ] `internal/embed/queue.go` — accept `eventbus.Publisher` in constructor; publish `embed_queue` event on enqueue/dequeue (debounced 500 ms inside Publisher wrapper, not at call sites).
- [ ] `internal/watcher/watcher.go` — accept `eventbus.Publisher`; publish `watcher` event on file change (rate-limited 10/sec per workspace).
- [ ] `internal/server/handlers/reindex.go` — accept `eventbus.Publisher`; publish `reindex` events at started/progress/completed/failed.
- [ ] `internal/harvest/harvest.go` — accept `eventbus.Publisher`; publish `harvest` events at started/progress/completed.
- [ ] `internal/server/server.go` — instantiate `eventbus.New(ctx)` at startup, pass `Publisher` interface into every producer constructor.

#### Config endpoints
- [ ] `internal/server/handlers/config.go`
  - [ ] `GET /api/v1/config` — return resolved config, redact secrets (use new `internal/config/secrets.go` listing).
  - [ ] `POST /api/v1/config` — accept partial patch, validate against allowlist + zod-equivalent Go validator, write back to file, trigger existing reload.
- [ ] `internal/config/secrets.go` — central list of secret field paths (`database.url`, `embedding.voyage_api_key`, `summarization.api_key`).
- [ ] `internal/config/patch.go` — safe-patch allowlist + YAML-preserving writeback (use `yaml.Node` to keep comments).

#### Doctor endpoint
- [ ] `internal/server/handlers/doctor.go` — `GET /api/v1/doctor` returning `[]{name, status, detail, hint}`.
- [ ] Refactor existing `cmd/nano-brain/doctor.go` checks into pure functions in `internal/health/doctor/` so both CLI and HTTP handler call them.

#### Stats endpoint
- [ ] `internal/server/handlers/stats.go` — `GET /api/v1/stats` aggregating: doc counts, chunk counts by `embed_status`, embeddings total, graph_edges by type, collections summary, top tags (LIMIT 20), recent docs (LIMIT 10), recent queries from telemetry (LIMIT 20).
- [ ] Add sqlc queries: `CountDocumentsByCollection`, `CountChunksByEmbedStatus`, `CountGraphEdgesByType`, `ListTopTags`, `ListRecentDocuments`, `ListRecentQueries`.

#### Graph neighborhood endpoint (dual-mode)
- [ ] `internal/server/handlers/graph_neighborhood.go` — `POST /api/v1/graph/neighborhood`.
  - [ ] Accepts: `{focus, depth (1-5), direction (in/out/both), edge_types [], workspace, node_kind: "symbol"|"doc" (default "symbol")}`.
  - [ ] Runs BFS using existing `GetOutgoingEdges` / `GetIncomingEdges` + new `GetEdgesByNodes` (batch).
  - [ ] Hard cap 500 nodes; returns `{node_kind, nodes, edges, truncated, frontier_nodes}`.
  - [ ] When `node_kind="doc"`, **also** issue a `ListDocumentsByIDs` query to enrich nodes with `title`, `collection`, `updated_at`, `tags` so the SPA doesn't need a second round-trip per node.
  - [ ] When `node_kind="doc"`, server filters `edge_types` to `["references"]` even if client sends other values (defense in depth).
  - [ ] Validate `node_kind`; reject with 422 if invalid.
  - [ ] Add sqlc queries: `GetEdgesByNodes(workspace, nodes[], edge_types[])`, `ListDocumentsByIDs(workspace, ids[])`.

#### Backlinks + resolve endpoints (NEW)
- [ ] `internal/server/handlers/links.go`:
  - [ ] `GET /api/v1/links/:doc_id/backlinks` — paginated `{doc_id, backlinks, total}`. Snippet extraction: find first wikilink occurrence in source content, return ±100 chars.
  - [ ] `GET /api/v1/links/resolve` — `?workspace=<h>&query=<title-or-id>` returns `{matched, ambiguous, kind}`. Routes through `internal/links/resolve.go` cache.
  - [ ] Both endpoints scope strictly to workspace; never return cross-workspace edges/docs.
- [ ] sqlc queries:
  - [ ] `ListBacklinksByTarget(workspace, target_node, limit, offset)` — returns `(source_doc.id, title, collection, updated_at, tags, source_doc.content)` joined on `graph_edges WHERE edge_type='references' AND target_node = $1`.
  - [ ] `CountBacklinksByTarget(workspace, target_node)`.

#### CSRF middleware (REVISED per Oracle — 7-step decision order)
- [ ] `internal/server/middleware.go` — add `csrfMiddleware(boundAddr string)` with the 7-step decision order from `web-ui-server/spec.md`:
  1. `X-Requested-With: nano-brain-ui` → allow
  2. Origin AND Referer both absent → allow (CLI/MCP)
  3. Origin: null → reject
  4. Origin same-host as boundAddr → allow
  5. Origin different host → reject
  6. Origin absent + Referer same-host → allow
  7. Otherwise → reject
- [ ] Mount on `/api/v1/*` POST/PUT/DELETE only; do NOT apply to `/health`, `/api/status`, `/sse`, `/mcp`.
- [ ] Unit-test all 7 decision branches plus loopback variations (`127.0.0.1`, `localhost`, `::1`).

### Frontend (web/)

#### Scaffolding
- [ ] Create `web/` directory. Add `package.json`, `vite.config.ts`, `tsconfig.json`, `.eslintrc.cjs`, `.prettierrc`.
- [ ] Add `npm` deps: react, react-dom, @tanstack/react-router, @tanstack/react-query, @tanstack/react-table, @tanstack/react-virtual, sigma, graphology, graphology-layout-forceatlas2, cmdk, react-hook-form, zod, sonner, lucide-react, date-fns, react-diff-viewer-continued, prismjs, **react-markdown, rehype-sanitize** (Oracle: XSS defense), **remark-wiki-link** (or equivalent — for `[[…]]` parsing; if no maintained package fits, hand-roll a tiny rehype plugin: 30–50 LOC).
- [ ] **Do NOT add Shiki** (Oracle: blows bundle budget; Prism covers v1).
- [ ] Add dev deps: typescript, vite, @vitejs/plugin-react, eslint, prettier, vitest, @testing-library/react.
- [ ] `vite.config.ts` — build target `internal/server/webui/dist`, hashed asset filenames, proxy `/api/*` and `/sse` to `http://localhost:3100` in dev.
- [ ] `index.html` — minimal, `<div id="root">`, no third-party `<link>` or `<script>`.
- [ ] Add `web/README.md` documenting dev + build workflows.
- [ ] Add `Makefile` targets: `make web-install`, `make web-dev`, `make web-build`, `make web-check`. Hook `make build` to depend on `make web-build`.

#### Core infrastructure
- [ ] `web/src/main.tsx` — entry, router, QueryClient, Toaster, ThemeProvider.
- [ ] `web/src/app/router.tsx` — TanStack Router routes for `/`, `/dashboard`, `/memory`, `/memory/:id`, `/graph`, `/symbols`, `/harvest`, `/settings`.
- [ ] `web/src/app/layout.tsx` — sidebar + main + cmd-K mount point.
- [ ] `web/src/app/theme.ts` — CSS variables, dark/light toggle, persisted to localStorage.
- [ ] `web/src/api/client.ts` — fetch wrapper adding `X-Workspace-Hash` and `X-Requested-With: nano-brain-ui` headers.
- [ ] `web/src/api/types.ts` — TS interfaces mirroring Go response shapes (manual sync; documented in design.md).
- [ ] `web/src/hooks/useEvents.ts` — single EventSource subscription, multiplex into Zustand store, expose typed selectors.
- [ ] `web/src/hooks/useStats.ts` — TanStack Query wrapper for `/api/v1/stats`.

#### Components (shared)
- [ ] `WorkspaceSelector` — fetches `/api/v1/workspaces`, persists choice, updates URL.
- [ ] `CommandPalette` — cmdk-based, fuzzy search across routes/actions/recent/workspaces/symbols.
- [ ] `StatusBadge` — ok/warn/fail/pending.
- [ ] `KeyHint` — renders `Cmd+K` / `Ctrl+K` based on platform.
- [ ] `TagChip`, `TagFilter` — multi-select with AND semantics.
- [ ] `DocDrawer` — slide-in detail view for documents.
- [ ] `Skeleton` — layout-matching loading placeholders.
- [ ] **`SafeMarkdown`** (Oracle: XSS defense) — wraps react-markdown + rehype-sanitize with a strict allow-list (p, a[rel=noopener], code, pre, ul, ol, li, h1-h6, blockquote, em, strong). All memory note content rendered in the UI MUST go through `<SafeMarkdown>`. Metadata JSONB renders as `<pre><code>` text-only. **Composes with `<WikilinkRewriter>` (below) — sanitize first, then rewrite `[[…]]` into anchors.**
- [ ] **`WikilinkRewriter`** — a remark/rehype plugin (or post-pipeline DOM walker) that scans rendered text nodes for `[[<target>]]` patterns and rewrites them into `<a data-doc-id="…">` elements. Calls a `useResolve(workspace, query)` hook (TanStack Query, cached) that hits `GET /api/v1/links/resolve`. Renders four states: resolved-link / ambiguous (plain text + dotted underline + tooltip listing candidates) / broken (plain text + dotted underline + "no match" tooltip) / escaped (literal).
- [ ] **`BacklinksList`** — drawer subcomponent. On drawer open for doc `D`, fetches `GET /api/v1/links/:D/backlinks?workspace=…`. Renders rows (title · collection · age · snippet) using the same row layout as `MemoryPanel`. Clicking a row swaps the drawer content to that referencing doc. Empty state: "No other documents reference this yet."

#### Panels
- [ ] **DashboardPanel** — calls `/api/v1/stats` + subscribes to SSE for live embed_queue/reindex/harvest.
- [ ] **MemoryPanel** — TanStack Table over paginated `/api/v1/query` (text filter), `/api/v1/tags` + filter logic; row click → `DocDrawer`.
- [ ] **MemoryDocDetail** (inside drawer) — full content rendered via `<SafeMarkdown>` + `<WikilinkRewriter>` so `[[…]]` becomes clickable, metadata JSONB tree, supersedes chain (vertical timeline via `react-diff-viewer-continued` for visual diff), **`<BacklinksList>` section ("Referenced by")**, edit/delete/copy actions.
- [ ] **GraphPanel** — Sigma + Graphology, **mode toggle (Code / Knowledge)**, focus input, depth slider, direction toggle, edge-type filter; calls `/api/v1/graph/neighborhood` with `node_kind` set from current mode; localStorage position cache keyed by `(workspace, focus, depth, direction, edge_types, mode)` (mode is part of the key — positions don't bleed across modes). Node color: Code mode → primary outgoing edge_type; Knowledge mode → document collection. Double-click in Knowledge mode opens `DocDrawer` in place. Mode switch clears focus input but remembers per-mode focus for restoration.
- [ ] **SymbolsPanel** — type-ahead via `/api/v1/symbols`, kind+language filters, "Show in graph" cross-link.
- [ ] **HarvestPanel** — list summaries via `/api/v1/query?collection=session-summary*`, trigger button → `/api/harvest`, SSE progress. **Clicking a session row opens the same `DocDrawer` used by Memory panel**, so summaries render as full sanitized Markdown with inline wikilinks + a "Referenced by" backlinks list — Obsidian-style. Harvest panel itself stays focused on the list + trigger UX; summary reading happens in the drawer.
- [ ] **SettingsPanel** — `useConfig` hook over `/api/v1/config`, react-hook-form + zod, save → `POST /api/v1/config`; Doctor section via `/api/v1/doctor`.

#### Keyboard + accessibility
- [ ] Global hotkeys: Cmd/Ctrl+K (palette), `g d/m/g/s/h/,` (panel nav), `?` (cheatsheet).
- [ ] Focus ring CSS via `:focus-visible` with high-contrast outline.
- [ ] Modal focus trap (cmdk handles this, verify also for DocDrawer).
- [ ] Manual a11y check: keyboard-only walkthrough of every panel.

### Tests

#### Backend
- [ ] Unit: each new handler has table-driven tests (config get/post/patch validation, stats counts, neighborhood depth/cap/truncation/node_kind, doctor structured output, events bus subscribe/publish/lag, **links.Parse over a corpus of valid+malformed wikilinks**, **resolver title/id/ambiguous/none paths**, **backlinks endpoint pagination + snippet generation**).
- [ ] Unit: CSRF middleware accepts CLI-style requests, rejects browser-style without header.
- [ ] Integration (build tag `integration`): live PG + nano-brain — start server, run migration, hit each new endpoint with HTTP client, assert shapes + status codes. Specifically: **write a document containing `[[<other-id>]]`, assert a `references` edge appears in `graph_edges`, then GET backlinks of the other doc and assert the writer is listed**.
- [ ] SSE integration: subscribe to `/api/v1/events`, trigger reindex, assert sequence of events with timeouts.
- [ ] Migration: run Up on a populated workspace, verify CHECK allows `'references'`; insert one row; run Down (expect failure); delete `references` rows; run Down again (expect success); run Up again (idempotent).

#### Frontend
- [ ] Vitest: component tests for `CommandPalette` fuzzy matching, `WorkspaceSelector` persistence, `useEvents` reconnect logic, `MemoryPanel` filter URL sync.
- [ ] Playwright (new): smoke test — load `/ui`, switch workspace, navigate panels via keyboard, run a search, open a doc detail, save a config change.

### Docs
- [ ] Update `CHANGELOG.md` — `[Unreleased]` ### Features: Web UI; ### Added: new endpoints; ### Security: CSRF middleware, bind-safety check.
- [ ] Update `README.md` — new section "Web UI" with screenshots from the huashu-design mockup, dev workflow, security notes.
- [ ] Update `docs/ROADMAP.md` — mark Pillar 3 as having a UI; note v2 stretch goals (auth, embedding viz).
- [ ] Add `web/README.md` — dev workflow, design tokens, contributing.
- [ ] Add `docs/architecture/web-ui.md` — link to design.md, sequence diagrams (Mermaid validated via `mermaid-validator`).

### Self-review evidence
- [ ] `docs/evidence/self-review-feat-web-ui.md` — binary size delta measurement, screenshot of each panel, network audit (third-party request count = 0), keyboard-only walkthrough recording or notes, Lighthouse score.
- [ ] Cross-browser smoke: Chrome latest, Firefox latest, Safari latest. Note any rendering quirks.

## Validation ladder (per `docs/HARNESS.md` high-risk lane)

- [ ] `validate:quick` — `go build ./... && go test -race -short ./...`
- [ ] `self-review:response-shape` — confirm every new handler's response struct fields all populated.
- [ ] `self-review:staged-files` — `git status` shows no `.opencode/`, no `web/dist` (ignored), no `package-lock.json` from accidental npm install at repo root.
- [ ] `test:integration` — `go test -race -tags=integration ./...`
- [ ] `smoke:e2e` — Playwright suite on built binary.
- [ ] Manual QA — exercise each panel, each shortcut, each form.

## Post-merge
- [ ] `openspec archive web-ui`.
- [ ] Close GitHub issue with PR link.
- [ ] Tag release with binary size before/after in release notes.
- [ ] File follow-up issues for v2: auth (`#TBD`), embedding viz (`#TBD`), mobile layout (`#TBD`).

## Forward-compat verifications
- [ ] Auth-ready: confirm `/ui` mount point can be wrapped in middleware later without restructuring.
- [ ] Embedding viz: confirm `/api/v1/stats` shape can grow a `projection` field without breaking existing clients (additive JSON).
- [ ] Multi-user: confirm no UI code assumes a single user — every query is workspace-scoped.
