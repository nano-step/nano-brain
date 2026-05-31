# Web UI for nano-brain

## Issue
TBD — create at intake gate. Suggested title: `feat(web-ui): operator dashboard for memory, graph, symbols, status`.

## Lane
**high-risk** — introduces a new user-facing surface, ships static assets via `embed.FS`, adds streaming endpoints (SSE), exposes mutating operations (write, delete, reset) over a browser-reachable channel, **and now adds a new edge type `references` to `graph_edges` (requires a migration)**. Touches hard gates: `public-api-contract` (new endpoints), `authorization` (browser-origin access control), `data-model` (migration extends `graph_edges.edge_type` CHECK constraint).

## Change Type
`user-feature`

## Why
nano-brain today is CLI + MCP + HTTP-API only. A power user has no way to:
1. **See what they have** — memory notes, tags, supersedes chains, harvested sessions, symbol graph, embedding queue health.
2. **Control the system live** — trigger reindex, harvest, summarize; reload config; remove workspaces; reset embeddings.
3. **Explore relationships** — symbol callers/callees, impact propagation, knowledge graph edges (contains/imports/calls), **and now: memory↔memory + session-summary↔memory cross-references**.
4. **Debug the index** — failed embeddings, orphan symbols, supersession history, telemetry of past queries.
5. **Navigate session summaries like a knowledge base** — current Harvest list shows only previews; users want Obsidian-style inline `[[wikilinks]]` between summaries and a backlinks panel so a single session digest becomes a navigable node in a personal knowledge graph.

CLI gives one row at a time. MCP is agent-only. `/api/status` is a JSON blob. There is no human-readable surface for the rich data nano-brain already stores.

## Desired Outcome
A single-binary nano-brain ships a built-in web UI at `http://localhost:3100/ui` that:
- Lives in the same binary (no separate frontend deploy, no node runtime at user machines).
- Reads from existing REST endpoints; adds new endpoints **only** where existing CLI/HTTP cannot answer the question (specifically: live progress streams, stats aggregations, graph neighborhoods, **memory/summary link extraction + backlinks queries**).
- Provides 6 first-class panels: **Dashboard / Memory / Graph / Symbols / Harvest / Settings**.
- The **Graph** panel has TWO modes: (a) **Code mode** — symbols + contains/imports/calls (the original v1 design); (b) **Knowledge mode** — memory notes + session summaries as nodes, with `references` edges extracted from `[[wikilinks]]` in document content. Single panel, mode toggle in the toolbar.
- The **Memory drawer** and **Harvest summary viewer** render document content as sanitized Markdown with **clickable `[[doc-id]]` and `[[Title]]` wikilinks** that open the target document's drawer in place. A persistent **"Referenced by"** section in every drawer lists incoming references (backlinks).
- Keyboard-first (Cmd+K palette), monospace-where-it-matters, dense-but-scannable — explicitly NOT a generic Tailwind admin template.
- Works offline; no external CDN; no telemetry beacons.

## Constraints
- **Single binary**: assets embedded via `//go:embed`. Total binary size increase ≤ 8 MB.
- **No new heavy backend deps**: reuse Echo, sqlc, pgvector, embedder. New deps allowed: tiny SSE helper if needed (likely just `http.Flusher`).
- **One forward-only migration allowed**: extends `graph_edges.edge_type` CHECK constraint to add `'references'`. No data backfill required (extractor populates on the fly during reindex/write). Old code remains compatible because the existing three edge types are preserved.
- **No auth in v1** (binds to `localhost` only; matches current posture). v2 may add token auth for non-loopback binds — out of scope here but spec'd as forward-compat.
- **Backward compatible (with one exception, above)**: every existing endpoint, CLI command, MCP tool stays. UI is additive. The migration is forward-only and safe to apply on an empty / partial / full `graph_edges` table.
- **Performance budget**: first paint < 500ms on localhost; query result render < 100ms for ≤200 rows; graph render smooth at 5k nodes. **Knowledge-mode graph hard-capped at 500 nodes** (same as code mode) to prevent hairball at large workspaces — extractor populates frontier nodes per neighborhood request.
- **Browser support**: latest Chrome, Firefox, Safari (last 2 versions). No IE/legacy.
- **Offline-first**: works without internet (no Google Fonts, no CDN scripts).

## Out of Scope (v1)
- Authentication / multi-user / RBAC (single-user, localhost-only).
- Editing source files in the index (read-only on indexed code; memory notes ARE editable).
- Diff-merge for supersession chains beyond visual side-by-side.
- Mobile-optimized layout (desktop-first; mobile is "view, don't drive").
- Theming beyond dark/light toggle.
- i18n (English only).
- Embedding visualizations (UMAP scatter) — deferred to v2; spec'd as forward-compat hook.
- Auto-execution / proactive agent suggestions (Pillar 4 of ROADMAP).

## Acceptance Criteria

1. **Single-binary delivery**: `nano-brain serve` exposes the UI at `/ui` with no extra steps. Build produces a single binary with all assets embedded. Binary size delta ≤ 8 MB.

2. **Dashboard panel** loads in under 500ms on a warm cache and shows: server version, uptime, embedding provider/model, workspace selector, doc count, chunk count (with embed_status breakdown), graph_edges count, harvest mode + last harvest time, embed queue depth (live via SSE), 10 most recent memory notes.

3. **Memory panel** lists documents for the selected workspace with: title, tags (multi-select filter), collection, created_at, supersedes chain indicator. Supports text filter (BM25), tag filter, collection filter. Clicking a row opens a detail drawer showing full content, metadata JSONB pretty-printed, supersedes/superseded-by chain (full lineage), and actions: edit (new doc that supersedes), delete, copy ID, add/remove tags.

4. **Graph panel** renders an interactive knowledge graph for the selected workspace via Sigma.js + Graphology. User can: pick a focus node (function/type by name), choose depth (1/2/3), choose direction (in/out/both), choose edge types (contains/imports/calls — multi-select). Hover shows source file + line. Click expands neighborhood. Layout positions persist per workspace in localStorage. Renders smoothly at 5k visible nodes.

5. **Symbols panel** type-ahead search across symbols (collection `symbol:*`); filter by kind (function/method/type/interface/struct/const/var) and language (go/python/typescript/javascript); jump to source path:line; show signature + impact count (incoming edges). Click → opens Graph panel focused on that symbol.

6. **Harvest panel** lists harvested sessions (collection `session-summary:*`) with source/agent/duration/created_at, summary preview, link to raw session via supersedes chain. Trigger harvest button posts to `/api/harvest`; live progress shown via SSE.

7. **Settings panel** reads current config from `/api/v1/config` (new), allows editing safe fields (search.rrf_k, search.recency_weight, watcher.debounce_ms, summarization.* except api_key), POSTs to `/api/reload-config`. Read-only display for secrets (api_key shown masked). Doctor diagnostics (`/api/v1/doctor` — new) shown as live health checks.

8. **Cmd+K command palette** opens with Ctrl/Cmd-K, indexes: every navigation route, every action (trigger reindex, trigger harvest, reload config), recent searches (last 20, localStorage), every workspace, every symbol matching typed query. Mnemonic shortcuts: M=memories, G=graph, S=symbols, H=harvest, ⚙=settings.

9. **Real-time updates** via SSE on `/api/v1/events` (new). Streams: embed_queue_depth, reindex_progress, harvest_progress, watcher events. UI auto-reconnects on disconnect.

10. **No regression**: all existing CLI commands, REST endpoints, MCP tools, and tests pass unchanged. `nano-brain status`, `nano-brain query`, etc. continue to work identically.

11. **Offline + no external CDN**: full UI loads with network disabled after first asset fetch. No `<script src="https://...">` or `<link href="https://...">` from third-party hosts.

12. **Accessibility floor**: keyboard navigable for primary flows (Dashboard, Memory list+detail, Symbols search, Settings). Focus rings visible. Modal dialogs trap focus. Color contrast WCAG AA on text.

13. **Knowledge-mode graph**: the Graph panel has a mode toggle in its toolbar with two options: **Code** (default — symbols + contains/imports/calls, as in AC4) and **Knowledge** (memory notes + session summaries as nodes + `references` edges). Knowledge mode reuses the same focus / depth / direction / edge-type controls. Both modes return ≤ 500 nodes from `POST /api/v1/graph/neighborhood`, just with a `node_kind=doc|symbol` discriminator in the request and response. Node colors in Knowledge mode encode document collection (memory / session-summary:opencode / session-summary:claudecode / symbol). Double-clicking a doc node opens that document's drawer.

14. **Inline wikilinks + backlinks**: rendered content of memory documents and session summaries SHALL support Obsidian-style inline references with TWO syntaxes:
    - `[[<doc-id>]]` — resolves to that specific document.
    - `[[<exact title>]]` — resolves to the most recent document in the workspace with that title (case-insensitive). Ambiguous titles render as plain text and the drawer warns of ambiguity with the list of candidates.
    
    Resolved wikilinks render as clickable links that open the target document's drawer in place (no full navigation). Unresolved wikilinks render as plain text underlined with a dotted style to indicate "broken link". Every drawer shows a persistent **"Referenced by"** section listing all documents whose content contains a `[[wikilink]]` resolving to this document (the backlinks). The list is workspace-scoped, sorted by most-recent updated_at, with the same row layout as the Memory panel list. Clicking a backlink opens that referencing document's drawer in place.

## References
- `docs/ROADMAP.md` — Pillar 3 "Memory & DX" already shipped; this is the human-facing layer.
- `docs/HARNESS.md` — gates apply; this is high-risk lane.
- Existing handlers: `internal/server/handlers/` — UI builds on these.
- Existing data model: see `migrations/00001..00009*.sql` — UI surfaces this without schema changes.
