# E2E Test Report — fix-stats-api-contract (#279)

Date: 2026-06-01
Server: localhost:3199 (dev build from `fix/279-stats-api-contract` branch)
Browser: Chrome DevTools (Chromium 148)

## Test #1: Stats API response shape ✅ PASS

```bash
$ curl 'http://localhost:3199/api/v1/stats?workspace=37b36e...' | jq 'keys'
[
  "chunks_by_embed_status",
  "chunks_total",
  "collections",
  "docs_total",
  "embedding",
  "embeddings_total",
  "graph_edges_by_type",
  "harvest",
  "migration_version",
  "recent_docs",
  "server_version",
  "tags_top_20",
  "uptime_sec",
  "watcher"
]
```

All 14 expected fields present. No legacy `chunks`, `graph_edges`, `top_tags`, `recent_queries`.

Sample populated values:
- `server_version: "dev"`, `uptime_sec: 73`, `migration_version: 12`
- `docs_total: 8812`, `chunks_total: 12704`, `embeddings_total: 12619`
- `chunks_by_embed_status: {pending: 5, embedded: 12619, embed_failed: 80}` (object, not array)
- `graph_edges_by_type: {calls: 22118, contains: 7416, imports: 4315}` (object, not array)
- `embedding: {provider: "ollama", model: "nomic-embed-text", dim: 0}`
- `watcher: {collections_watched: 5, debounce_ms: 2000, poll_interval_sec: 86400, dirty: 0}`
- `harvest: {mode: "db_path", last_at: "", sessions_seen: 0}`
- `tags_top_20[0]: {tag: "symbol", count: 5296}` (count, not doc_count)

## Test #2: Dashboard renders without errors ✅ PASS

Loaded http://localhost:3199/ui/dashboard?workspace=37b36e... in browser via Chrome DevTools.

Dashboard renders correctly with all cards populated:
- SERVER: vdev, up 0h 1m
- EMBEDDINGS: 12,619, ollama · nomic-embed-text
- DOCUMENTS: 8,812
- CHUNKS: 12,704, 99% embedded
- GRAPH EDGES: 33,849, contains · imports · calls
- EMBED QUEUE: 0, idle
- Embed status breakdown (pending 5, embedded 12,619, failed 80)
- Graph cardinality by edge type (calls 22,118, contains 7,416, imports 4,315)
- Recent documents (10 entries with timestamps)

Console: no errors.

Screenshot: `ui-dashboard-working.png` (full page).

## Test #3: Workspace selector still works (regression check for #277) ✅ PASS

Previously broken before #277, fixed in v2026.6.4. Re-verified still works: dropdown lists all 19 workspaces with name, hash prefix, doc count.

## Out of Scope (filed as follow-up)

During exploration, Memory page (`/ui/memory`) crashes with 404 on `GET /api/v1/query`. Backend only supports POST. This is a method/route mismatch unrelated to stats endpoint. Will be filed as a separate issue.

Other observed FE→BE endpoint mismatches (need investigation but NOT part of this PR):
- `DELETE /api/v1/documents/:id` — frontend uses it, backend route TBD
- `POST /api/v1/reset-embeddings` — frontend uses it, backend route TBD

This PR (#279) successfully fixes the stats endpoint contract drift, enabling the dashboard to render. Other pages will be unblocked in follow-up PRs as their drift is identified and fixed.

## Verdict

- **Stats API contract fix WORKS** ✅
- **Dashboard renders fully** ✅
- **No regression on workspaces** ✅
- **Other UI pages have separate bugs** — out of scope, tracked separately
