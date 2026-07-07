# Self-Review — Issue #573 (#542 F9: memory_search multi-word OR-fallback)

Change-type: bug-fix · Lane: tiny · Branch: `fix/search-multiword-or-fallback`
Author: kokorolx.

## Actions Taken

- `memory_search` with a multi-word phrase + `chunk_type` no longer silently
  returns 0. Its direct BM25 path uses `websearch_to_tsquery` (ANDs terms); a
  symbol chunk rarely contains every word, so under a chunk_type filter it
  collapsed to 0. Added the same OR-relaxation `HybridSearch` already uses.
- `internal/search/service.go`: exported `BuildORQuery` wrapping the existing
  unexported `buildORQuery` (stopword-filter + strip + join with " | ") so the
  MCP handler reuses the identical logic (incl. the shared stopword list).
- `internal/mcp/tools.go` (memory_search): after the AND BM25 populates
  `allRows`, if `len(allRows)==0 && len(tags)==0 && len(Fields(query))>=2`,
  rebuild an OR query and reissue via `BM25SearchAllOR` (ws=="all") /
  `BM25SearchOR`, passing the same `chunkTypeNull` so OR results stay filtered.

## Files Changed

- `internal/search/service.go` — `BuildORQuery` exported wrapper (+5).
- `internal/mcp/tools.go` — OR-fallback block in the memory_search handler (+44).
- `internal/mcp/search_multiword_573_integration_test.go` — e2e through the
  handler: seeds a symbol chunk, populates search_vector, searches a multi-word
  phrase whose AND misses; asserts the OR fallback rescues it.

## Findings Summary

- Scope: untagged only — there is no `BM25Search*WithTagsOR` SQL variant, so a
  tagged multi-word fallback is a follow-up (would need new queries). The
  reported repro is untagged.
- **Time-filter safety** (R88 MEDIUM, addressed): the OR SQL variants have no
  time-range params, so the fallback is additionally gated on `timeRange == nil`
  — with a time filter set it returns 0 (as before) rather than risk surfacing
  rows outside the requested window. (This is stricter than HybridSearch's
  fallback, which omits the guard.)
- **Red-green proven**: with the fallback stashed, the multi-word search returns
  `results:[] total:0` (the F9 bug); with it, the symbol chunk is returned.
- No regression: when the AND query returns ≥1 row, or the query has <2 words,
  or tags are present, the fallback block is skipped entirely. `BuildORQuery` is
  a pure passthrough — HybridSearch's existing callers are unaffected. OR-query
  failure returns an errResult (not swallowed).

## Resolution Status

- In scope resolved. No critical/major issues.
- `go build ./...` clean; `go test -race -short ./...` all ok.
- Integration (nanobrain_test): OR-fallback e2e test PASS.
- smoke:e2e: `docs/evidence/smoke-e2e-search-multiword-or-fallback.md` (MCP-over-HTTP
  on :3199 — multi-word AND-miss rescued by OR fallback). Dev DB never touched.

## Gemini Verification Triage

Gemini: COMMENTED, CI pass, MERGEABLE/CLEAN. One inline (MEDIUM).

| Comment ref | Agent verdict | Reasoning | Action |
| --- | --- | --- | --- |
| tools.go:804 [medium] — a stopword query (`"the deposit"`) passes `Fields>=2` but `BuildORQuery` yields a single term → the OR query equals the already-failed AND query (redundant round-trip) | VALID | The `" | "` check is stricter and more correct — it subsumes `Fields>=2` and skips the redundant query when <2 non-stopword terms survive. | **Fixed** — gate changed from `len(Fields(query))>=2` + `orQuery != ""` to `strings.Contains(orQuery, " | ")`. |
