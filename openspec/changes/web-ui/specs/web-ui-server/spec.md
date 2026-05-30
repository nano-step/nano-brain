# web-ui-server Delta — Web UI HTTP surface

## ADDED Requirements

### Requirement: Embedded SPA at /ui
The server SHALL serve a single-page application (SPA) at `/ui` and `/ui/*` from assets embedded in the binary via `//go:embed`. The handler SHALL serve `index.html` for any path under `/ui` that does not resolve to a static asset (SPA client-router fallback).

#### Scenario: Root UI path loads index
- **WHEN** a browser issues `GET /ui`
- **THEN** the server returns `200 OK` with `Content-Type: text/html` and the embedded `index.html` body
- **AND** the response includes appropriate `Cache-Control` for assets (immutable for hashed files, no-cache for `index.html`)

#### Scenario: Unknown sub-path falls back to index (SPA routing)
- **WHEN** a browser issues `GET /ui/memory/abc-123`
- **AND** no asset file matches that path
- **THEN** the server returns `200 OK` with `index.html`
- **SO THAT** the client router can resolve `/memory/abc-123`

#### Scenario: Static asset is served with correct content type
- **WHEN** a browser issues `GET /ui/assets/main-<hash>.js`
- **THEN** the server returns the embedded file with `Content-Type: application/javascript`
- **AND** `Cache-Control: public, max-age=31536000, immutable`

#### Scenario: UI is gated when bound to non-loopback without auth
- **WHEN** the server is configured with `server.host=0.0.0.0` (or any non-loopback)
- **AND** no auth configuration is present
- **AND** the startup flag `--unsafe-no-auth` is NOT set
- **THEN** the server SHALL refuse to start with a clear error message naming the bind address and required flag
- **AND** SHALL NOT mount the `/ui` route

#### Scenario: Missing UI build falls back to instructional page
- **WHEN** the binary was built without `web/dist/` populated (e.g., `go install` from source without `make web`)
- **AND** the user navigates to `/ui`
- **THEN** the server returns `200 OK` with a small hard-coded HTML page explaining how to obtain a UI-enabled build
- **AND** the page includes both options: `npx @nano-step/nano-brain@latest` (prebuilt) and `make web && go build` (from source)
- **AND** the REST API at `/api/v1/*` continues to function normally

### Requirement: Security headers on UI routes
The `/ui` route group SHALL apply security headers to defend against XSS, clickjacking, MIME-sniffing, and information leaks.

#### Scenario: Security headers present on UI responses
- **WHEN** a browser issues `GET /ui` or any sub-path
- **THEN** the response includes:
  - `Content-Security-Policy: default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'`
  - `X-Content-Type-Options: nosniff`
  - `X-Frame-Options: DENY`
  - `Referrer-Policy: same-origin`

#### Scenario: Security headers not applied to API endpoints
- **WHEN** a CLI client issues `POST /api/v1/write` (no Origin)
- **THEN** the response SHALL NOT include the UI security headers
- **SO THAT** non-browser API consumers are unaffected by browser-specific policy

### Requirement: Config read endpoint
The server SHALL expose `GET /api/v1/config` returning the current resolved configuration as JSON, with secret fields (api keys, passwords, connection strings containing credentials) replaced with the literal string `"<redacted>"`.

#### Scenario: Config endpoint returns resolved config
- **WHEN** a client issues `GET /api/v1/config`
- **THEN** the response body is a JSON object matching the runtime `config.Config` shape
- **AND** every field listed in `internal/config/secrets.go` (or equivalent) is `"<redacted>"`
- **AND** non-secret fields show actual values (host, port, model names, intervals, etc.)

#### Scenario: Config endpoint reports source of each value
- **WHEN** a client issues `GET /api/v1/config?include_source=true`
- **THEN** the response includes a `_sources` map showing for each top-level key whether the value came from `file`, `env`, or `default`

### Requirement: Config patch endpoint
The server SHALL expose `POST /api/v1/config` accepting a partial JSON patch of safe (non-secret, hot-reloadable) config fields, validating it, persisting it to the config file on disk, and triggering the existing hot-reload pipeline.

#### Scenario: Valid patch applies and reloads
- **WHEN** a client posts `{"search": {"rrf_k": 80}}` to `/api/v1/config`
- **THEN** the server validates the field is in the safe-patch allowlist
- **AND** writes the merged config back to `~/.nano-brain/config.yml` preserving comments and YAML order best-effort
- **AND** invokes the same path as `POST /api/reload-config`
- **AND** returns `200` with the new resolved config (secrets redacted)

#### Scenario: Patch attempts a secret field
- **WHEN** a client posts `{"summarization": {"api_key": "..."}}` to `/api/v1/config`
- **THEN** the server returns `400` with a body explaining `api_key` is not patchable via UI; user must edit the config file directly

#### Scenario: Patch fails validation
- **WHEN** a client posts `{"server": {"port": -1}}` to `/api/v1/config`
- **THEN** the server returns `422` with a structured error listing the offending field and the validation rule it failed
- **AND** the on-disk config is NOT modified

### Requirement: Doctor diagnostics endpoint
The server SHALL expose `GET /api/v1/doctor` returning the same checks as the `nano-brain doctor` CLI command in JSON form.

#### Scenario: Doctor returns structured check results
- **WHEN** a client issues `GET /api/v1/doctor`
- **THEN** the response is a JSON array of `{name, status, detail, hint}` objects, one per check (PG reachable, pgvector ext present, embedding provider reachable, model available, migrations up-to-date, watcher OK)
- **AND** `status ∈ {"ok","warn","fail"}`
- **AND** failed checks include a `hint` string telling the user how to fix it

### Requirement: Stats aggregation endpoint
The server SHALL expose `GET /api/v1/stats?workspace=<hash>` returning workspace-scoped aggregations powering the Dashboard panel.

#### Scenario: Stats returns dashboard aggregates
- **WHEN** a client issues `GET /api/v1/stats?workspace=<hash>`
- **THEN** the response includes counts: `docs_total`, `chunks_total`, `chunks_by_embed_status` (object pending/embedded/embed_failed), `embeddings_total`, `graph_edges_by_type` (object contains/imports/calls), `collections` (array of {name, doc_count}), `tags_top_20` (array of {tag, count}), `recent_docs` (10 most recently updated), `recent_queries` (last 20 from telemetry)
- **AND** the query runs in under 500 ms on a workspace with ≤ 1M chunks (covered by existing indexes)

#### Scenario: Stats handles empty workspace
- **WHEN** the workspace exists but has no documents
- **THEN** the response has all counts at 0 and arrays empty, with HTTP 200

### Requirement: Graph neighborhood endpoint
The server SHALL expose `POST /api/v1/graph/neighborhood` returning a bounded slice of the symbol or knowledge graph around a focus node. The request MAY include a `node_kind` discriminator with values `"symbol"` (default; code graph: contains/imports/calls) or `"doc"` (knowledge graph: references between documents).

#### Scenario: Neighborhood returns nodes and edges (default symbol mode)
- **WHEN** a client posts `{"focus": "processQuery", "depth": 2, "direction": "out", "edge_types": ["calls"], "workspace": "<hash>"}`
- **THEN** the response is `{node_kind: "symbol", nodes: [{id, label, kind, source_file, ...}], edges: [{source, target, edge_type, source_file, ...}], truncated: false}`
- **AND** total node count SHALL NOT exceed 500
- **AND** if the true neighborhood exceeds 500, the response includes `truncated: true` and `frontier_nodes: [...]` so the UI can render expand affordances

#### Scenario: Knowledge-mode neighborhood for memory documents
- **WHEN** a client posts `{"focus": "<doc-uuid>", "depth": 2, "direction": "both", "edge_types": ["references"], "node_kind": "doc", "workspace": "<hash>"}`
- **THEN** the response is `{node_kind: "doc", nodes: [{id: <uuid>, label: <title>, collection: <coll>, updated_at: <iso>, ...}], edges: [{source: <uuid>, target: <uuid>, edge_type: "references", source_file: <doc.source_path or empty>, ...}], truncated: false}`
- **AND** node IDs are document UUIDs (not symbol names)
- **AND** the 500-node hard cap applies in the same way as symbol mode
- **AND** only edges with `edge_type='references'` are returned (server filters even if the client sends other types)

#### Scenario: Invalid node_kind is rejected
- **WHEN** `node_kind` is present and is neither `"symbol"` nor `"doc"`
- **THEN** the server returns `422` with a clear error message

#### Scenario: Invalid depth is rejected
- **WHEN** depth > 5 or depth < 1
- **THEN** the server returns `422`

#### Scenario: Unknown focus node returns empty
- **WHEN** the focus node name does not appear in `graph_edges.source_node` or `graph_edges.target_node` for the workspace
- **THEN** the response is `{node_kind: <as-requested>, nodes: [], edges: [], truncated: false}` with HTTP 200

### Requirement: Backlinks endpoint
The server SHALL expose `GET /api/v1/links/:doc_id/backlinks?workspace=<hash>&limit=<n>` returning documents in the same workspace whose content contains a wikilink resolving to `doc_id`.

#### Scenario: Backlinks lists referencing documents
- **WHEN** a client issues `GET /api/v1/links/d-1042/backlinks?workspace=<hash>`
- **THEN** the response is `{doc_id: "d-1042", backlinks: [{id, title, collection, updated_at, tags, snippet}, ...], total: N}`
- **AND** results are sorted by `updated_at DESC`
- **AND** default `limit` is 50, max 200
- **AND** each `snippet` is up to 200 characters surrounding the first wikilink match (or null if backlink is via supersedes — out of scope for v1)

#### Scenario: Backlinks endpoint handles unknown doc
- **WHEN** `doc_id` does not exist in the workspace
- **THEN** the server returns `404`

#### Scenario: Backlinks endpoint scoped to workspace
- **WHEN** a doc in workspace A references a doc in workspace B (impossible today; defense in depth)
- **THEN** the cross-workspace edge SHALL NOT appear in the backlinks response
- **AND** the extractor SHALL refuse to insert cross-workspace `references` edges

### Requirement: Wikilink resolution endpoint
The server SHALL expose `GET /api/v1/links/resolve?workspace=<hash>&query=<title-or-id>` to let the SPA resolve a wikilink target in one round-trip per panel-render, not per click.

#### Scenario: ID lookup returns single match
- **WHEN** `query` matches the UUID pattern
- **AND** a document with that ID exists in the workspace
- **THEN** the response is `{matched: ["<uuid>"], ambiguous: false, kind: "id"}`

#### Scenario: Title lookup is case-insensitive and exact
- **WHEN** `query` is a non-UUID string
- **THEN** the server looks up documents in the workspace where `lower(title) = lower(query)` (full-string match, not substring)
- **AND** returns `{matched: [<uuid>, ...], ambiguous: <bool>, kind: "title"}`
- **AND** `ambiguous` is true iff `matched.length > 1`

#### Scenario: No match returns empty
- **WHEN** no document matches `query` in the workspace
- **THEN** the response is `{matched: [], ambiguous: false, kind: "id"|"title"}` with HTTP 200 (not 404 — frontend renders "broken link" affordance)

### Requirement: Cross-Site Request Forgery protection on mutating endpoints
All mutating endpoints under `/api/v1/*` (POST, PUT, DELETE) SHALL apply a CSRF middleware with the following decision order (top match wins):

1. **`X-Requested-With: nano-brain-ui` header present** → allow (the trusted client header — embedded SPA always sends this).
2. **Both `Origin` and `Referer` absent** → allow (non-browser client: CLI, curl, MCP, Go HTTP client).
3. **`Origin: null`** → reject with `403` (sandboxed iframe / opaque origin).
4. **`Origin` present and host is `127.0.0.1`, `localhost`, or `::1` matching the bound listen address** → allow (a local browser hitting the API directly).
5. **`Origin` present but mismatches the bound listen address** → reject with `403`.
6. **`Origin` absent but `Referer` present and matches the bound listen address** → allow (Origin-stripping proxy edge case).
7. **Otherwise** → reject with `403`.

#### Scenario: Browser POST without CSRF header from foreign origin is rejected
- **WHEN** a client posts to `/api/v1/write` with `Origin: http://malicious.example.com` and no `X-Requested-With` header
- **THEN** the server returns `403`

#### Scenario: CLI / curl request without Origin or Referer is accepted
- **WHEN** a client posts to `/api/v1/write` with no `Origin` and no `Referer` header (typical CLI/curl/MCP traffic)
- **THEN** the server processes the request normally
- **SO THAT** existing non-browser clients are unaffected

#### Scenario: UI request includes the trusted header
- **WHEN** the embedded SPA issues `POST /api/v1/write` with `X-Requested-With: nano-brain-ui`
- **THEN** the server processes the request normally

#### Scenario: Sandboxed iframe with Origin: null is rejected
- **WHEN** a client posts with `Origin: null` and no `X-Requested-With`
- **THEN** the server returns `403`

#### Scenario: Same-origin browser without trusted header is allowed
- **WHEN** a client posts with `Origin: http://localhost:3100` (matches bound listen) and no `X-Requested-With`
- **THEN** the server processes the request normally
- **SO THAT** a user hitting the API directly from a same-origin browser tab can still write
