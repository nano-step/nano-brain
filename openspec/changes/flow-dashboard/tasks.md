# Tasks: Flow Dashboard (Phase 2 - UI Layer)

Ordered for incremental, independently-verifiable delivery. Each numbered group should build (`CGO_ENABLED=0 go build ./...`) and pass `go test -race -short ./...` before moving on.

## 1. List API (Pagination for Documents)
- [x] 1.1 Add `ListDocumentsByWorkspacePaginated` SQL query (sqlc): `WHERE workspace_hash=$1 AND (updated_at, id) < ($cursor_ts, $cursor_id) ORDER BY updated_at DESC, id DESC LIMIT $limit`
- [x] 1.2 Add `cursor` and `limit` params to `GET /api/v1/documents` handler (default limit=50)
- [x] 1.3 Add `next_cursor` field to response JSON (null when no more results)
- [x] 1.4 Support `?collection=flows` filter (already exists, verify it works)
- [x] 1.5 Update CSP header in `internal/server/middleware/security_headers.go` to allow Mermaid CDN: `script-src 'self' https://cdn.jsdelivr.net`
- [ ] 1.6 Integration test: list flows returns paginated results with correct structure

## 2. HTML Template (Embedded in /ui SPA)
- [x] 2.1 Create `internal/server/webui/flow_dashboard.html` with embedded CSS/JS
- [x] 2.2 Add `//go:embed flow_dashboard.html` in `webui/embed.go`
- [x] 2.3 Layout: sidebar (endpoint list) + main (Mermaid diagram)
- [x] 2.4 Load Mermaid.js from CDN (`https://cdn.jsdelivr.net/npm/mermaid/dist/mermaid.min.js`)
- [x] 2.5 Initialize Mermaid with `startOnLoad: false`

## 3. UI Route (Serve from /ui Group)
- [x] 3.1 Add `GET /ui/flows` handler in `internal/server/webui/handler.go`
- [x] 3.2 Serve the embedded HTML page (no workspace middleware — page loads first, JS fetches data)
- [x] 3.3 Register route in `internal/server/routes.go` under `/ui` group
- [ ] 3.4 Integration test: UI route returns HTML with correct content-type

## 4. Client-Side JS
- [x] 4.1 On page load: fetch `/api/v1/workspaces` → populate workspace selector dropdown
- [x] 4.2 On workspace select: fetch `/api/v1/documents?collection=flows&workspace=<hash>` → populate sidebar
- [x] 4.3 On endpoint click: POST `/api/v1/graph/flow` → render Mermaid diagram
- [x] 4.4 Display chain stats below diagram (nodes, externals, middleware count)
- [x] 4.5 Handle loading states and errors gracefully (CDN failure, API errors, empty workspace)

## 5. Styling (Minimal v1)
- [x] 5.1 Functional layout (sidebar + main, no responsive breakpoints)
- [x] 5.2 Endpoint list: hover highlight, active state
- [x] 5.3 Mermaid container: scrollable, max-height

## 6. Verification
- [x] 6.1 Full build `CGO_ENABLED=0 go build ./...` and `go test -race -short ./...` green
- [ ] 6.2 Go integration test: `GET /api/v1/documents?collection=flows` returns paginated results
- [ ] 6.3 Go integration test: `GET /ui/flows` returns HTML with correct content-type
- [ ] 6.4 HTML content verification: check sidebar div, Mermaid script tag, SRI hash
- [x] 6.5 CSP validation: verify `script-src` allows Mermaid CDN
- [ ] 6.6 Manual test: open `http://localhost:3100/ui/flows` in browser
- [ ] 6.7 Verify: workspace selector populates, endpoint list loads, click renders diagram
- [ ] 6.8 Verify: CDN failure shows fallback message (not blank page)

## Out of scope (do NOT implement here)
- Search/filter on flows (Phase 3)
- Responsive/mobile layout (Phase 3)
- Authentication/authorization (Phase 3)
- Flow comparison/diff (Phase 3)
- Interactive editing (Phase 3)
- Real-time updates via WebSocket (Phase 3)
- Dark mode (Phase 3)
- Embedded Mermaid.js for offline support (Phase 3)
