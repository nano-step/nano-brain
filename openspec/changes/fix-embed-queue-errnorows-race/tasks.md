# Tasks: Fix Embed Queue ErrNoRows Race

Tracking: #259

## Phase A — TDD

- [ ] **A1** Read `internal/embed/queue.go` around the `failed to fetch chunk` log — note worker function, pending counter handling
- [ ] **A2** Read existing test setup `internal/embed/queue_test.go` — note PG harness pattern, mock embedder
- [ ] **A3** Write failing test `TestEmbed_SkipsDeletedChunk` in `queue_test.go` (real PG):
  - Insert workspace + doc + chunk
  - Enqueue chunk ID
  - DELETE the chunk row
  - Wait for worker to pop
  - Capture logs (use `zerolog.New(buf)`)
  - Assert: 0 `failed to fetch chunk` ERROR, 1 DEBUG with `chunk_id`, pending counter == 0
  - Run test → MUST FAIL on baseline code
- [ ] **A4** Apply fix in `internal/embed/queue.go`:
  ```go
  chunk, err := q.queries.GetChunkByID(ctx, chunkID)
  if err != nil {
      if errors.Is(err, sql.ErrNoRows) {
          q.logger.Debug().Str("chunk_id", chunkID.String()).Msg("embed-queue: chunk no longer exists (likely cascade-deleted), skipping")
          q.pending.Add(-1)
          return
      }
      q.logger.Error().Err(err).Str("chunk_id", chunkID.String()).Msg("failed to fetch chunk")
      q.pending.Add(-1)
      return
  }
  ```
- [ ] **A5** Add imports: `"database/sql"`, `"errors"` (verify not already present)
- [ ] **A6** Re-run test → MUST PASS
- [ ] **A7** Run full embed package tests: `go test -race -short ./internal/embed/...`
- [ ] **A8** Run full integration suite (excl. pre-existing failures): `go test -race -tags=integration $(go list ./... | grep -v internal/search)`

## Phase B — Validate ladder

- [ ] **B1** `validate:quick`: `go build ./... && go test -race -short ./...` → green
- [ ] **B2** `self-review:staged-files`: `git status` clean before commit (no `.opencode/`, no `package-lock.json`)

## Phase C — PR + merge

- [ ] **C1** Commit:
  ```
  fix(embed-queue): treat sql.ErrNoRows as benign skip on chunk fetch (#259)

  Embed worker fails to fetch chunk by ID when the chunk row was deleted between
  enqueue and pop (cascade from document re-upsert, workspace deletion, or
  cleanup-orphan-workspaces sweep). Previously emitted ERROR per occurrence.

  Now: ErrNoRows -> DEBUG log + skip + decrement pending. All other errors keep
  ERROR behavior.

  Test (TestEmbed_SkipsDeletedChunk) inserts workspace+doc+chunk, enqueues,
  DELETEs chunk, lets worker pop, asserts 0 ERROR logs + 1 DEBUG log + pending
  counter == 0.
  ```
- [ ] **C2** Push branch
- [ ] **C3** Open PR with `Closes #259`
- [ ] **C4** Wait for Gemini bot review, triage per R31 (if any findings)
- [ ] **C5** Squash merge with `--admin` if Gemini blocks but code is sound
- [ ] **C6** Close issue (manual if auto-close fails on b-main base)
- [ ] **C7** `openspec archive fix-embed-queue-errnorows-race --yes` + commit + push
- [ ] **C8** Remove worktree + delete local branch
