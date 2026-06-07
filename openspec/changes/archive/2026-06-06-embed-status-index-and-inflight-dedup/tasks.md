# Tasks — Embed Status Index + In-Flight Dedup (#322)

## Pre-implementation
- [ ] Run deep-design with Metis + Oracle on `proposal.md` + `design.md`. Revise until clean pass.
- [ ] Verify migration number 00014 is the next available (`ls migrations/` after rebase).
- [ ] Verify goose `-- +goose NO TRANSACTION` annotation is supported (grep migrations/ for prior use).

## Implementation

### Migration (Change A)
- [ ] Create `migrations/00014_add_chunks_embed_status_index.sql` with `Up` + `Down` blocks using `CREATE INDEX CONCURRENTLY` + `-- +goose NO TRANSACTION`.
- [ ] No sqlc regeneration needed (index doesn't change queries).
- [ ] Verify locally: `nano-brain db:migrate` succeeds on a dev PG with sample data.
- [ ] Capture `EXPLAIN ANALYZE` output for `GetPendingChunksAllWorkspaces` before + after migration. Paste in PR description.

### Queue dedup (Change B)
- [ ] Add `inflight sync.Map` field to `Queue` struct after `pending atomic.Int64` (`internal/embed/queue.go`).
- [ ] Modify `Enqueue(chunkID uuid.UUID) bool` to `LoadOrStore` the chunk ID; skip and return `false` if already loaded; delete on channel-full default branch.
- [ ] **Change `handleRetry` signature to return `bool`** (true = re-enqueued into channel, false = chunk leaving the hot path). Per D12.
- [ ] **Replace `defer q.pending.Add(-1)` in `processChunk` with conditional defer** that calls `q.inflight.Delete(chunkID)` only when `requeued == false`. Per D12.
- [ ] **Capture `requeued` flag from handleRetry call site** in processChunk's soft-failure path (line ~292): `requeued = q.handleRetry(ctx, chunkID, chunk.WorkspaceHash)`.
- [ ] No changes to scanByStatus, processChunk body otherwise, or any other method.

### Tests
- [ ] Unit: `TestQueue_Enqueue_DedupSameID` (double Enqueue → 1 channel send; first returns `true`, second returns `false`).
- [ ] Unit: `TestQueue_Enqueue_DifferentIDsBothEnqueued` (regression — both return `true`).
- [ ] Unit: `TestQueue_ProcessChunk_PanicCleansInflight`.
- [ ] Unit: `TestQueue_ChannelFull_DeletesInflight` (no leak in default branch; Enqueue returns `false`).
- [ ] Unit: `TestQueue_Enqueue_AfterProcessChunkDone_AllowsReEnqueue`.
- [ ] Unit: `TestQueue_BackpressureRejects_DoesNotAddToInflight` (Enqueue returns `false`, inflight Load returns absent).
- [ ] **Unit (D12 BLOCKER coverage): `TestQueue_HandleRetry_KeepsInflightOnSuccessfulReenqueue`** — call processChunk with embedder that returns soft error, retry count below max, channel has space → assert `inflight.Load(chunkID)` returns true after processChunk returns AND channel contains chunkID.
- [ ] **Unit: `TestQueue_HandleRetry_DeletesInflightOnChannelFull`** — same setup but channel pre-filled → assert handleRetry returns false, inflight does NOT contain chunkID, pending counter decremented.
- [ ] **Unit: `TestQueue_HandleRetry_DeletesInflightOnMaxRetries`** — pre-populate retries map to maxRetries-1 → run processChunk with soft error → handleRetry increments to maxRetries, calls MarkChunkEmbedFailed, returns false → assert inflight cleaned.
- [ ] Integration (build tag `integration`): `TestQueue_ScanByStatus_SkipsInflightChunks` — uses `testutil.SetupTestDB(t)`.
- [ ] Integration: `TestMigration_EmbedStatusIndex_Exists` — after running migrations, query `pg_indexes` to assert `idx_chunks_embed_status` exists with expected definition (replaces manual EXPLAIN-only check).

### Docs
- [ ] Update `CHANGELOG.md` `[Unreleased] ### Performance` entry.
- [ ] No README change (no new API/CLI/MCP surface).
- [ ] `internal/embed/AGENTS.md`: add 2-line note about `inflight sync.Map` (matches "Cross-Cutting Conventions" style).

### Self-review evidence
- [ ] `docs/evidence/322-self-review.md` with:
  - `git status --short` output (clean except staged files)
  - `validate:quick` output (paste full)
  - `test:integration` output for `internal/embed/...` only
  - `EXPLAIN ANALYZE` before/after migration
  - smoke:e2e SKIP justification per D11 in design.md

## Validation ladder
- [ ] `validate:quick` PASS: `go build ./... && go test -race -short ./...` exit 0
- [ ] `test:integration` PASS: `go test -race -tags=integration ./internal/embed/...` exit 0
- [ ] `self-review:response-shape` N/A (no HTTP handler change)
- [ ] `self-review:staged-files` PASS (no `.opencode/`, no `package-lock.json`)
- [ ] `smoke:e2e` SKIP — no API surface change. Document in PR per D11.

## Post-implementation
- [ ] IN-PROGRESS gate: `./scripts/harness-check.sh in-progress`
- [ ] Push branch + open PR (max 3 commits R29)
- [ ] PRE-MERGE gate: `./scripts/harness-check.sh pre-merge --pr <N>`
- [ ] `/review-work` skill (Oracle goal + quality + security + sub-agents)
- [ ] Address Gemini bot review (up to 3 cycles)
- [ ] Merge PR (squash, `--delete-branch`)
- [ ] POST-MERGE gate: `./scripts/harness-check.sh post-merge --pr <N>`
- [ ] `openspec archive embed-status-index-and-inflight-dedup --yes`
- [ ] Verify auto-tag + release + npm publish pipeline succeeds for the merge commit
- [ ] Close issue #322 with release note (auto-closed if PR body has `Closes #322`)
- [ ] Cleanup: `git worktree remove .opencode/worktrees/feat-322-embed-status-index`
- [ ] Cleanup: `git branch -D feat/322-embed-status-index-and-inflight-dedup`
- [ ] Write nano-brain memory summary of merge outcome
