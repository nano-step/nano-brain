# Validation — fix-embedding-insert-race

## Passing change-specific checks

```text
go build ./... && go test -race -short ./...                         PASS
go test -race -tags=integration ./internal/storage                    PASS
go test -race ./internal/embed ./internal/server/handlers             PASS
go test -race -tags=integration ./internal/storage \
  -run '^TestInsertEmbedding_returnsNoRows_when_sourceChunkWasDeleted$' PASS
```

All database-backed checks used `nanobrain_test`; the focused storage test uses an isolated schema. The API smoke is recorded in `smoke-e2e-fix-embedding-insert-race.md`.

## Full integration sweep

`go test -race -tags=integration ./...` was run twice: once without a test server and once with an isolated `:3199` test server. The second run removed the server-dependent benchmark failure, but both runs still failed in unrelated existing areas:

- `cmd/nano-brain`: backfill-summary fixture expectations and the orphan-cleanup schema fixture.
- `internal/storage/sqlc`: stale raw OpenCode cleanup expectations.
- `internal/summarize`: persisted-session collection and filename expectations.

The change-specific storage, queue, and handler packages passed in that full sweep.

## Base-revision comparison

A clean detached worktree at the unchanged base revision was checked with the same isolated server and test database:

```text
go test -race -tags=integration ./cmd/nano-brain ./internal/storage/sqlc ./internal/summarize
```

It reproduced the same failures in all three areas, confirming they are not introduced by this change. The temporary comparison worktree and test server were removed after verification.
