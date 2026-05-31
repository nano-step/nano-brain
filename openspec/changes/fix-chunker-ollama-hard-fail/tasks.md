# Tasks: Embedder Hard-Fail on 400

Tracking: #260

## Phase A — Implement

- [ ] **A1** Add `isHardFailureEmbedError(err error) bool` helper to `internal/embed/queue.go`. Returns true for substrings matching:
  - `"unexpected status 400"` (ollama + voyageai both use this)
  - `"unexpected status 401"`
  - `"unexpected status 403"`
  - `"unexpected status 413"`
  - `"unexpected status 422"`
  All other patterns → false.

- [ ] **A2** In `processChunk()`, after `q.embedder.Embed()` error branch, classify before retry:
  ```go
  if err != nil {
      q.logger.Error().Err(err).Str("chunk_id", chunkID.String()).Str("file", chunk.SourcePath).Msg("embedding failed")
      if isHardFailureEmbedError(err) {
          if mErr := q.queries.MarkChunkEmbedFailed(ctx, sqlc.MarkChunkEmbedFailedParams{
              ID: chunkID, WorkspaceHash: chunk.WorkspaceHash, Error: sql.NullString{String: err.Error(), Valid: true},
          }); mErr != nil {
              q.logger.Error().Err(mErr).Str("chunk_id", chunkID.String()).Msg("mark embed_failed (hard) failed")
          }
          q.pending.Add(-1)
          q.clearRetries(chunkID)
          return
      }
      q.increaseBackoff()
      q.handleRetry(ctx, chunkID, chunk.WorkspaceHash)
      return
  }
  ```
  (Verify exact `MarkChunkEmbedFailedParams` shape from sqlc generated code — adjust if different.)

- [ ] **A3** Run `go build ./...` and `go vet ./...` — both clean.

## Phase B — Tests

- [ ] **B1** `TestIsHardFailureEmbedError` — table-driven, 10+ cases covering all 5 status codes (positive), 3 transient codes (500/502/503 → false), bare errors (false).

- [ ] **B2** `TestProcessChunk_HardFailOn400` — use mockEmbedder returning `fmt.Errorf("ollama: unexpected status 400: %s", "...")`. Assert:
  - `MarkChunkEmbedFailed` called 1×
  - `increaseBackoff` NOT called (verify via `q.backoff` unchanged)
  - `pending` decremented to 0
  - retry map cleared (seed `q.retries[chunkID] = 1` first, assert it's gone)

- [ ] **B3** `TestProcessChunk_TransientErrorRetries` — mock returns `fmt.Errorf("connection refused")`. Assert:
  - `increaseBackoff` called (verify `q.backoff` > 0)
  - `MarkChunkEmbedFailed` NOT called
  - retry map populated

- [ ] **B4** Run `go test -race -short ./internal/embed/... -v` → all PASS including 3 new tests.

## Phase C — Validate ladder

- [ ] **C1** `validate:quick`: `go build ./... && go test -race -short ./...` → green.

- [ ] **C2** `self-review:staged-files`: `git status` clean before commit.

## Phase D — PR + merge

- [ ] **D1** Commit:
  ```
  fix(embed-queue): mark chunk embed_failed on hard provider errors (#260)

  Embed worker previously retried HTTP 400 errors every minute forever, trapping
  chunks in the queue (~374 occurrences in production logs). Fix: classify
  embed errors into hard-failures (400/401/403/413/422) vs transient. On hard
  failure, MarkChunkEmbedFailed + clearRetries + decrement pending — no retry.
  Transient errors continue using existing handleRetry path.

  Tests:
  - TestIsHardFailureEmbedError (10+ cases for the classifier)
  - TestProcessChunk_HardFailOn400 (asserts MarkChunkEmbedFailed + no retry)
  - TestProcessChunk_TransientErrorRetries (asserts retry path preserved)
  ```

- [ ] **D2** Push, open PR with `Closes #260`, await Gemini, triage per R31 if needed.

- [ ] **D3** Squash merge.

- [ ] **D4** Close issue, archive openspec, cleanup worktree + branch.
