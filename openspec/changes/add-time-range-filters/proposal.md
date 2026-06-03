## Why

Agents asking time-bounded questions ("what bugs have we fixed in the last 30 days?") cannot push the time filter down to the database today. They must fetch a page of results, filter client-side on `updated_at`, then paginate again — often requesting 100+ results to find 10 in the desired window. This wastes context tokens and round-trips on a filter the database could apply directly.

PR #359 (just merged) gave us snippet-only responses + cursor pagination, which made the wasteful pattern survivable but did not eliminate it. Direct `updated_after` / `created_after` filter parameters on `memory_query`, `memory_search`, and `memory_vsearch` finish what #358 started: one-round-trip time-bounded retrieval.

## What Changes

- **MCP tool schema (3 tools)**: `memory_query`, `memory_search`, `memory_vsearch` each accept four new optional parameters:
  - `updated_after` — RFC3339 timestamp OR relative duration (`"30d"`, `"720h"`, `"1w"`)
  - `updated_before` — same format
  - `created_after` — same format
  - `created_before` — same format
  All optional, all combinable with AND semantics. Default behavior unchanged when omitted.
- **REST handlers** (`internal/server/handlers/`): same four parameters added to `/api/v1/query`, `/api/v1/search`, `/api/v1/vsearch` request bodies.
- **Storage layer** (`internal/storage/queries/`): 8 sqlc queries gain optional `WHERE created_at`/`updated_at` clauses via named-arg `IS NULL OR ...` guards, so omitted filters compile to the same plan as today.
  - BM25 (`search.sql`): `BM25Search`, `BM25SearchAll`, `BM25SearchWithTags`, `BM25SearchAllWithTags`
  - Vector (`embeddings.sql`): `VectorSearch`, `VectorSearchAll`, `VectorSearchWithTags`, `VectorSearchAllWithTags`
- **Indexes** (`migrations/00015_add_documents_timestamp_indexes.sql` — NEW): add `idx_documents_created_at` and `idx_documents_updated_at` btree indexes. Confirmed missing across all 14 existing migrations during deep-design review.
- **CLI commands** (`cmd/nano-brain/commands.go`): `nano-brain query/search/vsearch` gain matching `--created-after`/`--created-before`/`--updated-after`/`--updated-before` flags for surface parity.
- **Search pipeline** (`internal/search/`): pass new filter struct through `Query`, `Search`, `VSearch` entry points. Cursor `QueryHash()` is extended to hash ALL filter inputs (query + tags + scope + collections + time-range raw strings) — this also fixes a pre-existing bug discovered during deep-design review where tag/scope changes between paginated calls silently returned wrong results.
- **Time parser** (new, `internal/timefilter/`): tiny helper that accepts RFC3339 OR Go-style duration (`time.ParseDuration` — `"720h"`) OR humanish relative (`"30d"`, `"1w"`, `"2mo"`) and returns an absolute `time.Time` (now-anchored for relative).
- **Index strategy**: confirm existing `documents.updated_at` / `documents.created_at` indexes are sufficient; add if missing. EXPLAIN ANALYZE on 10k-chunk workspace required as part of design validation.
- **MCP SKILL.md docs**: add time-range usage examples.

No breaking changes. All new parameters are optional. Cursor-pagination semantics from #358 are preserved by including filter values in the cursor query-hash.

## Capabilities

### New Capabilities

(none — extending existing capabilities)

### Modified Capabilities

- `mcp` — `memory_query`, `memory_search`, `memory_vsearch` tool schemas gain four optional time-range parameters
- `search-pipeline` — BM25 and vector queries accept optional `created_at`/`updated_at` bounds; cursor query-hash includes filter values

## Impact

**Affected code**:
- `internal/mcp/` — tool input schemas (3 tools: memory_query, memory_search, memory_vsearch)
- `internal/server/handlers/query.go`, `search.go`, `vsearch.go` — request DTO + plumbing
- `internal/storage/queries/search.sql` — 4 BM25 query rewrites
- `internal/storage/queries/embeddings.sql` — 4 vector query rewrites
- `internal/storage/sqlc/` — regenerated (DO NOT EDIT)
- `migrations/00015_add_documents_timestamp_indexes.sql` (NEW) — btree indexes on documents.created_at / updated_at
- `internal/search/pipeline.go` — thread `TimeRangeFilter` struct through `Query`/`Search`/`VSearch`
- `internal/timefilter/` (NEW) — relative-duration parser with negative-duration guard
- `internal/search/cursor.go` — extend `QueryHash()` to hash ALL filter inputs (query + tags + scope + collections + time-range raw strings)
- `cmd/nano-brain/commands.go` — CLI flag wiring for query/search/vsearch
- `.opencode/skills/nano-brain/SKILL.md` — usage examples
- `README.md` — one-line additions to MCP Tools table

**APIs**:
- MCP tool schema additions (backward compatible — additive optional params)
- REST request body additions (backward compatible — additive optional params)

**Dependencies**: none (#358/PR #359 already merged 2026-06-03T12:19Z)

**Risk**: query-plan regression on existing search paths if WHERE clauses interact badly with HNSW or GIN index selection. Must run EXPLAIN ANALYZE in design phase on a representative 10k-chunk workspace BEFORE implementation begins.

**Performance budget**: median search latency MUST NOT regress more than 10% when all four filters are omitted (the common case). With one filter set on an indexed column, latency target is "same or better" than the current paginate-and-client-filter pattern.
