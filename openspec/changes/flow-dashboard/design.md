# Design: Flow Dashboard (Phase 2 - UI Layer)

## Context

Phase 1 delivered flow extraction and on-demand Mermaid rendering via API/MCP. Phase 2 adds a **web dashboard** for human visualization. The dashboard integrates into the existing `/ui` SPA — no separate frontend build, no React/Vue, no npm.

## Goals / Non-Goals

**Goals**
- Serve a browsable dashboard at `GET /ui/flows`
- List all detected flows with metadata (endpoint, handler, chain length)
- Render Mermaid diagrams inline in the browser
- Support workspace selection via dropdown
- Show chain stats below diagram

**Non-Goals (deferred)**
- Search/filter on flows (Phase 3)
- Responsive/mobile layout (Phase 3)
- Authentication/authorization (Phase 3)
- Flow comparison/diff (Phase 3)
- Interactive editing (Phase 3)
- Real-time updates (polling sufficient for now)
- Dark mode (can add later)
- Embedded Mermaid.js for offline (Phase 3)

## Architecture

```
Browser → GET /ui/flows → HTML page (embedded in /ui SPA)
         → GET /api/v1/workspaces → workspace list (for selector dropdown)
         → GET /api/v1/documents?collection=flows&workspace=<hash> → flow list from materialized docs
         → POST /api/v1/graph/flow {workspace, entry} → Mermaid diagram (on-demand)
         → Client-side Mermaid.js rendering
```

### Components

1. **HTML Template** (`internal/server/webui/flow_dashboard.html`)
   - Embedded via `//go:embed` in `webui/embed.go`
   - Single-file HTML + CSS + JS (no build step)
   - Mermaid.js loaded from CDN (`https://cdn.jsdelivr.net/npm/mermaid/dist/mermaid.min.js`)

2. **List Endpoint** (`GET /api/v1/documents?collection=flows`)
   - Existing endpoint — no new SQL needed
   - Add cursor-based pagination (limit=50, cursor=last_id)
   - Returns JSON array of flow documents with metadata

3. **UI Endpoint** (`GET /ui/flows`)
   - Serves the embedded HTML page
   - No workspace middleware (page loads first, JS fetches data)
   - Served from `/ui` group (inherits SecurityHeaders middleware)

### Data Flow

```
1. Page loads → fetch /api/v1/workspaces → populate workspace selector dropdown
2. User selects workspace → fetch /api/v1/documents?collection=flows&workspace=<hash> → populate sidebar
3. User clicks endpoint → POST /api/v1/graph/flow {workspace, entry} → render Mermaid
4. CDN failure → show fallback message (not blank page)
```

### UI Layout

```
┌─────────────────────────────────────────────────────┐
│ nano-brain Flow Dashboard                    [Search]│
├──────────────────┬──────────────────────────────────┤
│ Endpoints        │ Flow Diagram                     │
│                  │                                   │
│ POST /api/v1/... │ graph TD                          │
│ POST /api/v1/... │   Entry[entry] --> Handler        │
│ GET /api/v1/...  │   Handler --> Service             │
│ POST /api/v1/... │   Service --> Repo                │
│ ...              │                                   │
│                  │                                   │
├──────────────────┴──────────────────────────────────┤
│ Chain: entry → handler → service → repo (4 nodes)   │
│ Externals: 15 | Middleware: 2                       │
└─────────────────────────────────────────────────────┘
```

## Technical Decisions

### List API Data Source
- **Use existing `GET /api/v1/documents?collection=flows`** — not `POST /api/v1/query`
- Why: `POST /api/v1/query` requires non-empty query and does relevance ranking — wrong for deterministic listing
- The documents endpoint already supports collection filtering
- Add cursor-based pagination to `ListDocumentsByWorkspace` SQL

### HTML Page Placement
- **Serve from `/ui` group** — not `data` group
- Why: `/ui` group has SecurityHeaders middleware (CSP), no workspace middleware
- Workspace is fetched client-side via `GET /api/v1/workspaces`
- Data fetches use `data` group endpoints (which have workspace middleware)

### Mermaid.js Loading
- **CDN** (`https://cdn.jsdelivr.net/npm/mermaid/dist/mermaid.min.js`)
- Pros: No build step, always latest version
- Cons: Requires internet, CDN outage risk
- **Mitigation**: SRI integrity hash on `<script>` tag
- **Fallback**: If CDN fails, show "Mermaid renderer not available" message

### CSP (Content Security Policy)
- **Current CSP** on `/ui` group: `default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'`
- **Problem**: `script-src` absent → falls back to `default-src 'self'` → **blocks CDN-loaded Mermaid.js**
- **Fix**: Add `script-src 'self' https://cdn.jsdelivr.net` to CSP in `internal/server/middleware/security_headers.go`
- **Alternative**: Use nonce-based approach with `script-src 'self' 'nonce-{random}'`
- **Note**: This is a prerequisite — Mermaid.js won't load without updating CSP

### HTML Embedding
- Use `//go:embed flow_dashboard.html` in `webui/embed.go`
- Single-file HTML (no external CSS/JS except Mermaid CDN)
- Keeps deployment simple: single binary serves everything

### Client-Side Rendering
- No server-side Mermaid rendering (keeps server stateless)
- Mermaid.js runs in browser (~500KB gzipped)
- First render ~200ms on modern hardware

### Workspace Selection
- Dropdown selector populated from `GET /api/v1/workspaces`
- User sees workspace names, not hashes
- Hash is passed to data fetch calls via JS
- For `POST /api/v1/graph/flow`: pass hash in JSON body `{"workspace": "<hash>", "entry": "..."}`
- Workspace middleware reads from JSON body for POST requests

## Open Questions

None — all decisions locked by user.
