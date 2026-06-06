## PR1: Backend — Token estimation + retry + failure tracking

- [ ] 1.1 Add config fields to `CodeSummarizationConfig`: `MaxBatchTokens int`, `MaxRetries int`, `RetryBackoffSeconds int`
- [ ] 1.2 Add defaults in `defaults.go`: max_batch_tokens=100000, max_retries=3, retry_backoff_seconds=1
- [ ] 1.3 Create migration `00018_code_summarization_failures.sql` with `code_summarization_failures` table
- [ ] 1.4 Add sqlc queries: `InsertCodeSummarizationFailure`, `UpdateCodeSummarizationFailure`, `GetUnresolvedFailures`, `ResolveFailure`, `GetSummarizationStatus`
- [ ] 1.5 Run `sqlc generate`
- [ ] 1.6 Create `internal/codesummarize/tokens.go` — `estimateTokens(symbols []SymbolForSummary) int` function
- [ ] 1.7 Create `internal/codesummarize/retry.go` — `sendWithRetry(ctx, batch)` with backoff + error classification
- [ ] 1.8 Refactor `service.go` batch loop → use `splitAndSend()` recursive function
- [ ] 1.9 Add failure tracking: on permanent error or retries exhausted → persist to DB
- [ ] 1.10 Unit tests: token estimation, retry logic (mock HTTP), error classification
- [ ] 1.11 `go build ./...` + `go test -race -short ./...` + `golangci-lint`

## PR2: Web UI — Status + retry endpoints

- [ ] 2.1 Create handler `GET /api/v1/code/summarize/status` — returns counts (total, summarized, pending, failed)
- [ ] 2.2 Create handler `GET /api/v1/code/summarize/failures` — list unresolved failures
- [ ] 2.3 Create handler `POST /api/v1/code/summarize/retry` — retry specific failure IDs
- [ ] 2.4 Create handler `POST /api/v1/code/summarize/retry-all` — retry all unresolved
- [ ] 2.5 Register routes in routes.go
- [ ] 2.6 Integration test: create failure → retry → verify resolved
- [ ] 2.7 smoke:e2e for new endpoints
