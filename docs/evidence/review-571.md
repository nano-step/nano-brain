## Review Verdict: PASS

Reviewer: oh-my-claudecode:code-reviewer (R88 independent correctness gate, spawned; ≠ author).
Date: 2026-07-07
Branch: `fix/query-chunk-type-vector` · Issue #571 (split from #542 F7)

Change: added `ChunkType: chunkTypeNullStr` to all 8 `VectorSearch*` param
structs in `HybridSearch`/`hybridSearchInner` so the vector leg honors the
`chunk_type` filter (BM25 legs already did). Lane relevance: search-quality.

| Concern | Verdict |
|---|---|
| Completeness (all 8 legs) | PASS (HIGH) — exactly 8 `VectorSearch` sites (4+4), all now carry ChunkType; no `VectorSearch*OR` variants exist; the only `*OR` is a BM25 leg (already had it). |
| Correctness / NULL semantics | PASS (HIGH) — same `chunkTypeNullStr` the BM25 legs use, in scope at each site; unset → `{Valid:false}` → SQL `IS NULL` branch → no-filter path byte-for-byte unchanged. |
| No unintended diff | PASS (HIGH) — 1 file, 8 insertions / 4 deletions (the 4 hybridSearchInner lines re-emitted with ChunkType inline); the incidental gofmt realignment of DebugSearch's var block is confirmed **reverted / not present**. |
| Test quality | PASS (HIGH) — capturing querier overrides only the 4 vector leaves, calls the REAL `HybridSearch`; mockEmbedder returns a valid vector so the vector leg genuinely runs; all 4 permutations assert `{symbol,valid}`; non-vacuous (red without fix / green with). |
| Recall / RRF / degrade | PASS (HIGH) — both legs now filter identically before RRF (no adverse merge); embed-failure degrade still yields BM25-filtered results. Recall change is the intended fix. |

Reviewer independently ran `go build ./...` (exit 0) + the targeted test (4/4
PASS). Author confirmed full `go test -race -short ./...` green. **0 findings at
any severity.**

### smoke:e2e — PASS
`docs/evidence/smoke-e2e-query-chunk-type-vector.md` — service-level red-green
(all 4 vector legs) + MCP-over-HTTP on :3199: `chunk_type=symbol`/`raw` correctly
filter `memory_query` results end-to-end. Dev DB never touched.

### Note
F9 (`memory_search` multi-word + chunk_type → 0, a distinct OR-fallback gap) is
tracked separately — not in this PR.
