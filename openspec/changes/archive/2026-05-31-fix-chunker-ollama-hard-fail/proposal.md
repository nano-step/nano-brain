# Fix Embedder Hard-Fail on 400 (No Infinite Retry)

## Issue
[#260 — fix(chunk): chunker produces chunks larger than embedder context window (ollama 400)](https://github.com/nano-step/nano-brain/issues/260)

## Lane
normal (2 risk flags: existing-behavior + weak-proof).

## Why
The embed worker retries chunks that fail with HTTP 400 (`input length exceeds the context length`) every minute forever, never making progress. Multiple files in production logs hit this same chunk repeatedly (same `chunk_id` every minute). Wastes ollama API quota, traps a chunk slot in the queue (contributes to backpressure 9999/10000), floods logs with `embedding failed` ERROR entries — 374+ occurrences observed.

Existing safeguard `truncateToMaxChars(chunk.Content, 4000)` is insufficient: CJK-dense content can produce 5500-8000 tokens from 4000 characters, exceeding nomic-embed-text's 8192 token limit. Even after truncation, some chunks still 400.

Root cause: `processChunk()` treats ALL embed errors uniformly — `q.increaseBackoff()` + `q.handleRetry()` retries every error until max retries, including HTTP 400 which is a hard client error that will never resolve through retries.

## Desired Outcome
Embed worker distinguishes **transient errors** (retry) from **hard failures** (mark `embed_failed`, no retry):

- **Transient**: network timeout, connection refused, 5xx server error, embedding provider transiently unavailable
- **Hard**: HTTP 400 (request invalid), 401/403 (auth), 413 (request too large), 422 (semantic error)

On hard failure, the chunk is marked `embed_failed` immediately in the DB with the error message captured. Worker pending counter decrements; retry counter cleared. Operator sees the failure in DB (queryable for diagnostics) but the queue is no longer stuck on it.

## Constraints
- Single-file change to `internal/embed/queue.go` + new helper to classify errors
- No new config keys (defensive defaults baked in)
- No schema migration (use existing `MarkChunkEmbedFailed`)
- Both ollama and voyageai providers emit errors with parseable status codes — both must be handled
- Truncation behavior unchanged (still happens before HTTP call)
- Existing `handleRetry()` path preserved for transient errors

## Out of Scope
- Pre-flight token-counting before embed call (would need tokenizer dependency; future improvement)
- Smart re-chunking on size overflow (would require chunker refactor)
- Provider-specific retry policies (deferred — uniform classification across providers)
- Issue #259 cleanup (already handled in separate PR)

## Acceptance Criteria
1. **HTTP 4xx is hard-fail**: On embed error matching status codes 400/401/403/413/422, the worker:
   - Logs ERROR with the full provider response
   - Calls `MarkChunkEmbedFailed` immediately (DB persists the failure state + message)
   - Decrements pending counter
   - Calls `clearRetries(chunkID)` to free retry map slot
   - Does NOT call `increaseBackoff()` or `handleRetry()`
2. **Other errors still retry**: Network errors, 5xx, timeouts continue to use existing retry+backoff path. No regression.
3. **Same chunk doesn't reappear**: Once a chunk is marked `embed_failed` via this path, the periodic rescanner does NOT re-enqueue it (uses existing `embed_status` filter).
4. **Tests**:
   - `TestProcessChunk_HardFailOn400` — mock embedder returns "unexpected status 400" error → assert `MarkChunkEmbedFailed` called, `increaseBackoff` NOT called, `pending == 0`, no retry map entry remains
   - `TestProcessChunk_TransientErrorRetries` — mock embedder returns generic "connection refused" → assert `handleRetry` called, `increaseBackoff` called, `MarkChunkEmbedFailed` NOT called
   - `TestIsHardFailureError` — table-driven test for the classifier: ollama 400/401/403/413/422 → true; ollama 500/502/503 → false; voyageai equivalents → true; bare error string → false
5. **No regression**: existing `internal/embed/...` tests pass unchanged. `go test -race -short ./...` green.
6. **Validate ladder**: `validate:quick` PASS.

## Risk Flags
- [x] Existing behavior (chunks that 400 used to retry indefinitely, now stop after 1 attempt)
- [x] Weak proof (no current test for 4xx classification)

2 flags + 0 hard gates → **normal lane** confirmed.

## Operator impact
Existing `embedding_failed` chunks remain `embed_failed` (no schema/data change). New 4xx failures will be marked failed faster — operator now sees a stable count of failed chunks in `chunks.embed_status = 'failed'` rather than a stuck queue with cyclic retries.

To re-attempt a previously-failed chunk after fixing root cause (e.g., re-chunking): manually update `chunks.embed_status = 'pending'` for the chunk_id, or delete the chunk row and let re-indexing recreate it.
