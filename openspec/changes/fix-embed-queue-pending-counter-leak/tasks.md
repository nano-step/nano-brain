## 1. Code fix: handleRetry channel-full (Mode 6)

- [x] 1.1 Remove `q.pending.Add(-1)` from the `select default` branch in `handleRetry` (queue.go:361-367)
- [x] 1.2 Keep the existing WARN log line; update message to `"retry re-enqueue failed (channel full)"` if needed for clarity
- [x] 1.3 Add doc comment on `pending` field stating the invariant: `pending == COUNT(chunks WHERE embed_status='pending')`

## 2. Code fix: MarkChunkEmbedded failure (Mode 2)

- [x] 2.1 Remove `q.pending.Add(-1)` from the MarkChunkEmbedded error branch (queue.go:303-310)
- [x] 2.2 Add `q.publishStatus()` call after the ERROR log in the same branch
- [x] 2.3 Verify InsertEmbedding has `ON CONFLICT` clause in queries/embeddings.sql; if not, add it

## 3. Tests

- [x] 3.1 Add `TestQueue_RetryRequeueChannelFull` to `internal/embed/queue_test.go`: create queue with channel cap 1, fill it, trigger transient error on a chunk, assert pending unchanged + status='pending' + WARN logged
- [x] 3.2 Add `TestQueue_MarkChunkEmbeddedFailure` to `internal/embed/queue_test.go`: mock MarkChunkEmbedded to fail after successful InsertEmbedding, assert pending unchanged + status='pending' + embedding row exists + publishStatus called
- [x] 3.3 Add `TestQueue_InvariantPendingMatchesDB` to `internal/embed/queue_test.go`: table-driven test covering 7 scenarios (success, transient error, mark failure, hard fail) asserting pending delta matches expected DB state per-chunk

## 4. Verification

- [x] 4.1 Run `go test -race -short ./internal/embed/...` → all PASS (51 pass, 1 skip)
- [x] 4.2 Run `go build ./...` → exit 0
- [x] 4.3 Run `go vet ./...` → no warnings
- [x] 4.4 Run full validate:quick: `go build ./... && go test -race -short ./...` → exit 0

## 5. PR + Review

- [ ] 5.1 Commit with conventional message: `fix(embed-queue): preserve pending counter invariant on retry channel-full + mark-embedded failure (#272)`
- [ ] 5.2 Push branch `feat/272-pending-counter-leak` to origin
- [ ] 5.3 Open PR linking #272, label `change-type:bug-fix`, `lane:normal`, `status:in-review`
- [ ] 5.4 Wait for Gemini Code Review bot; create `docs/evidence/fix-embed-queue-pending-counter-leak/gemini-triage.md` per R31
- [ ] 5.5 Address Gemini findings (≤3 push cycles per R31)
- [ ] 5.6 Squash merge with `--delete-branch` after CI green
- [ ] 5.7 Close issue #272 with merge comment

## 6. Archive + Release

- [ ] 6.1 Pull merged b-main locally
- [ ] 6.2 `openspec archive fix-embed-queue-pending-counter-leak --yes`
- [ ] 6.3 Commit archive, push to b-main
- [ ] 6.4 Tag next `v2026.5.31NN` on merge SHA
- [ ] 6.5 Watch Release workflow; verify npm publish on both `@nano-step/nano-brain` and `nano-brain` packages
- [ ] 6.6 Remove worktree, delete local branch
- [ ] 6.7 Switch gh auth back to `nus-rick`
