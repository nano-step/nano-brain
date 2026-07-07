# Self-Review — Issue #545 (memory_vsearch over-fetch, Phase 3 PR-C)

## Actions Taken

- `memory_vsearch` (`internal/mcp/tools.go` `registerMemoryVSearch`): when
  `group_by == "document"`, over-fetch chunks before dedup —
  `fetchLimit = max(baseFetchLimit, min(baseFetchLimit*5, 200))` — so
  document-dedup can yield up to `max_results` distinct docs. Named consts
  `vsearchDedupOverFetchFactor`/`vsearchDedupOverFetchCap`.
- Removed a duplicated `if groupBy=="document" {…deduplicateByDocument…}` block
  (copy-paste, byte-identical) — one block remains.
- R88 follow-up: floored the cap at `baseFetchLimit` so deep pagination
  (`offset+max_results+1 > 200`) is never starved.
- `group_by != "document"` path unchanged (each chunk is its own result).
- `memory_query`/`DebugSearch` deliberately NOT changed (hybrid legs + reranker
  fan-out — see design.md G-C3).

## Files Changed

- `internal/mcp/tools.go`: over-fetch + floor + duplicate-block removal.
- `internal/mcp/vsearch_overfetch_545_integration_test.go` (new): 4 tests.
- `.planning/phases/03-search-quality/design.md` (new): G-C1..G-C4 decisions.

## Findings Summary

- Root cause: fetch `offset+maxResults+1` CHUNKS then dedup-to-document → few
  results when top chunks cluster into few docs (no similarity threshold exists;
  purely fetch-too-few). Fixed by over-fetching candidates.
- R88 caught a HIGH cap-starvation regression at deep pagination — fixed with a floor.
- 4 quality-axis teammate reviews (efficiency/simplification/reuse/altitude): clean.

## Resolution Status

- All findings resolved. R88: FAIL (cap-floor) → fixed → PASS (`docs/evidence/review-545.md`).
- 6 integration tests PASS; `go test -race -short ./...` 31 pkgs ok; build exit 0.
- smoke:e2e PASS (`docs/evidence/smoke-e2e-vsearch-overfetch-dedup.md`).
- No unresolved critical/major. Out-of-scope follow-ups noted in review-545.md.

## Gemini Verification Triage

_Pending — populate after the Gemini bot reviews the PR (one row per inline
comment; verdict vocabulary per HARNESS.md § PR + Bot Review Loop)._

| Comment ref | Agent verdict | Reasoning | Action |
| --- | --- | --- | --- |
| _(none yet)_ | | | |
