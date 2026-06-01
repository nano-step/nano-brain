## Context

`/api/v1/stats` is consumed by `web/src/panels/DashboardPanel.tsx` via `useStats` hook. Dashboard renders a grid of cards: version + uptime, embedding stats, docs/chunks totals, graph edges, recent docs, collection breakdown.

Backend handler `internal/server/handlers/stats.go` currently aggregates per-workspace data from PG (`stats.sql.go`):
- `CountDocsByCollectionGrouped` → array of `{collection, doc_count}`
- `CountChunksByEmbedStatus` → array of `{embed_status, chunk_count}`
- `CountGraphEdgesByType` → array of `{edge_type, edge_count}`
- `ListTopTags` → array of `{tag, doc_count}`
- `ListRecentDocuments` → array of doc summaries
- `ListRecentQueries` → array of recent search queries

Frontend interface (web/src/api/types.ts) expects:
- Server-level info: `server_version`, `uptime_sec`, `migration_version`, `embedding {provider, model, dim}`
- Aggregated totals: `docs_total`, `chunks_total`, `embeddings_total`
- Per-workspace breakdown: `chunks_by_embed_status` (object), `graph_edges_by_type` (object), `tags_top_20`, `collections`, `recent_docs`
- Subsystem status: `harvest`, `watcher`

Stats endpoint must aggregate per-workspace data AND inject server-level context.

## Goals / Non-Goals

**Goals:**
- Align /api/v1/stats response with frontend StatsResponse interface (14 fields, correct shapes).
- Reuse existing aggregate queries; add minimal new ones for totals.
- Inject server context (version, uptime, embedding config, etc.) via constructor pattern matching Health handler.
- Regression test pins the contract.
- E2E verification on dev port 3199 shows dashboard renders.

**Non-Goals:**
- Schema-driven contract codegen (separate proposal).
- Refactoring Health/Status handlers to share types with Stats.
- Optimizing aggregate query performance beyond what already exists.

## Decisions

### D1: Stats handler becomes struct with injected context

**Decision:** Convert `Stats(q, logger)` constructor function into a `StatsHandler` struct that holds queries + server context (version, startTime, embedding config, migration version, getCfg func for harvest, watcher status getter). Pattern matches existing `Health` struct in `health.go:66`.

**Rationale:**
- Frontend needs server-level fields (version, uptime, embedding info) — these don't come from per-workspace queries.
- Constructor injection keeps handler testable.
- Health handler already does this; consistent pattern.

**Alternative considered:** Pass closure for each context field. Rejected — too many args, ugly.

### D2: chunks_by_embed_status as object, not array

**Decision:** Response field `chunks_by_embed_status` is an object `{pending: int64, embedded: int64, embed_failed: int64}`. Backend transforms array result from `CountChunksByEmbedStatus` into this fixed-key object.

**Rationale:**
- Frontend access pattern: `stats.chunks_by_embed_status.embedded`.
- Object shape is self-documenting; array requires consumer to find by status string.
- Object explicitly enumerates valid status values; array allows unknown statuses to leak through.

**Alternative considered:** Keep as array, frontend reduces to object. Rejected — frontend already typed as object, would require frontend change. Backend reshape is cheaper.

### D3: graph_edges_by_type as object

**Decision:** Same as D2 but for graph edges. Object keyed by edge type (`contains`, `imports`, `calls`, `references`), values are counts. Unknown edge types are included as additional keys (frontend uses `[key: string]: number | undefined` index signature).

**Rationale:** Same as D2.

### D4: New SQL aggregates

**Decision:** Add two `:one` queries:

```sql
-- name: CountChunksByWorkspace :one
SELECT COUNT(*) FROM chunks WHERE workspace_hash = $1;

-- name: CountEmbeddingsByWorkspace :one
SELECT COUNT(*) FROM embeddings e
JOIN chunks c ON e.chunk_id = c.id
WHERE c.workspace_hash = $1;
```

`CountDocumentsByWorkspace` already exists.

**Rationale:** Frontend needs totals. Cheaper to count in DB than sum the grouped result.

**Alternative considered:** Sum the existing `CountChunksByEmbedStatus` rows in Go. Rejected — explicit COUNT is faster and clearer.

### D5: TagCount field rename

**Decision:** Backend `tagCount` struct field JSON tag `doc_count` → `count`. Field name matches frontend `TagCount {tag, count}`.

**Rationale:** Same convention as workspaces fix (#277). Frontend is source of truth for FE-facing field names.

**Alternative considered:** Keep `doc_count` and update frontend. Rejected — `count` is the more natural name for a tag aggregate.

### D6: tags_top_20 rename

**Decision:** Rename `top_tags` → `tags_top_20` in JSON response.

**Rationale:** Matches frontend interface field name. Number 20 is the existing limit in `ListTopTags` query.

### D7: HarvestInfo + WatcherInfo: best-effort population

**Decision:** Populate `harvest {mode, last_at, sessions_seen}` from existing `Health.harvestStatus` data (same data source as `/api/status` harvester block). Populate `watcher {collections_watched, debounce_ms, poll_interval_sec, dirty}` from watcher + config.

For fields not yet tracked (e.g., `sessions_seen`, `dirty` counter), populate with 0 / empty string and add TODO comments — frontend handles undefined gracefully.

**Rationale:**
- Unblocks /ui without requiring new instrumentation.
- Each future field has clear path to populate properly.

**Alternative considered:** Defer harvest/watcher fields entirely. Rejected — frontend renders these cards; would show empty boxes.

### D8: routes.go wiring

**Decision:** Replace `data.GET("/stats", handlers.Stats(s.queries, s.logger))` with `data.GET("/stats", s.statsHandler.Handle)` where `s.statsHandler` is constructed in `server.New` with full context.

**Rationale:** Mirrors Health handler wiring pattern.

## Risks / Trade-offs

| Risk | Mitigation |
|---|---|
| Stats handler now depends on more server state | Constructor injection keeps testability; mock in tests via interfaces |
| harvest/watcher placeholder fields show 0 | Document in code comments; future PRs add real data sources |
| TagCount rename affects MCP tools or CLI consumers | Verified: no other consumers; `/api/v1/tags` is separate endpoint |
| Regression test brittle on new fields | Test asserts presence + type only, not exact values (for dynamic counts) |
