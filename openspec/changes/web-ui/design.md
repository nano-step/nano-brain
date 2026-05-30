# Design — Web UI

## Approach

Ship a built-in single-page React app embedded into the nano-brain Go binary via `//go:embed`. The UI is served at `/ui/*` by the same Echo server that already serves `/api/v1/*`. The frontend is a Vite-built React 18 + TypeScript app, talks to existing REST endpoints, and gets live updates via a thin new SSE endpoint.

This is **deliberately conservative**: no separate frontend deployment, no auth surface (yet), no schema migrations. The hard part is the architecture seams (build pipeline, embed graph, SSE backpressure, graph viz perf), not the React app.

## Architecture

```
┌────────────────────────────────────────────────────────────────────┐
│                       nano-brain binary                            │
│                                                                    │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │ Echo HTTP Server :3100                                       │  │
│  │                                                              │  │
│  │   /ui/*       → http.FS(embedFS) (SPA static serve)         │  │
│  │   /api/v1/*   → existing handlers (unchanged)               │  │
│  │   /api/v1/events  → NEW: SSE stream multiplexer             │  │
│  │   /api/v1/config  → NEW: read current config                │  │
│  │   /api/v1/doctor  → NEW: live diagnostics                   │  │
│  │   /api/v1/stats   → NEW: aggregations for dashboard         │  │
│  │   /api/v1/graph/neighborhood → NEW: focused graph slice     │  │
│  │   /sse, /mcp  → existing MCP transports (unchanged)         │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                              │                                     │
│  ┌───────────────────────────┴─────────────────────────────────┐  │
│  │ internal/server/webui/                                       │  │
│  │   dist/        (embedded via //go:embed dist/*)              │  │
│  │   webui.go     (handler: serves index.html for SPA fallback) │  │
│  │   events.go    (SSE multiplexer + event bus)                 │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                              │                                     │
│           ┌──────────────────┼──────────────────┐                  │
│           ▼                  ▼                  ▼                  │
│       embed queue         watcher           harvester              │
│       (publishes        (publishes         (publishes              │
│        events)           events)            events)                │
└────────────────────────────────────────────────────────────────────┘

   web/ (sibling to internal/, NOT shipped in npm tarball except as built dist)
   ├── package.json     (vite, react, typescript, tanstack, sigma, etc.)
   ├── vite.config.ts   (build → ../internal/server/webui/dist)
   ├── tsconfig.json
   ├── index.html
   └── src/
       ├── main.tsx              (entry, router setup)
       ├── app/
       │   ├── router.tsx        (TanStack Router routes)
       │   ├── layout.tsx        (sidebar + main + cmd-K)
       │   └── theme.ts          (CSS variables, dark/light)
       ├── api/
       │   ├── client.ts         (fetch wrapper, SSE EventSource)
       │   └── types.ts          (mirror Go response shapes)
       ├── components/
       │   ├── CommandPalette.tsx
       │   ├── WorkspaceSelector.tsx
       │   ├── StatusBadge.tsx
       │   └── ...
       ├── panels/
       │   ├── DashboardPanel.tsx
       │   ├── MemoryPanel.tsx
       │   ├── GraphPanel.tsx
       │   ├── SymbolsPanel.tsx
       │   ├── HarvestPanel.tsx
       │   └── SettingsPanel.tsx
       └── hooks/
           ├── useEvents.ts      (SSE subscription)
           ├── useStats.ts       (TanStack Query for /stats)
           └── useGraph.ts       (Sigma + Graphology integration)
```

## Build Pipeline

1. **Dev**: `cd web && npm run dev` → Vite dev server on :5173, proxies `/api/*` to :3100. No embedding; pure HMR loop.
2. **Build**: `cd web && npm run build` → emits to `internal/server/webui/dist/`. The Go `//go:embed dist/*` picks it up.
3. **Go build**: `go build ./cmd/nano-brain` — vanilla. CI runs `npm ci && npm run build` before `go build`.
4. **CI gate**: `web/` lint + type-check + unit tests gate the Go build. `make web` target wraps both.

**Why Vite over Next.js**: We don't need SSR, edge routing, or any of Next's server-side surface. Vite ships a static SPA in 30 seconds; Next would force a Node runtime or complicated static export. The whole UI fits in <2 MB gzipped — Vite is the right tool.

## Tech Stack (frontend)

| Layer | Choice | Reason |
|---|---|---|
| Framework | React 18 | Industry default; TanStack ecosystem is React-first |
| Build | Vite 5 | Fast, native ESM, simple static output |
| Language | TypeScript 5 | Matches backend rigor; sqlc-like type safety |
| Routing | TanStack Router | Type-safe, file-free, integrates with TanStack Query |
| Data fetching | TanStack Query v5 | Stale-while-revalidate, optimistic updates, SSR-not-needed |
| Tables | TanStack Table v8 + react-virtual | Headless, 50k+ rows, full design control |
| Graph viz | Sigma.js 3 + Graphology | WebGL, 5k+ nodes smooth, Web Worker layouts |
| Forms | react-hook-form + zod | Tiny, type-safe schema validation |
| Styling | CSS Modules + vanilla CSS variables | Zero runtime, no Tailwind bloat, design-system via tokens |
| Icons | lucide-react | Tree-shaken SVG icons, no font dependency |
| Code highlight | Prism.js (lazy-loaded) | ~15 KB gzipped with TS/Go/Python/JS grammars; bundle budget fits. Shiki rejected for v1 (500+ KB WASM blows the 600 KB budget — Oracle finding). Shiki is a v2 candidate if bundle headroom appears. |
| Diff | react-diff-viewer-continued | GitHub-style supersedes chain rendering |
| Markdown rendering + XSS defense | react-markdown + rehype-sanitize | Memory note content is user-generated and may contain Markdown/HTML; all rendering goes through a `<SafeMarkdown>` wrapper (Oracle finding). ~12 KB combined. |
| Date/time | date-fns | Tiny, modular, no moment.js bloat |
| Cmd palette | cmdk (Radix) | Battle-tested, keyboard-first, headless |
| Toasts | sonner | Stack of one developer's good taste; no dep tree |

**Total bundle target**: < 600 KB gzipped initial + lazy-load Graph panel (Sigma adds ~150 KB gzip).

## Backend Changes

### New endpoints (additive only)

| Method | Path | Purpose |
|---|---|---|
| GET | `/api/v1/events` | SSE multiplexed event stream (workspace-scoped via query param) |
| GET | `/api/v1/config` | Return current resolved config (with secrets masked) |
| POST | `/api/v1/config` | Validate + apply partial config patch, then trigger existing reload |
| GET | `/api/v1/doctor` | JSON form of `doctor` CLI checks (PG, pgvector, ollama, model) |
| GET | `/api/v1/stats` | Workspace stats aggregation (doc counts by collection, tag distribution, embed_status breakdown, graph cardinality, telemetry top queries) |
| POST | `/api/v1/graph/neighborhood` | Return graph slice: focus node + depth + direction + edge_type filter + **`node_kind=symbol\|doc` discriminator** → nodes[] + edges[] (≤500 nodes hard cap). Default `node_kind=symbol` for backward-compat with v1 design. |
| GET | `/api/v1/links/:doc_id/backlinks` | **NEW** — returns the list of documents in the same workspace whose content contains a wikilink resolving to `doc_id`. Workspace-scoped. Paginated (`?limit=` default 50). |
| GET | `/api/v1/links/resolve?workspace=<h>&query=<title-or-id>` | **NEW** — resolve a wikilink target. Returns `{matched: [doc_id, ...], ambiguous: bool}` so the frontend can render the link correctly without a round-trip per click. |
| GET | `/ui` and `/ui/*` | Serve embedded SPA; SPA fallback to `index.html` for client routes |

### Existing endpoints — no change
All 30+ existing `/api/v1/*`, `/health`, `/api/status`, `/api/harvest`, `/api/reload-config`, `/sse` (MCP), `/mcp` continue to work identically. UI uses them via the same JSON shapes the CLI uses.

### Schema migration — `graph_edges.edge_type` extended

Today's `migrations/00008_knowledge_graph.sql` declares:

```sql
edge_type TEXT NOT NULL CHECK (edge_type IN ('contains', 'imports', 'calls'))
```

We add a forward-only migration (`migrations/0001N_graph_edge_references.sql`) that:

1. Drops the existing CHECK constraint by name (or by re-emitting the column constraint, depending on Postgres-version idiom — goose script handles this).
2. Adds a new CHECK constraint allowing `('contains', 'imports', 'calls', 'references')`.
3. No data backfill needed. No data is lost. Old code that only ever inserts the original three types continues to work. New rows of type `references` can be inserted only by the new link extractor and the new `/api/v1/graph/neighborhood` UI surface.

Rollback strategy: a Down migration restores the old CHECK. **This will fail if any `references` rows exist** — the operator must delete them first. That's deliberate; we don't want a silent loss of edges.

### Link extraction pipeline (`internal/links/`)

A new internal package, `internal/links/`, owns wikilink parsing and graph-edge upsert.

```go
// internal/links/links.go
package links

type Link struct {
    Raw       string  // "[[some title]]" or "[[d-1042]]"
    TargetRef string  // "some title" or "d-1042"
    Kind      string  // "title" | "id"
    Start     int     // byte offset in source content
    End       int
}

func Parse(content string) []Link

type Resolver interface {
    ResolveTitle(ctx context.Context, workspace, title string) ([]uuid.UUID, error)
    ResolveID(ctx context.Context, workspace string, id uuid.UUID) (bool, error)
}

type Extractor struct {
    queries  Resolver
    bus      eventbus.Publisher
}

// Extract reads a document's content, parses wikilinks, resolves each to target doc IDs,
// and upserts edges into graph_edges with edge_type='references'.
// source_node = doc.id, target_node = resolved doc.id, source_file = doc.source_path,
// metadata = {"raw_link": "...", "byte_offset_start": N, "byte_offset_end": M, "kind": "title|id"}.
func (e *Extractor) Extract(ctx context.Context, doc Document) error
```

**When extraction runs:**
1. `POST /api/v1/write` handler — after successful insert/update, run Extract synchronously (the doc is just-written; this is cheap).
2. `nano-brain reindex` — after each document is upserted.
3. `internal/harvest/harvest.go` — after each session summary is written.
4. NOT in the watcher hot-path debounce loop — files in indexed source code don't get treated as memory wikilinks; extraction is for `collection IN ('memory', 'session-summary:*')` only.

**Performance**: Parse is a single-pass regex/lexer over the document content. Resolve is one SQL query per unique link. Most memory notes have ≤10 wikilinks. Negligible cost.

**Idempotency**: Before upserting new edges, the extractor deletes existing `edge_type='references'` rows where `source_node = doc.id`. Re-extraction always yields a clean set.

**Cache invalidation ordering (Oracle Q4)**: the resolver's title→id LRU cache for a workspace SHALL be flushed BEFORE `Extractor.Extract` runs on a written / updated / deleted document. Otherwise, a write that changes a doc's title could leave stale cache entries that resolve wikilinks in OTHER documents to the wrong ID during the same extraction pass. Concrete rule: `handlers/write.go` and `handlers/reindex.go` call `resolver.FlushWorkspace(wh)` immediately before `extractor.Extract(...)`. Cost: the cache refills lazily on the next resolve query — negligible.

**Write-path latency (Oracle Q5 / AC10)**: the extractor adds ~10–15 ms to every write of a `memory` or `session-summary:*` document (parse + resolve + delete-existing + upsert-new, all inside one transaction). This is the only "regression" in AC10's strict sense. Other writes — source-code documents, embeddings, symbols — are NOT touched. The latency is within the existing write-path budget. Documented here so any future perf regression test has a baseline.

**Ambiguous titles**: if `ResolveTitle` returns >1 doc, the extractor inserts an edge to the most recent (highest `updated_at`) and stamps `metadata.ambiguous = true` with the candidate list — so the UI can warn.

**Wikilink syntax (parser rules)**:
- `[[content]]` where `content` matches `[^\[\]\n]{1,200}` (no nesting, single line, max 200 chars).
- If `content` matches the UUID pattern → treated as ID lookup.
- Otherwise → title lookup (case-insensitive against `documents.title`).
- Escape: `\[[...]\]` renders as literal `[[...]]`. The parser ignores escaped sequences.

### Graph panel — two modes (Code vs Knowledge)

The Graph panel toolbar adds a single mode toggle:

```
┌─────────────────────────────────────────────────────────────┐
│ [Code] [Knowledge]     focus: __________   depth: 1 2 3 …  │
└─────────────────────────────────────────────────────────────┘
```

**Code mode** (default):
- Calls `POST /api/v1/graph/neighborhood` with `node_kind=symbol`.
- Edge types: `contains`, `imports`, `calls`.
- Node color: edge-type-of-primary-outgoing-edge.

**Knowledge mode**:
- Calls `POST /api/v1/graph/neighborhood` with `node_kind=doc`.
- Edge type: `references` (extracted from wikilinks).
- Nodes are documents; `source_node` and `target_node` of edges are document UUIDs.
- Node color encodes collection: `memory` (accent blue), `session-summary:opencode` (status green), `session-summary:claudecode` (status amber), `symbol:*` (text-3 gray; shows up if a memory references a symbol by ID — uncommon but legal).
- Hover shows title + collection + updated_at; double-click opens the document drawer.

**Implementation note**: the same `Sigma.js + Graphology` instance handles both modes. Only the data fetch changes. Node-styling is a one-line switch on `node.kind`. Positions persist separately per mode (different localStorage keys) because the data is fundamentally different.

### SSE event bus (`internal/eventbus/bus.go`)

**Package placement (REVISED per Oracle):** Bus lives in a standalone `internal/eventbus/` package with **zero internal dependencies**. This avoids the import cycle that would form if it lived in `internal/server/webui/` (since `server` already imports `embed`, `watcher`, `harvest` — producers can't import back into `server/webui`).

Producers depend on the `eventbus.Publisher` interface (Publish-only); `server.go` wires the concrete `*Bus` at startup via constructor injection — same pattern used today for `embed.Queue` + `QueueQuerier`.

```go
// internal/eventbus/bus.go
package eventbus

type Event struct {
    Type      string          // "embed_queue", "reindex", "harvest", "watcher", "hello", "lag"
    Workspace string          // empty = global
    Payload   json.RawMessage
    TS        time.Time
}

type Publisher interface {
    Publish(Event)
}

type Bus struct {
    ctx       context.Context
    cancel    context.CancelFunc
    incoming  chan Event              // single producer-side channel
    subs      map[*subscriber]struct{}
    subsMu    sync.RWMutex
}

type subscriber struct {
    ch       chan Event
    workspace string  // optional filter; "" = all
    dropped   atomic.Uint64
}

func New(ctx context.Context) *Bus
func (b *Bus) Publish(e Event)                                  // non-blocking; if Bus is overloaded, drop + log warn
func (b *Bus) Subscribe(workspace string) (<-chan Event, func()) // unsubscribe via returned func
func (b *Bus) Close()                                            // drain + close all subs
```

**Pattern (REVISED per Oracle): fan-out goroutine, drop-newest, no per-publisher mutex**

A single background goroutine `runFanout()` reads from `b.incoming`, walks the subs map (under RLock), and does a **non-blocking send** to each subscriber's bounded channel (size 64). If a subscriber's buffer is full, the new event is dropped and the subscriber's `dropped` counter increments. A periodic ticker (every 5s) emits a synthetic `lag` event into subscribers with `dropped > 0`, then resets the counter.

This eliminates the drop-oldest race (Go channels can't atomically drain-and-send), keeps publish path O(1) lock-free at the producer site, and serializes contention into the fan-out goroutine.

Producers (embed queue, watcher, reindex handler, harvest runner) hold `eventbus.Publisher` references obtained at construction. SSE handler subscribes per HTTP request with the current workspace filter, writes `data: <json>\n\n` + `flusher.Flush()`, sends a `:` heartbeat comment every 30s to keep proxies happy, and exits on `c.Request().Context().Done()`.

**Backpressure semantics:** drop-newest (not drop-oldest). Client receives `lag` event with `dropped: N` and re-queries authoritative state via REST.

**Graceful shutdown:** `Bus.Close()` cancels ctx, drains `incoming`, closes every subscriber channel. SSE handlers' `<-ch` unblock and goroutines exit cleanly.

## Security headers + content sanitization (REVISED per Oracle)

The `/ui` route group applies a security headers middleware (separate from the `/api/v1/*` middleware stack to avoid affecting CLI/MCP consumers):

```
Content-Security-Policy: default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Referrer-Policy: same-origin
```

`'unsafe-inline'` for styles only (Vite emits a tiny inline `<style>` for critical CSS). No `'unsafe-inline'` for scripts — Vite produces hashed external `.js` files.

All memory note content rendered in the UI goes through a `<SafeMarkdown content={...} />` component that pipes `react-markdown` → `rehype-sanitize` with a strict allow-list (paragraphs, links with `rel="noopener noreferrer"`, inline + block code, lists, headings, blockquotes, emphasis, strong). No raw HTML passthrough. Metadata JSONB is rendered as `<pre><code>` (text-only).

## "UI not built" fallback (REVISED per Oracle)

`//go:embed dist/*` with the `dist/` directory containing only `.gitkeep` compiles, but ships an empty UI for users running `go install github.com/.../nano-brain@latest` who have not run `make web`. To avoid a confusing blank `/ui`:

The webui handler checks `embedFS` for `dist/index.html` at startup. If absent, the `/ui` route serves a small hard-coded HTML page:

```
<!DOCTYPE html><html><body style="font-family:system-ui;padding:2em;max-width:600px">
<h1>nano-brain Web UI not built</h1>
<p>This binary was built without the bundled web UI.</p>
<p>Either:</p>
<ul>
  <li>Install the prebuilt npm package: <code>npx @nano-step/nano-brain@latest</code> (includes UI)</li>
  <li>Or build from source: <code>make web && go build ./cmd/nano-brain</code></li>
</ul>
<p>The REST API at <code>/api/v1/*</code> works regardless.</p>
</body></html>
```

The npm tarball ships `dist/` populated via the `prepublishOnly` step in `web/package.json`.

## Frontend ↔ Backend Contracts

All types live twice: Go (sqlc + handler structs) and TS (mirrored in `web/src/api/types.ts`). Sync is **manual** for v1 — too small to justify a code-gen pipeline. A `make verify-api-types` script in v2 will diff handler JSON tags vs TS interfaces.

## Workspace Selection

Global `?workspace=<hash>` in URL. UI persists last-used workspace in `localStorage`. Default = the first workspace returned by `GET /api/v1/workspaces`. Most endpoints already accept workspace via header (`X-Workspace-Hash`) or body; UI uses header consistently.

## Graph Performance Strategy

| Scale | Strategy |
|---|---|
| ≤ 500 nodes | Render all; force-atlas2 layout client-side via Graphology worker |
| 500–5k nodes | Same renderer; precompute layout on first load, cache positions in localStorage keyed by `(workspace, focus_node, depth)` |
| > 5k nodes | Hard cap on `/graph/neighborhood` response (500 nodes). UI shows "expand" buttons on hull nodes. No "render the whole graph" button in v1 — that's an anti-pattern at scale (Obsidian lesson) |

## Trade-offs

| Decision | Pro | Con | Mitigation |
|---|---|---|---|
| Embed SPA in binary | Single distribution unit; works offline; no node runtime at user | Binary grows; need rebuild for UI changes | Lazy-load panels; ≤ 8 MB budget |
| No auth in v1 | Ship fast; matches current localhost-only posture | If user binds 0.0.0.0, anyone on LAN can mutate | Bind check at startup: if non-loopback and no auth configured, refuse to mount /ui unless `--unsafe-no-auth` flag set |
| SSE over WebSocket | Trivial in Go; auto-reconnect built into EventSource; no upgrade dance | Server → client only (fine; UI doesn't push state to server outside of REST POSTs) | — |
| Vite over Next.js | Fast, simple, no server runtime | No SSR (we don't need it) | — |
| CSS Modules over Tailwind | No build-time class bloat; design tokens explicit; no copy-paste-Tailwind look | More CSS to write | — |
| Sigma.js over react-flow | WebGL scales to 5k+ nodes; matches our cardinality | React-flow nodes are simpler to customize (HTML in nodes) | Use react-flow if we ever need rich inline node editing; not v1 |

## Forward-compat hooks (not built v1, but designed in)

1. **Auth**: a `/api/v1/auth/*` namespace is reserved. v2 adds bearer-token middleware mounted before the `/ui` static handler and `/api/v1/*` group.
2. **Embedding viz**: `GET /api/v1/stats/embedding-projection?method=umap&dims=2` deferred to v2. UI has a panel slot.
3. **Multi-user**: workspace_hash is already the per-project scope; future user table can map user → allowed workspace_hashes. No schema change needed for v1.
4. **Editing source files**: not in scope. The "edit" affordance in Memory panel writes a new document that supersedes; we never mutate `documents.content` in place.

## Risk Register

| Risk | Severity | Mitigation |
|---|---|---|
| Binary size blowup | Med | Hard budget 8 MB; lazy-load panels; tree-shake aggressively; benchmark CI |
| SSE consumes goroutines on flapping clients | Med | Bound subscribers per IP (max 8); idle timeout 5 min; lag→disconnect policy |
| Browser-origin CSRF on mutating endpoints | High | Require `X-Requested-With: nano-brain-ui` header on POST/DELETE; reject if Origin is non-loopback when bound to loopback |
| Graph viz hairball at large workspaces | Med | Hard 500-node cap in /graph/neighborhood; force users to start from a focus node |
| TS/Go type drift | Low | Manual sync in v1; verify-api-types script in v2 |
| Vite/npm supply chain | Med | Lockfile committed; CI runs `npm audit`; freeze versions; no postinstall scripts (`npm ci --ignore-scripts` in CI) |
| User runs Firefox without EventSource? | Vanishing | EventSource is universal; if absent, polling fallback in `useEvents.ts` |
| **Migration rollback fails** because `references` rows exist (no Down path) | Med | Document operator procedure: `DELETE FROM graph_edges WHERE edge_type='references' AND workspace_hash=?` before rollback. Surface this in migration comment + CHANGELOG. |
| **Wikilink injection** — malicious content writes `[[victim-id]]` to fake a backlink | Low | Backlinks are metadata, not authority. Anyone with write access to the workspace can already do anything. The link extractor only resolves IDs and titles within the workspace, never cross-workspace. |
| **XSS via wikilink title** — malicious title text rendered in link | Med | `<SafeMarkdown>` sanitizes ALL content. The wikilink renderer outputs `<a>` with `rel="noopener noreferrer"` and an internal-route href; the text content (title) is sanitized by rehype-sanitize. |
| **Title resolution thrash** at large workspaces (1M docs) | Med | Cache the title→id index in memory, invalidated on write/update/delete. Index size for 1M titles ≈ 50 MB — acceptable. Spec note: future v2 may move to a tsvector trigram index for fuzzy match. |
| **Stale backlinks** after a doc is deleted | Med | When a doc is deleted, the extractor deletes the OUTBOUND `references` rows where `source_node = doc.id`. Inbound references where `target_node = doc.id` are kept but rendered as "→ &lt;deleted doc-id&gt;" in the source doc's drawer — purposeful, so users can see history. |
| **Migration applied on old client** that doesn't know about `references` | Vanishing | Old clients only ever insert/select the original three edge types and ignore unknown rows. No code change required in the read path. |
