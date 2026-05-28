## 1. Config & LLM Client

- [x] 1.1 Add `SummarizationConfig` struct to `internal/config/config.go` (enabled, provider_url, api_key, model, max_tokens, concurrency, output_dir) and wire into `Config`
- [x] 1.2 Add summarization defaults to `internal/config/defaults.go` (enabled=true, model="nano-brain", max_tokens=4096, concurrency=3, output_dir="~/.nano-brain/summaries")
- [x] 1.3 Add env var fallback for API key (`NANO_BRAIN_SUMMARIZE_API_KEY`) in config loading
- [x] 1.4 Create `internal/summarize/client.go` — OpenAI-compatible HTTP client: `ChatCompletion(ctx, systemPrompt, userPrompt) (string, TokenUsage, error)` with SSE streaming parser, retry (3x backoff for 429/5xx), structured logging (model, tokens, latency)
- [x] 1.5 Write tests for LLM client: SSE parsing, retry logic, error handling

## 2. Strip Logic

- [x] 2.1 Create `internal/summarize/strip.go` — `StripOpenCode(content string) string` function: remove system prompts, replace tool output >200 chars with placeholder, collapse code blocks >20 lines, deduplicate repeated errors, remove base64
- [x] 2.2 Add `StripClaude(content string) string` function: replace tool_result output >200 chars, replace long tool_use commands >5 lines
- [x] 2.3 Write tests for strip: verify system prompt removal, tool placeholder format, code block collapse, error dedup, base64 removal, content preservation

## 3. Map-Reduce Pipeline

- [x] 3.1 Create `internal/summarize/prompts.go` — map prompt template (extract activities, decisions, files, problems, learnings per chunk) and reduce prompt template (merge into structured markdown with 5 sections)
- [x] 3.2 Create `internal/summarize/pipeline.go` — `Summarize(ctx, sessionContent, metadata) (string, error)` orchestrator: strip → chunk → map (parallel, bounded concurrency) → reduce → return markdown
- [x] 3.3 Implement short-circuit: if stripped content fits in single chunk (<4000 chars), skip map-reduce and do single summarization call
- [x] 3.4 Implement hierarchical reduce: if merged chunk summaries exceed estimated context limit, batch-reduce in groups of 10 then final reduce
- [x] 3.5 Add session metadata header generation: title, date, duration, agent, project, session ID, parent/child/sibling links (query OpenCode DB for relationships)
- [x] 3.6 Write tests for pipeline: single-chunk shortcut, multi-chunk map-reduce, hierarchical reduce, metadata header with relationships

## 4. Persistence & Integration

- [x] 4.1 Create `internal/summarize/persist.go` — save summary: write `.md` file to output_dir (create dir if needed, slugify filename), upsert document with `source_path="summary://{source}/{session_id}"` + collection="session-summary", chunk, enqueue for embedding
- [x] 4.2 Modify `internal/harvest/opencode_sqlite.go` — after successful harvest of a session, call summarizer with rendered content + session metadata (title, id, parent_id, agent, path, timestamps)
- [x] 4.3 Modify `internal/harvest/claudecode.go` — after successful harvest, call summarizer with rendered content + session metadata (filename, first user message for title)
- [x] 4.4 Modify `internal/harvest/runner.go` — accept Summarizer dependency, pass to harvesters
- [x] 4.5 Modify `cmd/nano-brain/main.go` — initialize Summarizer from config, pass to harvest runner. Skip initialization if summarization disabled.
- [x] 4.6 Write integration test: mock LLM client, harvest a session, verify `.md` file written + document upserted + chunks enqueued

## 5. Documentation & Validation

- [x] 5.1 Update setup/onboarding docs with `summarization` config section: provider URL, API key (config or env var), model, output_dir, concurrency
- [x] 5.2 Run full validation: `CGO_ENABLED=0 go build ./... && go vet ./... && go test -race -short ./...`
- [ ] 5.3 Manual smoke test: start nano-brain with summarization enabled, trigger harvest, verify summary `.md` created and searchable via `memory_query`
