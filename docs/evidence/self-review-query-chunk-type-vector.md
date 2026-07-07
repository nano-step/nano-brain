# Self-Review — Issue #571 (#542 F7: chunk_type on hybrid vector leg)

Change-type: bug-fix · Lane: tiny · Branch: `fix/query-chunk-type-vector`
Author: kokorolx.

## Actions Taken

- `memory_query` with `chunk_type:"symbol"` now actually filters. Added
  `ChunkType: chunkTypeNullStr,` to all 8 `VectorSearch*` param structs in
  `HybridSearch` (4) and `hybridSearchInner` (4) in `internal/search/service.go`.
  The BM25 legs already passed it; the vector legs dropped it, so the vector half
  of the hybrid returned raw/markdown chunks that RRF merged back in.
- Pure wiring of an existing-but-dropped param: the `ChunkType` field already
  exists on the sqlc vector param types and the vector SQL (`embeddings.sql`)
  already has the `chunk_type` narg. No schema/param change.

## Files Changed

- `internal/search/service.go` — 8× `ChunkType: chunkTypeNullStr,` on the vector
  legs (8 insertions; nothing else — an incidental gofmt realignment of an
  unrelated `var` block in `DebugSearch` was reverted to keep the diff surgical).
- `internal/search/chunk_type_vector_571_test.go` — white-box test: a capturing
  querier records the `ChunkType` each of the 4 vector legs receives; asserts all
  four paths (all/ws × tags/no-tags) get `{symbol,valid}`.

## Findings Summary

- The fix mirrors the BM25 legs exactly (same `chunkTypeNullStr`), so no
  behavior change when `chunk_type` is unset (`{Valid:false}` → SQL `IS NULL`
  branch → no filtering).
- **Red-green proven** at the service level: with the fix stashed, all 4 vector
  legs receive `{Valid:false}` (the F7 bug); with it, `{symbol,valid}`.
- F9 (`memory_search` multi-word + chunk_type → 0, a distinct OR-fallback gap) is
  tracked separately — not in this PR.

## Resolution Status

- In scope resolved. No critical/major issues.
- `go build ./...` clean; `go test -race ./internal/search/` all ok (incl. the
  new white-box test).
- smoke:e2e: `docs/evidence/smoke-e2e-query-chunk-type-vector.md` — MCP-over-HTTP
  on :3199: `chunk_type=symbol`/`raw` correctly filter `memory_query` results
  end-to-end. Dev DB never touched.

## Gemini Verification Triage

_Pending — populate after the Gemini bot reviews the PR._

| Comment ref | Agent verdict | Reasoning | Action |
| --- | --- | --- | --- |
| _(none yet)_ | | | |
