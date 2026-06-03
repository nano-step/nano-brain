## 1. Index Migration + Baseline EXPLAIN (BLOCKER — must complete before SQL changes)

- [x] 1.1 Confirm via `\d documents` on `nanobrain_dev` that no indexes exist on `created_at` / `updated_at` (expected per deep-design — all 14 migrations grepped show zero hits)
- [x] 1.2 Write new migration `migrations/00015_add_documents_timestamp_indexes.sql` with `CREATE INDEX idx_documents_created_at ON documents(created_at)` and `CREATE INDEX idx_documents_updated_at ON documents(updated_at)`, plus matching `+goose Down` drops
- [x] 1.3 Run migration on `nanobrain_dev` and verify with `\d documents` that both indexes are present
- [x] 1.4 Run `EXPLAIN ANALYZE` on representative `BM25SearchAll` and `VectorSearchAll` queries (no filters) against a workspace with ≥10k chunks (master pre-change behavior — even with the new index present, no time WHERE means index is unused); capture to `docs/evidence/issue-360-explain-baseline.md`
- [x] 1.5 If baseline plan uses sequential scan on `documents` for the unfiltered path, STOP and adjust design before proceeding

## 2. Time Filter Parser (new package)

- [x] 2.1 Create `internal/timefilter/parser.go` with `Parse(input string, now time.Time) (time.Time, error)` — tries RFC3339, then `time.ParseDuration`, then humanish (`30d`/`1w`/`2mo`/`1y`)
- [x] 2.2 Define error type that includes the parameter name (set by caller via wrapping) and the offending input
- [x] 2.3 Negative/zero-duration guard: for ANY relative-duration parse (Go or humanish), compute the cutoff with `now.Add(-d)` where `d` MUST be `> 0`. Reject `-30d`, `-720h`, `0d`, `0h` with explicit error. RFC3339 inputs are NOT subjected to this check (per design D2)
- [x] 2.4 Write `internal/timefilter/parser_test.go` covering: RFC3339 happy path (incl. future date allowed), all Go duration units, all humanish units (`s`/`m`/`h`/`d`/`w`/`mo`/`y`), case-insensitivity, leading/trailing whitespace, invalid input (`banana`, `30`, `30x`), date-only rejection (`2026-05-04`), empty string rejection, negative-duration rejection (`-30d`, `-720h`), zero-duration rejection (`0d`, `0h`)
- [x] 2.5 Verify `go test -race -short ./internal/timefilter/...` passes

## 3. SQL Query Updates (8 queries across 2 files)

- [x] 3.1 Edit `internal/storage/queries/search.sql` — add 4 named-arg IS-NULL-guarded clauses to each of: `BM25Search`, `BM25SearchAll`, `BM25SearchWithTags`, `BM25SearchAllWithTags`
- [x] 3.2 Edit `internal/storage/queries/embeddings.sql` — add same 4 clauses to each of: `VectorSearch`, `VectorSearchAll`, `VectorSearchWithTags`, `VectorSearchAllWithTags`
- [x] 3.3 Clauses use named args: `(@updated_after::timestamptz IS NULL OR d.updated_at >= @updated_after)` etc. — total 4 clauses per query, all combined with AND
- [x] 3.4 Run `sqlc generate` (or `make sqlc` if present) to regenerate `internal/storage/sqlc/` typed wrappers
- [x] 3.5 Verify the regenerated `*Params` structs include the four new `sql.NullTime` fields (NOT `pgtype.Timestamptz` — project uses `database/sql` driver per `sqlc.yaml`)
- [x] 3.6 Run `EXPLAIN ANALYZE` on each modified query with all four time params NULL on the same 10k-chunk fixture used in 1.4; capture to `docs/evidence/issue-360-explain-omitall.md`
- [x] 3.7 Verify the omit-all plan is byte-identical (or only differs in row-count estimates) to the baseline captured in 1.4 — hard regression gate
- [x] 3.8 Run `EXPLAIN ANALYZE` on each modified query with `updated_after` set to a 30-day-ago timestamp; capture to `docs/evidence/issue-360-explain-filtered.md`. Verify the planner uses chunks-side index as driver with documents JOIN as filter, OR uses `idx_documents_updated_at` as driver when selectivity warrants. No seq scan on either side

## 4. Search Pipeline Plumbing (includes pre-existing cursor-hash bug fix)

- [x] 4.1 Define `TimeRangeFilter` struct in `internal/search/`: four `*time.Time` fields (nil = omitted) plus the original raw input strings preserved for cursor-hash use, plus a method to convert each to `sql.NullTime`
- [x] 4.2 Thread `TimeRangeFilter` through `Query`, `Search`, `VSearch` entry points in `internal/search/pipeline.go` and pass to the regenerated sqlc `*Params` structs
- [x] 4.3 Extend `internal/search/cursor.go:QueryHash` to hash ALL filter inputs: query text + tags (sorted+joined) + scope + collections (sorted+joined) + the four time-range RAW input strings (NOT parsed absolute times — per Oracle revision R1; see design D5). Use a stable delimiter that cannot appear in any input (e.g. `\x1f` ASCII unit separator)
- [x] 4.4 This change fixes a pre-existing bug discovered during deep-design: master `QueryHash` only hashes query text, so tag/scope/collection changes between paginated calls silently returned wrong results. Document in commit message
- [x] 4.5 Write unit tests in `internal/search/cursor_test.go`: (a) same query + same all-filters → same hash; (b) query change → different hash; (c) tag change → different hash (regression test for pre-existing bug); (d) scope change → different hash; (e) collections change → different hash; (f) any time-range change → different hash; (g) raw `"30d"` hashed twice with different `now` values → SAME hash (proves raw-string approach prevents drift)
- [x] 4.6 Verify `go test -race -short ./internal/search/...` passes

## 5. Handler Layer (REST + MCP + CLI)

- [x] 5.1 Add 4 optional time-range fields to REST request DTOs in `internal/server/handlers/query.go`, `search.go`, `vsearch.go`
- [x] 5.2 In each REST handler, parse the 4 fields using `timefilter.Parse(input, time.Now().UTC())`; on error return HTTP 400 with `{error: ..., param: <name>, value: <input>}`
- [x] 5.3 Add 4 optional time-range fields to MCP tool input schemas in `internal/mcp/` for `memory_query`, `memory_search`, `memory_vsearch`
- [x] 5.4 In each MCP tool handler, parse the same way; on error return MCP error result with the same payload structure
- [x] 5.5 Construct `TimeRangeFilter` from parsed values, pass to the search pipeline call
- [x] 5.6 Add 4 matching CLI flags to `nano-brain query`, `nano-brain search`, `nano-brain vsearch` in `cmd/nano-brain/commands.go`: `--created-after`, `--created-before`, `--updated-after`, `--updated-before`. Each accepts a string (RFC3339 or relative duration). Pass through to the REST request body — CLI talks to the daemon, does not call the pipeline directly. Parse errors should surface as the existing CLI error path (clean exit with descriptive stderr message)
- [x] 5.7 Update CLI help text examples in `cmd/nano-brain/commands.go` to show `nano-brain search "bug fix" --updated-after=30d`
- [x] 5.8 Verify `go build ./...` succeeds

## 6. Integration Tests

- [x] 6.1 New file `internal/server/handlers/timefilter_integration_test.go` (build tag `integration`); uses `testutil.SetupTestDB(t)`
- [x] 6.2 Test: seed 3 documents at different `updated_at` timestamps, search with `updated_after="30d"`, assert only in-window docs returned
- [x] 6.3 Test: same with `created_after` RFC3339
- [x] 6.4 Test: all four filters combined (AND semantics)
- [x] 6.5 Test: invalid duration returns HTTP 400 with parameter name in error body — no DB call
- [x] 6.6 Test: date-only string returns HTTP 400
- [x] 6.7 Test: negative duration (`-30d`) returns HTTP 400
- [x] 6.8 Test: inverted range (`updated_after` > `updated_before`) returns 200 with empty results, NOT 400
- [x] 6.9 Test: filter that matches zero documents returns 200 with empty results
- [x] 6.10 Test: paginate with filter, advance with same filter — cursor works; change time-range filter — cursor invalidated
- [x] 6.11 Test: paginate with tags, change tags between calls — cursor invalidated (regression test for pre-existing bug)
- [x] 6.12 New file `internal/mcp/timefilter_integration_test.go` mirroring 6.2–6.11 via MCP tool calls
- [x] 6.13 Verify `go test -race -tags=integration ./internal/server/handlers/...` and `./internal/mcp/...` pass

## 7. smoke:e2e Evidence (REST + CLI)

- [x] 7.1 Build binary: `go build -o ./bin/nano-brain ./cmd/nano-brain/`
- [x] 7.2 Start server on port 3199 against `nanobrain_dev` with no embedding provider
- [x] 7.3 Curl `/api/v1/query` with `updated_after="30d"` against a real workspace — verify 200, non-empty results, only docs within window
- [x] 7.4 Curl `/api/v1/search` with `created_after="2026-05-01T00:00:00Z"` — verify 200, correct filtering
- [x] 7.5 Curl `/api/v1/vsearch` with all 4 filters combined — verify 200, AND semantics
- [x] 7.6 Curl with `updated_after="banana"` — verify 400 with parameter name in error body
- [x] 7.7 Curl with `updated_after="-30d"` — verify 400 (negative-duration rejection)
- [x] 7.8 CLI smoke: `./bin/nano-brain search "test" --updated-after=30d --server-url=http://localhost:3199` — verify exit 0 with correct filtering
- [x] 7.9 CLI error path: `./bin/nano-brain search "test" --updated-after=banana --server-url=http://localhost:3199` — verify non-zero exit, descriptive stderr
- [x] 7.10 Kill server, capture full curl + CLI transcripts to `docs/evidence/issue-360-smoke-e2e.md`

## 8. Documentation

- [x] 8.1 Update `.opencode/skills/nano-brain/SKILL.md` — add "Time-range filters" subsection under each of `memory_query`, `memory_search`, `memory_vsearch` with at least one worked example each (e.g. "Find bugs fixed in last 30 days: `memory_search(query='bug fix', updated_after='30d')`")
- [x] 8.2 Document the "rough relative" caveat: `30d` = 30×24h, `1mo` = 30d, `1y` = 365d. For calendar-precise queries, pass RFC3339
- [x] 8.3 Update README.md MCP Tools table description for the 3 tools to mention time-range support (one-line additions, no separate section)

## 9. Validation Ladder

- [x] 9.1 `go build ./... && go test -race -short ./...` passes
- [x] 9.2 `go test -race -tags=integration ./...` passes
- [x] 9.3 `openspec validate add-time-range-filters --strict --no-interactive` passes
- [x] 9.4 `./scripts/harness-check.sh in-progress --json` passes
- [x] 9.5 Self-review against acceptance criteria from issue #360 (paste checklist into `docs/evidence/issue-360-self-review.md`)
- [x] 9.6 smoke:e2e evidence captured (gate 7 above)

## 10. PR + Merge

- [x] 10.1 Verify issue #360 carries lane (`high-risk`) + change-type (`user-feature`) labels
- [x] 10.2 Commit + push branch `feat/360-time-range-filters`
- [x] 10.3 Open PR — title `feat(mcp): time-range filters on search tools (#360)`, body links #360, includes EXPLAIN evidence, smoke:e2e transcript, integration test summary
- [x] 10.4 `./scripts/harness-check.sh pre-merge --json` passes (or override with documented reason)
- [x] 10.5 Await Gemini review, address verdict-based triage
- [x] 10.6 Merge → `./scripts/harness-check.sh post-merge --pr <N> --json` passes
- [x] 10.7 npm release watcher → `./scripts/harness-check.sh post-merge-npm-release --json` passes
- [x] 10.8 `openspec archive add-time-range-filters`
- [x] 10.9 `./scripts/harness-check.sh next-ready --json` passes
