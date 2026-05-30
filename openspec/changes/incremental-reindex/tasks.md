# Tasks — Incremental Reindex (#158)

## Pre-implementation
- [ ] Run deep-design with Metis + Oracle on `proposal.md` + `design.md`. Revise until clean pass.
- [ ] Confirm `documents.content_hash` is populated for ALL existing rows (one-time validation query): `SELECT count(*) FROM documents WHERE content_hash IS NULL OR content_hash = ''` → must be 0.

## Implementation

### Backend
- [ ] Add `ListDocumentsByWorkspace` sqlc query returning `(source_path, content_hash, id)` rows.
- [ ] Refactor `internal/server/handlers/reindex.go` (or wherever the handler lives):
  - [ ] Replace full-wipe with diff loop described in design.md.
  - [ ] Wrap per-document delete-old + insert-new in single transaction.
  - [ ] Return counters `{scanned, skipped, embedded, deleted, duration_ms}` in response body.
- [ ] CLI: `cmd/nano-brain/commands.go runReindexCmd` — accept new `--force-wipe` flag for the old full-wipe behavior; default = incremental. Update `printUsage()`.
- [ ] Logging: structured log at INFO with counters, at DEBUG with per-file decision.

### Tests
- [ ] Unit: `reindex_test.go` covering 4 cases (new / unchanged / changed / deleted file) with mock querier.
- [ ] Integration (build tag `integration`): live PG, populate workspace, reindex twice, assert counters.
- [ ] CLI test: `runReindexCmd` with `--force-wipe` triggers old path; default triggers new.

### Docs
- [ ] Update CHANGELOG `[Unreleased] ### Features` and `### Breaking` if any.
- [ ] Update README CLI table for reindex flag.
- [ ] Update README REST section if response shape documented there.

### Self-review evidence
- [ ] `docs/evidence/self-review-feat-158-incremental-reindex.md` with E2E proof (timing comparison full vs incremental on nano-brain repo).

## Post-merge
- [ ] `openspec archive incremental-reindex`
- [ ] Close issue #158 with link to PR
