## Why

Harvested sessions (OpenCode SQLite, Claude Code JSONL) are currently chunked and embedded raw — tool outputs, system prompts, and binary noise pollute the vector DB, waste embedding tokens, and degrade search quality. Users need meaningful session summaries (goals, decisions, files touched, problems, learnings) with cross-session linking, saved as `.md` files and embedded for semantic retrieval.

## What Changes

- Add OpenAI-compatible LLM client (`net/http`, no new dependencies) for text generation
- Add 3-stage summarization pipeline: Strip → Map-Reduce → Save
  - **Strip**: Remove system prompts, tool call results (keep name+short desc), large code blocks, binary/base64, repeated errors
  - **Map**: Chunk stripped content, LLM-summarize each chunk in parallel (bounded goroutines)
  - **Reduce**: Merge chunk summaries into structured final summary via single LLM call
- Save summary as physical `.md` file to configurable output dir + store in DB + chunk + embed
- Summary includes cross-session links (parent/child/sibling via `parent_id`, project grouping via `path`)
- Add `summarization` section to config.yaml (provider URL, API key, model, max_tokens, output_dir, enabled flag)
- Document LLM provider configuration in setup/onboarding guide
- Wire pipeline into harvest cycle — summarize newly harvested sessions where content_hash changed

## Capabilities

### New Capabilities
- `llm-client`: OpenAI-compatible HTTP client for chat completions (streaming SSE parsing, retry, configurable provider)
- `session-strip`: Content filtering that removes low-value noise (tool output, system prompts, code blocks) while preserving intent, reasoning, and decisions
- `map-reduce-summarize`: Parallel map-reduce pipeline that summarizes arbitrarily long sessions (100K-1M+ tokens) into structured markdown
- `summary-persistence`: Save summaries as `.md` files + DB documents with source_path-based idempotent upsert and automatic embedding

### Modified Capabilities
- `search-pipeline`: Summary documents become a new searchable collection alongside existing code/memory documents

## Impact

- **New package**: `internal/summarize/` (client, strip, pipeline, prompts)
- **Config**: New `SummarizationConfig` struct in `internal/config/`
- **Harvesters**: `opencode_sqlite.go` and `claudecode.go` gain post-harvest summarization call
- **Runner**: `runner.go` wired with summarizer dependency
- **Main**: `cmd/nano-brain/main.go` initializes summarizer from config
- **Docs**: Setup guide updated with LLM provider configuration
- **External dependency**: ai-proxy.thnkandgrow.com (or any OpenAI-compatible endpoint) — network call during harvest
- **No new Go module dependencies**
