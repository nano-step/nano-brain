## Why

`/api/v1/stats?workspace=<hash>` returns a shape that doesn't match the frontend `StatsResponse` interface. Dashboard crashes after workspace selection with "Cannot convert undefined or null to object" because backend returns 6 fields while frontend expects 14 — including object-shaped chunks_by_embed_status and graph_edges_by_type that backend returns as arrays.

This is the second known case of API contract drift after #277 (workspaces endpoint). Same root cause: no schema source of truth between Go handlers and TS types.

This change unblocks the /ui dashboard immediately. The systemic fix (schema-driven codegen via trpcgo or OpenAPI) is tracked separately.

## What Changes

- Refactor `Stats` handler to inject server context (version, startTime, embedding config, migration version, harvester status, watcher status) instead of pure DB querier.
- Add SQL aggregates: `CountChunksByWorkspace`, `CountEmbeddingsByWorkspace`.
- Reshape response:
  - `chunks` array → `chunks_by_embed_status` object `{pending, embedded, embed_failed}`
  - `graph_edges` array → `graph_edges_by_type` object `{contains, imports, calls, references, ...}`
  - `top_tags` → `tags_top_20`
  - TagCount field `doc_count` → `count` (frontend convention)
- Add 7 new fields: `server_version`, `uptime_sec`, `embedding {provider, model, dim}`, `migration_version`, `docs_total`, `chunks_total`, `embeddings_total`, `harvest {mode, last_at, sessions_seen}`, `watcher {collections_watched, debounce_ms, poll_interval_sec, dirty}`.
- Update route wiring in `routes.go` to pass server context.
- Add regression test asserting full shape matches frontend interface.
- E2E verify dashboard renders without errors on dev port 3199.

## Capabilities

### New Capabilities
- `stats-api-contract`: Pins the canonical shape of `GET /api/v1/stats?workspace=<hash>` response. Includes all 14 fields with correct nested types matching `web/src/api/types.ts:StatsResponse`. Regression test catches future drift.

### Modified Capabilities
None.

## Impact

- **Code:** `internal/server/handlers/stats.go`, `internal/server/routes.go`, `internal/storage/queries/stats.sql` (new aggregates), `internal/storage/queries/embeddings.sql`, `internal/storage/sqlc/*.sql.go` (regenerated).
- **Behavior:** Dashboard renders correctly. No external API consumers affected — CLI doesn't use /stats.
- **Risk:** Low — bug-fix to align with frontend. Aggregates use existing tables, no schema migration.
- **Performance:** 2 additional COUNT(*) queries per stats call. Negligible (<10ms each on indexed tables).
- **Database:** No migration. New `:one` queries only.

## Out of Scope

- Schema-driven contract (trpcgo migration) — separate proposal, terminal fix for class of bug.
- Other endpoints with drift (e.g., /collections, /tags) — file follow-up issues if encountered during E2E.
- Reshape harvest/watcher status into shared types used by both /api/status and /api/v1/stats — defer; for now stats has its own structs that match frontend interface.
