# web-ui-app Specification

## Purpose
TBD - created by archiving change web-ui. Update Purpose after archive.
## Requirements
### Requirement: Six primary panels
The SPA SHALL expose six panels reachable via the persistent sidebar: **Dashboard**, **Memory**, **Graph**, **Symbols**, **Harvest**, **Settings**. Each panel has a dedicated route under `/ui/<panel>` and a mnemonic keyboard shortcut surfaced in the command palette.

#### Scenario: Sidebar navigation
- **WHEN** the user clicks the "Memory" sidebar entry
- **THEN** the URL becomes `/ui/memory?workspace=<hash>`
- **AND** the Memory panel renders its first paint within 100 ms (data may stream in subsequently)

#### Scenario: Mnemonic shortcuts
- **WHEN** the user presses `g m` (sequence)
- **THEN** the router navigates to the Memory panel
- **AND** similar sequences exist: `g d` (dashboard), `g g` (graph), `g s` (symbols), `g h` (harvest), `g ,` (settings)

### Requirement: Workspace selector
A workspace selector SHALL appear in the top-left of every panel. Its value is reflected in the URL (`?workspace=<hash>`) and persisted to `localStorage`.

#### Scenario: Switching workspace
- **WHEN** the user opens the workspace selector and chooses a different workspace
- **THEN** the URL updates with the new hash
- **AND** all panel queries are re-fetched scoped to the new workspace
- **AND** the new value is persisted to `localStorage` under key `nano-brain.workspace`

### Requirement: Command palette (Cmd+K / Ctrl+K)
The SPA SHALL provide a command palette opened by `Cmd+K` (macOS) or `Ctrl+K` (Windows/Linux) that fuzzy-searches across: navigation routes, actions (trigger reindex/harvest/reload-config), recent searches (last 20, localStorage), workspaces, and symbol names.

#### Scenario: Palette opens and closes
- **WHEN** the user presses `Cmd+K` anywhere in the app
- **THEN** a modal palette opens with the search input focused
- **AND** pressing `Esc` closes it returning focus to the previously focused element

#### Scenario: Searching symbols
- **WHEN** the user types `proc` in the palette
- **THEN** within 150 ms, results include symbols whose name contains `proc` (subject to the current workspace), grouped under a "Symbols" section
- **AND** pressing `Enter` on a symbol result navigates to the Graph panel focused on that symbol

#### Scenario: Running an action from the palette
- **WHEN** the user types `reindex` and selects "Trigger reindex"
- **THEN** the UI calls `POST /api/v1/reindex` with the current workspace
- **AND** displays a toast confirming the request was accepted

### Requirement: Dashboard panel
The Dashboard panel SHALL display: server version, uptime, embedding provider + model, doc count, chunk counts grouped by `embed_status`, embeddings total, graph_edges count by edge_type, harvest mode + last harvest timestamp, embed queue depth (live via SSE), and a list of 10 most recently updated documents.

#### Scenario: Live embed queue counter
- **WHEN** the embed queue depth changes server-side
- **THEN** within 1 second the Dashboard panel reflects the new count (via SSE)
- **AND** no manual refresh is required

#### Scenario: Initial load completes under 500 ms on warm cache
- **WHEN** the user navigates to `/ui/dashboard` after the SPA bundle is cached
- **THEN** first contentful paint occurs within 500 ms
- **AND** the stats data is fetched from `/api/v1/stats` and rendered within the same paint frame for the values already returned (skeleton placeholders for any in-flight data)

### Requirement: Memory panel
The Memory panel SHALL list documents for the selected workspace with columns: title, tags, collection, updated_at, supersedes indicator. The list SHALL support: BM25 text filter, multi-select tag filter, collection filter, sort by updated_at/created_at/title.

#### Scenario: Tag filter
- **WHEN** the user selects tags `decision` and `auth`
- **THEN** the list SHALL show only documents whose `tags` array contains BOTH (AND logic)
- **AND** the URL updates with `?tags=decision,auth` so the filter is shareable

#### Scenario: Open document detail
- **WHEN** the user clicks a row
- **THEN** a side drawer opens showing full content, metadata JSONB (pretty-printed), supersedes chain (rendered as a vertical timeline), and action buttons: "Edit (creates new doc that supersedes this one)", "Delete", "Copy ID", "Add tag", "Remove tag"
- **AND** the URL becomes `/ui/memory/<doc-id>?workspace=<hash>` so the detail view is shareable

#### Scenario: Edit creates supersedes
- **WHEN** the user edits a document's content and saves
- **THEN** the UI calls `POST /api/v1/write` with the new content and `supersedes_id` set to the original document's ID
- **AND** the original document remains unchanged
- **AND** the chain in the drawer updates to show the new version at the top

### Requirement: Graph panel (dual-mode)
The Graph panel SHALL render an interactive graph using Sigma.js + Graphology with a mode toggle in its toolbar offering exactly two modes:

- **Code mode** (default) — symbols + `contains` / `imports` / `calls` edges. Node colored by primary outgoing edge type.
- **Knowledge mode** — documents (memory notes + session summaries + symbol docs) + `references` edges extracted from wikilinks. Node colored by document collection.

Controls (shared by both modes): focus search input, depth slider (1–5), direction toggle (in/out/both), edge-type multi-select. Hover shows source location (Code mode: `file:line`; Knowledge mode: `title · collection · updated_at`). Click expands the neighborhood by 1 in the current direction.

#### Scenario: Render bounded neighborhood (Code mode)
- **WHEN** the user enters focus `processQuery`, depth 2, direction out, edge_types `calls`, mode=Code
- **THEN** the UI calls `POST /api/v1/graph/neighborhood` with `node_kind=symbol`
- **AND** renders returned nodes/edges
- **AND** the layout settles within 2 seconds for ≤ 500 nodes via Web Worker force-atlas2
- **AND** if `truncated: true` is in the response, hull nodes display a `+` affordance to expand

#### Scenario: Render Knowledge-mode neighborhood
- **WHEN** the user toggles mode to Knowledge and enters focus `<doc-uuid>`, depth 2, direction both, edge_types `references`
- **THEN** the UI calls `POST /api/v1/graph/neighborhood` with `node_kind=doc`
- **AND** renders documents as nodes (color by collection)
- **AND** the 500-node cap applies identically; hull nodes show `+` to expand

#### Scenario: Mode switch resets focus
- **WHEN** the user is in Code mode focused on `processQuery` and switches to Knowledge mode
- **THEN** the focus input is cleared (since symbol names rarely resolve to doc UUIDs)
- **AND** a helper hint appears: "Enter a memory document title or ID, or Cmd+K to find one"
- **AND** the prior Code-mode focus is remembered so toggling back to Code restores it without re-typing

#### Scenario: Position persistence is per-mode
- **WHEN** the user drags nodes in Knowledge mode
- **AND** later switches to Code mode and back to Knowledge with the same focus
- **THEN** Knowledge-mode positions SHALL be restored from `localStorage` keyed by `(workspace, focus, depth, direction, edge_types, mode)`
- **AND** Code-mode positions for the same focus/params SHALL be stored under a separate key (no cross-mode bleed)

#### Scenario: Click-to-symbol cross-link (Code mode)
- **WHEN** in Code mode the user double-clicks a node
- **THEN** the Symbols panel opens with that symbol pre-filtered

#### Scenario: Double-click in Knowledge mode opens drawer
- **WHEN** in Knowledge mode the user double-clicks a doc node
- **THEN** the document drawer opens in place (same drawer used by the Memory panel)
- **AND** the Graph panel remains in the background; closing the drawer returns focus to the graph

### Requirement: Inline wikilinks in rendered content
All Markdown content rendered in the SPA (memory notes, session summaries, any document's `content` field) SHALL be processed through a wikilink rewriter that converts `[[<target>]]` syntax into clickable links.

#### Scenario: ID wikilink resolves to inline link
- **WHEN** the content contains `[[d-1042]]` where `d-1042` is a valid doc ID in the current workspace
- **THEN** the rendered output replaces it with an anchor element styled as an internal link
- **AND** clicking the anchor opens the target document's drawer in place (no full navigation)
- **AND** the anchor's accessible name is the target document's title (or the literal ID if title is empty)

#### Scenario: Title wikilink resolves case-insensitively
- **WHEN** the content contains `[[Decision: use eventbus pkg]]` and exactly one document in the workspace has that title (any case)
- **THEN** the rendered output is a single anchor pointing to that document

#### Scenario: Ambiguous title wikilink renders with warning
- **WHEN** the content contains `[[Decision]]` and 3 documents in the workspace have that exact title
- **THEN** the rendered output is plain text (NOT a link) with a dotted underline
- **AND** hovering shows a tooltip listing the 3 candidate document IDs
- **AND** the drawer's header shows a small warning chip "1 ambiguous wikilink" so the author can fix the source content

#### Scenario: Unresolved wikilink renders as broken
- **WHEN** the content contains `[[non-existent]]` matching no document in the workspace
- **THEN** the rendered output is plain text with a dotted underline (broken-link style)
- **AND** hovering shows a tooltip "No document with that ID or title in this workspace"

#### Scenario: Escaped wikilink renders literal
- **WHEN** the content contains `\[[not a link]\]`
- **THEN** the rendered output is the literal string `[[not a link]]` with no link affordance

#### Scenario: Wikilinks honor XSS sanitization
- **WHEN** a malicious document contains `[[<script>alert(1)</script>]]`
- **THEN** the content is passed through `rehype-sanitize` BEFORE the wikilink rewriter sees it
- **AND** no script tag survives to the DOM

### Requirement: Backlinks panel in document drawer
Every open document drawer SHALL display a "Referenced by" section listing other documents in the same workspace whose content contains a wikilink resolving to the open document.

#### Scenario: Drawer fetches backlinks on open
- **WHEN** the user opens a document drawer for `d-1042`
- **THEN** the drawer issues `GET /api/v1/links/d-1042/backlinks?workspace=<hash>`
- **AND** renders the list in a "Referenced by" section, even if empty
- **AND** for empty results, the section displays a meaningful placeholder ("No other documents reference this yet")

#### Scenario: Each backlink shows title + collection + age + snippet
- **WHEN** the backlinks list has results
- **THEN** each row shows: title (clickable), collection (mono small), updated_at age (e.g., "3h ago"), a 200-char snippet around the wikilink occurrence
- **AND** the row layout matches the Memory panel list rows for visual consistency
- **AND** clicking a row opens that referencing document's drawer in place, replacing the current drawer's content

#### Scenario: Backlinks update after content change
- **WHEN** the user (or the agent) writes a new document referencing `d-1042` via `[[d-1042]]`
- **AND** the Memory drawer for `d-1042` is open
- **THEN** within 2 seconds (next backlinks refetch on event or stale-while-revalidate tick) the new referencing doc appears in the "Referenced by" list

### Requirement: Symbols panel
The Symbols panel SHALL provide type-ahead search across symbol documents (collection prefix `symbol:`) with filters by kind (function/method/type/interface/struct/const/var) and language (go/python/typescript/javascript). Results show name, kind, language, source path, line, and impact count (incoming edges).

#### Scenario: Search with kind filter
- **WHEN** the user types `Handler` and filters kind=`type`
- **THEN** only documents matching both criteria appear
- **AND** results are sorted by impact count (most-referenced first) by default

#### Scenario: Jump to graph
- **WHEN** the user clicks "Show in graph" on a symbol result
- **THEN** the Graph panel opens with that symbol as the focus node

### Requirement: Harvest panel
The Harvest panel SHALL list harvested sessions (collection prefix `session-summary:`) with: source (opencode/claudecode), agent, duration, project path, created_at, summary preview (first 200 chars). User can: trigger harvest (`POST /api/harvest`), view full summary, open the raw session via the supersedes chain.

#### Scenario: Trigger harvest with live progress
- **WHEN** the user clicks "Trigger harvest"
- **THEN** the UI POSTs to `/api/harvest`
- **AND** subscribes to `/api/v1/events` for `harvest` events
- **AND** displays a progress bar / counter that updates as `sessions_seen` and `sessions_summarized` increment
- **AND** shows a final toast on completion

### Requirement: Settings panel
The Settings panel SHALL read `/api/v1/config` and render a form for safe-patch fields (search.rrf_k, search.recency_weight, search.recency_half_life_days, search.limit, watcher.debounce_ms, watcher.reindex_interval, summarization.enabled, summarization.model, summarization.max_tokens, summarization.concurrency, summarization.requests_per_second, logging.level). Secrets (api keys, db url) are shown as read-only with literal "<redacted>".

#### Scenario: Apply config change
- **WHEN** the user changes `search.rrf_k` from 60 to 80 and clicks Save
- **THEN** the UI POSTs `{"search":{"rrf_k":80}}` to `/api/v1/config`
- **AND** on 200, displays a toast "Config saved + reloaded"
- **AND** the form re-renders with the new resolved value

#### Scenario: Invalid input is caught client-side
- **WHEN** the user enters `port: -1` (if exposed) or any field failing zod schema
- **THEN** the Save button is disabled
- **AND** the field shows the validation error inline

#### Scenario: Doctor checks panel
- **WHEN** the Settings panel mounts
- **THEN** it issues `GET /api/v1/doctor` and renders the check results as a list with status badges (✅/⚠️/❌)
- **AND** failed checks expand to show the `hint` text

### Requirement: Real-time updates via SSE
The SPA SHALL maintain a single `EventSource` connection per session subscribed to `/api/v1/events?workspace=<current>`, multiplex events into a Zustand (or equivalent) store, and re-render dependent components.

#### Scenario: Auto-reconnect on disconnect
- **WHEN** the SSE connection drops (server restart, network blip)
- **THEN** the browser's native `EventSource` retry SHALL re-establish the connection within 3 seconds
- **AND** UI components dependent on live data show a "reconnecting…" indicator during the gap

#### Scenario: Workspace switch re-subscribes
- **WHEN** the user changes workspace
- **THEN** the existing EventSource is closed
- **AND** a new EventSource is opened with the new workspace query parameter

### Requirement: Offline + no third-party CDN
The SPA SHALL load and operate without any HTTP request to a domain other than the origin serving `/ui`. All fonts, icons, scripts, and styles SHALL be bundled.

#### Scenario: Network audit
- **WHEN** the SPA is loaded in a browser with all third-party requests blocked
- **THEN** the UI renders fully (no missing fonts, icons, scripts, or fetches to `googleapis.com`, `cloudflare.com`, `jsdelivr.net`, etc.)

### Requirement: Destructive operations require double-confirm
All UI actions that delete or reset data SHALL require a two-step confirmation: the user clicks the action, a modal appears asking them to type the exact name/hash of the target, and only then is the request sent.

Scope of destructive actions: delete document, reset workspace (`POST /api/v1/reset-workspace`), remove workspace (`DELETE /api/v1/workspaces/:hash`), reset embeddings (`POST /api/v1/reset-embeddings`), remove collection.

#### Scenario: Delete document requires typed confirmation
- **WHEN** the user clicks "Delete" on a memory note
- **THEN** a modal opens displaying the document title and ID
- **AND** the modal contains a text input that must match the document ID exactly before the "Confirm delete" button enables
- **AND** typing a non-matching string keeps the button disabled
- **AND** pressing Esc cancels without deleting

#### Scenario: Reset workspace requires workspace name confirmation
- **WHEN** the user clicks "Reset workspace" in Settings
- **THEN** a modal opens warning that ALL documents, chunks, embeddings, and graph edges for the workspace will be deleted (workspace registration preserved)
- **AND** the user must type the workspace name exactly to enable the destructive action

#### Scenario: Remove workspace requires double confirmation
- **WHEN** the user clicks "Remove workspace"
- **THEN** the modal displays the workspace hash + path, and requires typing the workspace hash (not just the name) to enable removal
- **AND** displays a final "This cannot be undone" line

### Requirement: Non-loopback bind warning banner
When the server is bound to a non-loopback address (started with `--unsafe-no-auth`), the SPA SHALL display a persistent red warning banner at the top of every panel.

#### Scenario: Banner appears for non-loopback bind
- **WHEN** the SPA loads and `GET /api/v1/config` returns `server.host` ≠ `localhost`/`127.0.0.1`/`::1`
- **THEN** a red banner appears at the top reading "⚠ Server bound to <host>:<port> without authentication. Anyone on the network can read and modify your memory. Bind to loopback or configure auth."
- **AND** the banner remains visible across all panel navigation
- **AND** the banner is NOT dismissible

#### Scenario: Banner absent for loopback bind
- **WHEN** the server is bound to `localhost`, `127.0.0.1`, or `::1`
- **THEN** no banner appears

### Requirement: Accessibility floor
The SPA SHALL meet a baseline accessibility bar: keyboard navigation for all primary flows, visible focus rings on interactive elements, WCAG AA color contrast on text, focus traps on modal dialogs.

#### Scenario: Keyboard navigation through primary panels
- **WHEN** a user with no mouse navigates Dashboard → Memory list → Memory detail → close drawer → Symbols search → Settings → Save
- **THEN** every step is reachable via `Tab`, `Shift+Tab`, `Enter`, `Esc`, and arrow keys
- **AND** focus indicator is visible at all times

