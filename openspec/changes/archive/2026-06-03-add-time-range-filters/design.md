## Context

PR #359 (merged 2026-06-03T12:19Z) shipped snippet-only response shape + cursor pagination for `memory_query`, `memory_search`, `memory_vsearch`. That fixed token bloat and let agents page through results without re-running the query.

It did NOT fix the upstream problem: time-bounded questions ("what bugs have we fixed in the last 30 days?") still require fetching results, parsing `updated_at` client-side, filtering, and looping. On a workspace with months of history and a narrow window, an agent may fetch 100+ snippets to find 10 in the target window — pure waste.

This change pushes the time filter into the SQL `WHERE` clause so a single round-trip returns only the documents within the window.

**Current state**:
- 8 sqlc queries handle search (`BM25Search` × 4 tag variants, `VectorSearch` × 4 tag variants)
- All queries already filter by `workspace_hash`, optionally by `collections` and `tags`
- `documents.created_at` and `documents.updated_at` columns exist (timestamptz, NOT NULL)
- Cursor query-hash (introduced in #359) includes query text + filter values to invalidate on filter changes

**Constraints**:
- Default behavior (filters omitted) MUST compile to the same SQL plan as today — no regression for the 95%+ of calls that omit time filters
- Cursor pagination semantics must hold: paging through filtered results MUST NOT skip or duplicate
- Performance budget: ≤10% median latency regression on the omit-all-filters common case; "same or better" than client-side filter pattern when one filter is set

## Goals / Non-Goals

**Goals:**
- Three MCP tools, three REST handlers, AND three CLI commands accept four optional time-range parameters (`created_after`, `created_before`, `updated_after`, `updated_before`)
- All filters combine with AND semantics
- Accept both RFC3339 timestamps (`"2026-05-04T12:00:00Z"`) AND relative durations (`"30d"`, `"720h"`, `"1w"`); reject negative / zero durations
- Default-omit case produces SQL plan identical to today (verified by EXPLAIN ANALYZE)
- Pagination cursors invalidate when ANY filter changes (query / tags / scope / collections / time-range) — fixes pre-existing bug
- Integration tests cover: single filter, all-four-combined, relative parsing, invalid input rejection, negative-duration rejection, inverted-range empty result
- SKILL.md updated with worked examples

**Non-Goals:**
- Chunk-level time filtering (chunks inherit document timestamps — sufficient for this use case)
- Time-bucketed aggregations / faceting ("group by day") — out of scope
- Per-tool overrides on default time window — agents pass filters explicitly each call
- Changes to harvest or write paths — pure read-path change
- Natural-language time parsing (`"yesterday"`, `"last monday"`) — out of scope, NLP rabbit-hole
- Deprecating `RecentDocuments` / `memory_wake_up` — semantic overlap acknowledged but unification is a separate concern
- Search telemetry columns for filter-usage tracking — defer to follow-up issue

## Decisions

### D1: Filter at document level, not chunk level

**Decision**: Filter on `documents.created_at` and `documents.updated_at` via JOIN, not on chunk-level timestamps.

**Rationale**:
- Chunks have no independent timestamp column today; adding one is a migration with backfill cost
- All chunks of a document share the document's lifecycle — if a user wrote a note today, all its chunks are "today's chunks". Per-chunk timestamps would only differ if we re-chunked partial documents, which we don't
- Existing queries already JOIN `chunks → documents` for `workspace_hash` and `collection` filters; piggybacking the time WHERE on that JOIN is zero-cost structurally

**Alternative considered**: Add `chunks.indexed_at` column + backfill migration. Rejected — needless complexity and migration risk for a behavioral identical filter at the document granularity we need.

### D2: Time parser accepts RFC3339 OR Go duration OR humanish relative

**Decision**: New `internal/timefilter` package with single function `Parse(input string, now time.Time) (time.Time, error)` accepting:
1. RFC3339 (`"2026-05-04T12:00:00Z"`) — absolute
2. Go-style duration (`"720h"`, `"30m"`) — relative, subtracted from `now`
3. Humanish relative (`"30d"`, `"1w"`, `"2mo"`, `"1y"`) — relative, subtracted from `now`. Units: `s`, `m`, `h`, `d`, `w`, `mo` (30 days), `y` (365 days)

Parser tried in that order; first successful match wins. `now` is injected (not `time.Now()` inside the package) so tests are deterministic.

**Negative / zero duration guard (added per Oracle review C3)**: After ANY successful relative-duration parse (Go or humanish), the result is computed as `now.Add(-d)` where `d` MUST be `> 0`. If the parsed duration is zero or negative (`"-30d"`, `"0d"`, `"-720h"`), the parser returns an error before any time arithmetic. Rationale: `time.ParseDuration("-720h")` succeeds and would silently produce a *future* cutoff (`now - (-720h) = now + 720h`), which is never what an agent means by "updated_after". RFC3339 inputs are NOT subjected to this check — an agent passing a future RFC3339 timestamp is making an explicit choice (e.g., bound future-dated documents).

**Rationale**:
- RFC3339 covers absolute "since 2026-05-01" use cases (auditor / reproducible queries)
- Go durations are familiar to anyone reading config (`time.ParseDuration`-compatible)
- `30d`/`1w`/`1mo` is what users actually type in chat queries — covering it eliminates a foot-gun
- 30-day month and 365-day year are explicitly approximate — documented in SKILL.md as "rough relative", not calendar-arithmetic. If anyone needs calendar precision they pass RFC3339.

**Alternative considered**: RFC3339 only. Rejected — forces every agent to do `time.Now().Add(-30 * 24 * time.Hour).Format(time.RFC3339)`, which is exactly the verbosity we're trying to eliminate at the agent layer.

**Alternative considered**: Parse `"yesterday"`, `"last monday"` (natural language). Rejected — out of scope, NLP rabbithole, breaks determinism.

### D3: SQL filter via named-arg IS-NULL guards

**Decision**: For each of the 8 sqlc queries (4 in `search.sql`, 4 in `embeddings.sql`), add four optional sqlc named parameters and four `AND` clauses guarded by `IS NULL OR ...` so omitted params compile to a no-op:

```sql
-- existing WHERE ...
AND (@updated_after::timestamptz IS NULL OR d.updated_at >= @updated_after)
AND (@updated_before::timestamptz IS NULL OR d.updated_at <= @updated_before)
AND (@created_after::timestamptz IS NULL OR d.created_at >= @created_after)
AND (@created_before::timestamptz IS NULL OR d.created_at <= @created_before)
```

The Go caller passes `sql.NullTime` with `Valid: false` for omitted filters — NOT `pgtype.Timestamptz`. Confirmed by reading `sqlc.yaml` and `internal/storage/sqlc/db.go`: the project uses sqlc's `database/sql` driver (bridged from pgx via `stdlib.OpenDBFromPool`), which generates `sql.NullTime` for nullable timestamp columns and accepts named-arg parameters as `sql.NullTime` values.

PostgreSQL's planner short-circuits `NULL IS NULL` predicates at planning time, so when all four params are NULL the WHERE reduces to today's plan exactly. EXPLAIN ANALYZE on the omit-all path MUST show identical scan/index node selection compared to master pre-change.

**Rationale**:
- Single sqlc query file per search type — no `if` / dynamic SQL string-building in Go
- Type safety preserved (sqlc generates `pgtype.Timestamptz` params, not `interface{}`)
- The `param IS NULL OR <pred>` pattern is the documented sqlc idiom for optional filters and the PG planner handles it efficiently
- Combined with named args, the 4 params are positional-independent — easy to read in callsites

**Alternative considered**: Per-filter sqlc query variants (combinatorial explosion — 8 base × 16 filter combinations = 128 queries). Rejected, obviously.

**Alternative considered**: String-template the WHERE clause in Go. Rejected — defeats sqlc type safety, makes the codebase chaotic, hurts reviewability.

### D4: Index strategy — add btree indexes (confirmed missing)

**Decision**: Ship a new goose migration `00015_add_documents_timestamp_indexes.sql` adding two btree indexes UNCONDITIONALLY:

```sql
-- +goose Up
CREATE INDEX idx_documents_created_at ON documents(created_at);
CREATE INDEX idx_documents_updated_at ON documents(updated_at);

-- +goose Down
DROP INDEX IF EXISTS idx_documents_updated_at;
DROP INDEX IF EXISTS idx_documents_created_at;
```

**Rationale (revised per Oracle review C2 + Metis Gap 4)**: Deep-design code-reading grepped all 14 existing migrations for `idx.*documents.*(created_at|updated_at)` — zero hits. The indexes are **definitely missing**. Without them, the planner will degrade to seq scan on `documents` when the time predicate becomes selective enough to be chosen as a driver, or will fail to use the documents JOIN efficiently as a secondary filter.

The documents table is low-write (writes happen on harvest / API write — not request hot-path), so btree index maintenance overhead is negligible. BRIN was considered and rejected — BRIN excels on sequentially-correlated columns at huge scale (>10M rows), but our documents table is ≤100k rows for typical workspaces and `updated_at` is not strictly monotonic (any reindex updates it).

**Required EXPLAIN ANALYZE evidence** (block implementation if any fails):
1. Run on `nanobrain_dev` against a workspace with ≥10k chunks.
2. Baseline EXPLAIN (master, no filters) → captured in `docs/evidence/issue-360-explain-baseline.md`.
3. After-change EXPLAIN with all 4 filters NULL → MUST show identical scan/index node selection vs baseline (same plan = same cost = no regression).
4. After-change EXPLAIN with `updated_after = now - 30d` set → planner MUST use either chunks-side BM25/HNSW index as driver with documents JOIN as filter, OR `idx_documents_updated_at` as driver when selectivity is high. NOT a sequential scan on either side.
5. Latency on omit-all case MUST be within 10% of master pre-change (median over 1000 calls).

### D5: Cursor query-hash includes ALL filter values (also fixes pre-existing bug)

**Decision**: Extend the cursor query-hash function from #359 (`internal/search/cursor.go:QueryHash`) to hash ALL filter inputs, not just query text. The new hash input is the concatenation (with delimiter) of:

1. The query text
2. Tags (sorted, joined)
3. Scope (string)
4. Collections (sorted, joined)
5. The four time-range filter **raw input strings** (`"30d"`, `"2026-05-04T00:00:00Z"`, or literal `"null"` if omitted)

**Critical**: hash the **raw input strings**, NOT the parsed absolute times (Oracle revision R1). Relative durations like `"30d"` produce a different absolute time on each call as `now` shifts. Hashing the absolute time would invalidate cursors between pages (seconds apart). Hashing `"30d"` keeps the cursor stable across paginated calls. The small drift between page 1 and page 2 absolute windows is acceptable per the "rough relative" caveat in D2.

**Pre-existing bug fix (per Oracle confirmation)**: Code reading of `internal/search/cursor.go` showed `QueryHash()` only hashes the query string. Today, changing tags or scope between paginated calls silently returns wrong results. This change extends `QueryHash` to incorporate all filter inputs, fixing that bug as a side-effect. Documented in proposal and tasks as explicit scope.

**Rationale**: Pagination is offset-into-result-set. Different filters = different result set = same offset is meaningless. Reusing existing #359 invalidation machinery (`VerifyCursor` returns `cursor invalidated, restart pagination` error) means zero new error-path code.

### D6: Validation order — parse before any DB call

**Decision**: In each handler (REST + MCP), parse all four time filters BEFORE invoking the search pipeline. Invalid duration / RFC3339 returns HTTP 400 (REST) / MCP error (MCP). Errors include the offending parameter name and the rejected string.

**Rationale**: Fail-fast on bad input, no DB round-trip wasted, error messages give agents exactly what they need to retry.

## Risks / Trade-offs

| Risk | Mitigation |
|------|------------|
| Query plan regression when one filter is set (e.g., planner picks documents-index scan when it should drive from chunks) | EXPLAIN ANALYZE on 10k-chunk fixture as design gate; integration test asserts plan choice for representative cases |
| 30-day month / 365-day year is "wrong" for users who expect calendar arithmetic | Documented as "rough relative" in SKILL.md; calendar-precise users pass RFC3339 |
| Filter values in cursor hash break existing pagination flows mid-flight | One-time disruption acceptable; #359 just shipped so few users have persisted cursors. Restart pagination is the documented behavior on any query-shape change |
| Time-zone confusion: agent passes `"2026-05-04"` (date-only, no zone) | Parser rejects date-only input — only RFC3339 (timezone-required) or relative durations accepted; clear error message tells agent to add `T00:00:00Z` |
| Coalesced `NULL OR ...` predicates somehow hurt planner on omit-all path | EXPLAIN ANALYZE comparison master vs branch on omit-all path is a hard merge gate |
| Extending `QueryHash` to include all filter inputs invalidates persisted cursors that older clients may hold | One-time disruption acceptable. #359 just shipped 2026-06-03; clients have not yet built up large persistent-cursor state. Restart-pagination behavior is already the documented contract on cursor invalidation |
| Negative duration parser bypassed via RFC3339 future timestamp (`"2099-01-01T00:00:00Z"` as `updated_after`) | Accepted as explicit user intent — RFC3339 is absolute and not validated against `now`. Documented in SKILL.md as intentional |
| Hashing raw input strings means `"30d"` cursors drift slightly between calls (page 1 window vs page 2 window differ by seconds) | Acceptable per D2 "rough relative" caveat. Documents added between page 1 and page 2 at the boundary may appear/disappear — same behavior as today's tag-filtered pagination when documents are tagged mid-query |

## Migration Plan

No data migration required. Pure additive code change.

**Rollback**: revert the PR. No schema changes (unless D4 verification reveals missing indexes, in which case the migration is additive and reversible). Existing API callers ignore the new optional params — no breakage.

**Deployment**: ships in the next auto-tagged release after merge. Agents adopt at their own pace by adding the new optional params to their MCP tool calls.

## Open Questions

(none — all four issue-body open questions resolved in Decisions above)
