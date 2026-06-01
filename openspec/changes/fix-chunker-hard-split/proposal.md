Tracking: #297

## Why

`internal/chunk/chunk.go` is **line-aware but not char-aware**. `findSplitPoints` inserts split points only at line boundaries. If a single line exceeds `Config.TargetSize` (3600 chars default), the chunk containing that line cannot be shrunk by the chunker — it is emitted at the line's full length. The embed queue then truncates the oversized chunk to `embedding.max_chars` and logs `chunk truncated before embedding (exceeds context limit)`.

Production observation today: raising the embed limit (3000 → 4000 → higher) does NOT eliminate the warning. There is no finite ceiling that silences it because adversarial input (minified files, single-line JSON, harvested LLM messages with no internal `\n`, content trapped inside unclosed code fences) can always exceed any threshold. Truncation is silent data loss; the embedder receives partial content, the vector index drifts from the source.

## What Changes

Add a hard-split safety net at the end of `chunk.Split`:

1. After `buildChunks` produces line-aware chunks, scan each chunk.
2. If `len(content) <= TargetSize + searchWindow/2` (4000 by default) → keep as-is.
3. Otherwise → force-split into pieces of size `<= TargetSize` at the best available char-level boundary, with priority:
   - Blank line (`\n\n`)
   - Single newline (`\n`)
   - Sentence terminator (`. `, `! `, `? `, `。`)
   - Whitespace (` `, `\t`)
   - Hard rune-boundary cut (UTF-8 safe — never cut mid-multibyte rune)

This preserves existing line-aware behavior for normal content while guaranteeing every emitted chunk is `<= TargetSize + searchWindow/2`. The embed queue's truncation warning becomes a true safety net that fires only on configuration drift, not on routine input.

## Capabilities

### Modified Capabilities
- `chunker`: existing capability — adds a contract that no chunk emitted by `chunk.Split` exceeds `TargetSize + searchWindow/2` chars.

### New Capabilities
None.

## Impact

- **Code:** `internal/chunk/chunk.go` (~80 new lines for `hardSplit` + helpers), `internal/chunk/chunk_test.go` (~120 lines of new tests).
- **Behavior:** Oversized chunks (single long lines, fence-trapped content) now split correctly at sub-line boundaries. No silent data loss in the embed pipeline. Number of chunks per document may increase slightly for adversarial input; unchanged for normal markdown/code.
- **Risk:** Low — post-process step. Existing `findSplitPoints` and `buildChunks` are untouched. New code path only triggers on chunks that would otherwise have been truncated downstream.
- **Performance:** O(n) extra scan over chunk content; only runs on chunks already known to exceed the threshold. Negligible.

## Out of Scope

- Replacing line-aware logic with AST/tree-sitter chunking — future work.
- Token-true counting (currently bytes via `len()`; real BPE tokenization is a separate issue).
- Surfacing oversized-chunk count as a `/api/v1/status` metric — separate observability change.
- Changing `embedding.max_chars` default — independent config.
