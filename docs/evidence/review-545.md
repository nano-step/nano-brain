## Review Verdict: PASS

Reviewer: oh-my-claudecode:code-reviewer (R88 independent correctness gate, spawned; ≠ author — implemented by separate Sonnet executors).
Date: 2026-07-07
Commit: 03a7f12 (branch `fix/vsearch-overfetch-dedup`)

Change: `memory_vsearch` over-fetches chunks (factor 5, cap 200, floored at the
page window) before `group_by=document` dedup so multi-concept queries return
enough distinct documents (#545). Also removed a duplicated dedup block.

### Correctness gate (R88) — FAIL → fixed → PASS

- **Round 1 (commit edc6cf0): FAIL [HIGH].** Cap had no floor: when
  `baseFetchLimit = offset+maxResults+1 > 200` (reachable — `max_results` caps at
  100, so page-2 with offset=100 → 201), `min(base*5, 200)` picked 200 < 201,
  fetching FEWER chunks than the page window → silent page truncation + wrong
  `has_more` at deep pagination (a regression vs pre-fix `fetchLimit==base`).
- **Fix (commit 03a7f12):** `fetchLimit = int32(max(baseFetchLimit, min(base*factor, cap)))`
  — the cap bounds the over-fetch *boost* only, never reduces below the page
  window. New test `TestMemoryVSearch_OverFetchCapFloor_DeepPagination`.
- **Round 2: PASS / APPROVE** @ 03a7f12. Reviewer confirmed the floor fully
  resolves the HIGH finding and introduces no new issue (`max()` only raises to
  the caller's own page window; `min(...,cap)` still bounds the boost).

### Quality axes — independent teammate reviews (all clean, no blocking)

| Axis | Reviewer | Verdict |
|---|---|---|
| Efficiency | simplify-efficiency | Clean — over-fetch → SQL LIMIT (not post-filter), 200 cap bounds worst case, duplicate-dedup removal is a real win |
| Simplification | simplify-simplification | Clean — consts/vars load-bearing; one pre-existing out-of-scope note (vsRow↔search.Result round-trip) |
| Reuse | simplify-reuse | Clean — helpers reused from existing tests; pre-existing repo-wide `sha256` hash-helper duplication noted as optional follow-up |
| Altitude | simplify-altitude | Right altitude — over-fetch at call site not inside pure dedup; consts not config; flagged DebugSearch doc gap (fixed in design.md) |

### Validation (run by orchestrator)

- `go test -race -tags=integration ./internal/mcp/ -run 'VSearch|Overfetch|545|Floor|Pagination'` → 6 PASS.
- `go test -race -short ./...` → 31 pkgs ok, 0 FAIL. `go build ./...` exit 0.
- smoke:e2e — `docs/evidence/smoke-e2e-vsearch-overfetch-dedup.md` (REST vsearch HTTP 200 with live ollama).
- All against `nanobrain_test` — never dev DB / :3100.

### Out-of-scope follow-ups (noted, not blocking)

- Dedup `vsRow↔search.Result` round-trip simplification (pre-existing).
- Shared `testutil.HashContent()` for the repo-wide `sha256` test-hash pattern.
- `memory_query`/`DebugSearch` intentionally NOT over-fetched (G-C3: hybrid legs + reranker fan-out) — a scoped follow-up only if #545-shaped under-retrieval is reported there.
