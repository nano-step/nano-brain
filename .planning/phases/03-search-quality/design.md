# Phase 3 — Design: memory_vsearch under-retrieval on multi-concept queries

Tracking: #545 (PR-C)

## Root cause

`registerMemoryVSearch` (`internal/mcp/tools.go:844`, formerly at line ~844
before the fix) computed `fetchLimit := int32(offset + maxResults + 1)`
(formerly line ~953) and fetched that many **chunks** via
`VectorSearch`/`VectorSearchAll` (`internal/storage/queries/embeddings.sql:68`
— `ORDER BY embedding <=> query LIMIT max_results`, no similarity
threshold). When `group_by == "document"` (the default), those chunks were
then collapsed to documents by `deduplicateByDocument` (`tools.go:293`).

When the top-N chunks by cosine distance cluster into a handful of
documents — e.g. a diluted compound-query embedding scores several chunks
from the same one or two documents higher than any chunk from other
documents — dedup collapses the page to far fewer than `max_results`
distinct documents. #545's repro: `max_results:7` returned `total:2`.

There is no similarity threshold anywhere in this path; the bug is purely
"fetch too few chunks, then dedup," not a relevance/ranking defect.

**Also found and fixed (G-C2):** the handler had a duplicated
`if groupBy == "document" { ... deduplicateByDocument ... }` block — the
same dedup logic appeared twice back-to-back (copy-paste), each rebuilding
`allRows` from the previous result. Functionally harmless (idempotent) but
dead weight; removed, one block remains.

## Decisions (Gate 1.7 style, single-pass — no PR/design review loop for a
tiny bugfix; recorded per orchestrator directive)

**G-C1 — over-fetch chunks before dedup, no similarity threshold.**
When `group_by == "document"`, fetch
`min((offset+max_results+1) * vsearchDedupOverFetchFactor, vsearchDedupOverFetchCap)`
chunks instead of `offset+max_results+1`. Named consts in `tools.go`:

```go
const (
	vsearchDedupOverFetchFactor = 5
	vsearchDedupOverFetchCap    = 200
)
```

- **Why a multiplicative factor, not a threshold:** a similarity cutoff
  requires picking a magic number that's wrong for some embedding
  models/query shapes; over-fetching more *candidates* for the existing
  exact dedup logic fixes the actual defect (too few candidates) without
  guessing at "how similar is similar enough."
- **Why factor=5, cap=200:** 5x covers the reported failure mode (2
  documents dominating the top 8 chunks) with room to spare — even a
  document contributing 4-5 near-duplicate chunks won't starve the other
  ~(5x-5) fetched slots. The absolute cap (200) bounds worst-case query cost
  on workspaces with very large `max_results` requests; 200 chunks is still
  a cheap HNSW-indexed fetch (see latency note below).
- **When `group_by != "document"`:** each chunk is already its own result
  (no collapsing happens), so the over-fetch buys nothing — fetch stays at
  `offset + maxResults + 1`, unchanged from today. Verified by
  `TestMemoryVSearch_GroupByNone_FetchLimitUnaffected`.
- **Cap has a floor at `baseFetchLimit` (R88 review #545 follow-up).** The
  200-cap is a ceiling on the over-fetch *boost* only — it must never reduce
  the fetch below what the page window itself needs. At deep offsets
  (`offset+max_results+1 > 200`), a bare `min(base*factor, cap)` picks a
  fetchLimit smaller than `baseFetchLimit`, starving the page and reporting
  `has_more=false` when more documents exist. Fixed to
  `max(baseFetchLimit, min(base*factor, cap))`. Verified by
  `TestMemoryVSearch_OverFetchCapFloor_DeepPagination`.
- **No similarity threshold (explicitly rejected).** Out of scope per
  intake — would change *what counts as a match*, not just *how many
  candidates dedup gets to work with*. Conflates two different fixes.
- **No "under-retrieval signal" field on the response.** YAGNI — the fix
  makes under-retrieval not happen (for the fetch-then-dedup shape); a
  signal field is a workaround for a problem that no longer needs working
  around.

**G-C2 — remove duplicated dedup block.** Done; one
`if groupBy == "document" { ... }` block remains at `tools.go` (post-fix,
~line 1052).

**G-C3 — memory_query parity: do NOT apply the same over-fetch.**
`registerMemoryQuery` (`tools.go:305`) has a superficially similar shape —
compute `fetchLimit := offset + maxResults + 1` (line 404), pass it to
`HybridSearch`, then `deduplicateByDocument` the returned results
(line 421-423) — but the *path underneath* is materially different from
vsearch's plain `VectorSearch` LIMIT query, so the same multiplier is not
"low-cost" there:

1. `HybridSearch` (`internal/search/service.go:164`) does not use the
   caller's `fetchLimit` as a DB `LIMIT` directly. It derives its own
   internal leg-fetch limit — `int32(maxResults * 3)`, floor 30
   (`service.go:196-199`) — for the BM25 and vector legs, RRF-merges
   (`DynamicRRFMerge`), dedups exact-ID duplicates
   (`DeduplicateResults`), and applies code-aware/extension/recency/
   entity/pagerank boosts before an **optional external reranking API
   call** (`internal/search/reranking/reranker.go:64`,
   `apiReranker.Rerank`) and only then truncates to the caller's
   `maxResults` (`service.go:625-627`) — *before* returning to
   `registerMemoryQuery`, which then dedups by document.
2. The reranker, when enabled (`config.RerankingConfig.Enabled`), sends
   **every** candidate in the pre-truncation `boosted` slice to an external
   API as document text (`reranker.go:76-86`,
   `docTexts := make([]string, len(docs))` — no internal truncation before
   building the request payload). Increasing the `maxResults` passed into
   `HybridSearch` fans out through the x3 leg multiplier into a
   proportionally larger `boosted` slice, which — with reranking enabled —
   means proportionally larger external API payloads, latency, and cost.
   That is not "low-cost"; it's a real, config-dependent latency/cost
   regression on the codebase's *default first tool*.
3. Contrast with vsearch: `VectorSearch`'s `fetchLimit` **is** the literal
   SQL `LIMIT`, an indexed HNSW fetch with no external call downstream — a
   5x-200 over-fetch there is genuinely cheap.
4. The `DebugSearch` path (`memory_query mode:"debugging"` at `tools.go:408`
   and `registerMemorySearch`'s debug branch ~`tools.go:611`, both →
   `service.go:642`) fans out through the hybrid legs, same family as (1) —
   not a plain `VectorSearch` LIMIT — so this exclusion covers it too. #543's
   debugging-mode buckets (PR-D) is a separate concern and does not change
   retrieval depth.

**Decision: leave `registerMemoryQuery` (and the `DebugSearch` paths) untouched.** The task's own escape
clause ("if it materially changes hybrid-search latency/behavior or the
path differs, do NOT touch it") applies directly. Filing this as a
follow-up: if #545-shaped under-retrieval is later reported against
`memory_query` specifically, the fix should be scoped to the case
`reranking.Enabled == false` (or given its own, smaller-multiplier
constant), not a blind copy of vsearch's factor/cap.

**G-C4 — no signal field.** Covered under G-C1; restated for parity with
the PR-B design doc's numbering convention.

## Latency note

An HNSW-indexed `ORDER BY embedding <=> query LIMIT N` fetch of 5x the
previous row count (bounded at 200 total rows) is not a meaingfully
different query shape — same index, same predicate, larger `LIMIT`. No new
joins, no new sequential scans. This is the reason G-C1 is safe to ship
without a benchmark: the cost model is "fetch a few hundred more small
rows from an index," not "add a new query."

## Testing approach

Integration test (`-tags=integration`, `testutil.SetupTestDB`, runs against
`nanobrain_test`), new file
`internal/mcp/vsearch_overfetch_545_integration_test.go`:

- A fixed-vector embedder (`vsearchFixedEmbedder`) pins the query embedding
  to `[1, 0, ..., 0]`. Chunk embeddings are constructed as
  `[alpha, sqrt(1-alpha^2), 0, ...]` so cosine similarity to the query is
  exactly `alpha` — deterministic ranking without depending on a real
  embedding provider (matches the pattern already used in
  `internal/search/isolation_test.go`'s `fakeEmbedder`/`makeVec`).
- **Collapse repro** (`TestMemoryVSearch_OverFetch_FewDocumentsCollapse`):
  2 "hot" documents x 5 near-identical high-similarity chunks each (alpha
  0.99/0.98) + 10 "cold" documents x 1 lower-similarity chunk each (alpha
  0.50 down to 0.41). Pre-fix, `max_results=7` would fetch only the top 8
  chunks — all from the 2 hot documents — collapsing to `total=2`. Post-fix
  it fetches all 20 chunks, dedups to `total=12`, and the page fills to the
  requested 7 distinct documents.
- **No-regression control**
  (`TestMemoryVSearch_ManyDocumentsAlreadyDiverse`): 10 documents x 1 chunk
  each at distinct similarities — dedup was never the bottleneck here;
  confirms the over-fetch doesn't change the already-correct case.
- **Scope control**
  (`TestMemoryVSearch_GroupByNone_FetchLimitUnaffected`): same hot/cold
  seed data, `group_by="none"` — asserts `total=8`
  (`offset+max_results+1`), proving the over-fetch is applied only when
  `group_by == "document"`.

No unit test was added for `memory_query` (G-C3: not modified).
