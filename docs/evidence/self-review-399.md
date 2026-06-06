# Self-Review: #399 Token-Aware Batching + Retry (PR1)

## Staged Files Check
- Only intended files staged (codesummarize, config, storage, migration, openspec)
- No `.opencode/`, `package-lock.json`, or unrelated files

## Acceptance Criteria

| AC | Status | Evidence |
|----|--------|----------|
| Token estimation before batch send | ✅ | `tokens.go`: EstimateTokens (chars/4 + overhead) |
| Auto-split when estimated > max_batch_tokens | ✅ | `service.go`: splitAndSend recursive binary split |
| Retry: max 3, exponential backoff, log each | ✅ | `retry.go`: sendWithRetry with attempt logging |
| Error classification: transient vs permanent | ✅ | `retry.go`: ClassifyError checks HTTP status patterns |
| Failed symbols tracked in DB | ✅ | Migration 00018 + sqlc queries (UpsertCodeSummarizationFailure) |
| Config fields: max_batch_tokens, max_retries, retry_backoff_seconds | ✅ | config.go + defaults.go |

## Verification
```
go build ./...                     # PASS
go test -race -short ./...         # PASS (all packages)
golangci-lint run ./internal/codesummarize/...  # PASS
```

## Pre-existing Issues
- TestBackfill_DryRun (same as #397) — unrelated to this PR
