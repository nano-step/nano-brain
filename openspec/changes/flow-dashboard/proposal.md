## Why

Phase 1 (execution-flow-visualization) successfully implemented flow extraction and on-demand Mermaid rendering via `memory_flow` MCP tool and `POST /api/v1/graph/flow`. However, viewing flows requires either:
- MCP tool call (agent-only, no human visualization)
- curl command (developer-only, no visual output)
- Copy-paste Mermaid to external renderer (manual, tedious)

A **web dashboard** integrated into the existing `/ui` SPA would let developers:
- Browse all detected HTTP endpoints in one place
- View Mermaid flowcharts directly in the browser
- Select workspaces via dropdown (no hash copying)
- Quickly understand system architecture without reading code

This is **Phase 2 (UI layer)** of the execution-flow-visualization roadmap.

## What Changes

- **New UI route**: `GET /ui/flows` serves embedded HTML page (integrated into existing SPA)
- **Pagination**: Add cursor-based pagination to `GET /api/v1/documents` (limit=50, cursor=last_id)
- **Embedded Mermaid.js**: HTML page loads Mermaid from CDN with SRI integrity hash
- **Workspace selector**: Dropdown populated from `GET /api/v1/workspaces`
- **Client-side rendering**: Fetches flow list from existing `GET /api/v1/documents?collection=flows`

## Capabilities

### New Capabilities
- `flow-ui-dashboard`: Web UI for browsing and viewing execution flows (integrated into /ui SPA)
- `documents-pagination`: Cursor-based pagination for documents list endpoint

### Modified Capabilities
- `documents-list`: Add `cursor` and `limit` params to `GET /api/v1/documents`

## Impact

- **Code affected**:
  - `internal/server/webui/flow_dashboard.html` — new embedded HTML template
  - `internal/server/webui/embed.go` — add `//go:embed flow_dashboard.html`
  - `internal/server/webui/handler.go` — add `GET /ui/flows` handler
  - `internal/server/routes.go` — register new route under `/ui` group
  - `internal/server/handlers/documents.go` — add cursor/limit params
  - `internal/storage/queries/documents.sql` — add pagination to ListDocumentsByWorkspace

- **Dependencies**: Mermaid.js via CDN (no new Go dependencies)

- **Performance**: Negligible (static HTML + client-side rendering)

- **API changes**: One new route (`GET /ui/flows`), one modified endpoint (pagination)

- **Risk**: Low (read-only UI, no data mutation)
