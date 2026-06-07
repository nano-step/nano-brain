## 1. Config + Migration (foundation)

- [ ] 1.1 Add `CodeSummarizationConfig` struct to `internal/config/config.go` with all fields (enabled, provider_url, api_key, model, batch_size, max_output_tokens, concurrency, max_requests_per_day, max_symbol_lines, poll_interval_seconds, max_summaries_per_cycle, fallback_model)
- [ ] 1.2 Add `CodeSummarization CodeSummarizationConfig` field to root `Config` struct
- [ ] 1.3 Add default values in config loading (batch_size=30, max_output_tokens=8000, concurrency=2, max_requests_per_day=0, max_symbol_lines=500, poll_interval_seconds=60, max_summaries_per_cycle=300)
- [ ] 1.4 Support env var `NANO_BRAIN_CODE_SUMMARIZE_API_KEY` fallback for api_key
- [ ] 1.5 Create goose migration `migrations/000XX_code_summarization_usage.sql` with `code_summarization_usage` table (workspace_hash TEXT, usage_date DATE, request_count INT, PK on workspace_hash+usage_date)
- [ ] 1.6 Add sqlc queries for budget: `IncrementCodeSummarizationUsage`, `GetCodeSummarizationUsage`
- [ ] 1.7 Run `sqlc generate` and verify generated code compiles

## 2. Core package (`internal/codesummarize/`)

- [ ] 2.1 Create `internal/codesummarize/summarize.go` — `CodeSummarizer` interface, `SymbolForSummary` and `SymbolSummary` types
- [ ] 2.2 Create `internal/codesummarize/provider.go` — OpenAI-compatible LLM client (chat completions, JSON response parsing). Reuse patterns from `internal/summarize/client.go`
- [ ] 2.3 Create `internal/codesummarize/prompt.go` — prompt template builder: takes `[]SymbolForSummary`, renders multi-symbol prompt with structured JSON output instructions
- [ ] 2.4 Create `internal/codesummarize/budget.go` — DB-persisted daily budget counter (increment, check, uses sqlc queries from 1.6)
- [ ] 2.5 Create `internal/codesummarize/summarize_test.go` — unit tests for prompt building, response parsing, composite key matching
- [ ] 2.6 Create `internal/codesummarize/provider_test.go` — unit test with mock HTTP server returning structured JSON
- [ ] 2.7 Create `internal/codesummarize/budget_test.go` — integration test with real DB (TestBudgetIncrement, TestBudgetExhausted, TestBudgetNewDay)

## 3. Query for unsummarized symbols

- [ ] 3.1 Add sqlc query `GetUnsummarizedSymbols` — LEFT JOIN chunks against documents to find symbol chunks without matching summary doc (by content_hash[:8] in source_path)
- [ ] 3.2 Verify query returns correct results with integration test (create symbol chunks, verify they appear; create summary doc, verify they disappear from results)

## 4. Manual trigger API endpoint

- [ ] 4.1 Create handler `internal/server/handlers/code_summarize.go` — `POST /api/v1/code/summarize` accepting `{"workspace": "<hash>"}`
- [ ] 4.2 Handler calls `CodeSummarizer.RunOnce(ctx, workspaceHash)` which processes up to `max_summaries_per_cycle` symbols
- [ ] 4.3 Returns `{"processed": N, "skipped": M, "errors": E}` on success
- [ ] 4.4 Returns 400 if feature disabled, 200 with message if budget exhausted
- [ ] 4.5 Register route in `internal/server/server.go`
- [ ] 4.6 Integration test: register workspace, create symbol chunks, call endpoint, verify summary docs created

## 5. Summary document storage

- [ ] 5.1 Implement `upsertSummaryDoc` in codesummarize package — creates document via existing `UpsertDocument` with correct source_path format, tags, metadata, embedding_strategy=summary
- [ ] 5.2 After upserting document, enqueue resulting chunk_ids for embedding via embed queue
- [ ] 5.3 Unit test: verify source_path format `<file>?symbol=<name>&kind=<kind>&hash=<hash[:8]>&summary=true`
- [ ] 5.4 Unit test: verify metadata contains symbol_name, symbol_kind, source_file, source_content_hash, model_version

## 6. Background polling service (PR2)

- [ ] 6.1 Create `internal/codesummarize/service.go` — polling goroutine with configurable interval, context cancellation
- [ ] 6.2 Poll loop: query unsummarized symbols → check budget → batch → process → enqueue embeddings
- [ ] 6.3 Respect `max_summaries_per_cycle` cap per cycle
- [ ] 6.4 Check embedding queue depth before processing (pause if >1000 pending)
- [ ] 6.5 Wire service startup in `cmd/nano-brain/` server boot (only if enabled)
- [ ] 6.6 Graceful shutdown: finish in-progress batch, stop polling

## 7. Orphan cleanup (PR2)

- [ ] 7.1 Add sqlc query `GetOrphanSummaries` — find summary docs whose source_content_hash doesn't match any current chunk
- [ ] 7.2 Implement hourly GC in service loop — delete orphan summaries
- [ ] 7.3 Integration test: create summary, delete source symbol chunk, run GC, verify summary removed

## 8. Validation + smoke test

- [ ] 8.1 `go build ./...` passes
- [ ] 8.2 `go test -race -short ./...` passes
- [ ] 8.3 `go test -race -tags=integration ./...` passes (budget, unsummarized query, manual trigger, orphan cleanup)
- [ ] 8.4 smoke:e2e — build binary, start server, register workspace with Go files, call `POST /api/v1/code/summarize`, verify summary docs appear in `POST /api/v1/search` with query matching summary text
