# Code Symbol Summarization

Tracking: #397

## Problem

When users ask "how does feature X work?", nano-brain returns irrelevant results because:
1. Search only matches keywords/symbols — no semantic understanding of code logic
2. No pre-computed explanations exist for code modules/features
3. Symbol index has structure (name, kind, signature) but no behavioral description

Example: querying "recursive .gitignore loading" returns harness-loop docs (keyword "watcher" match) instead of `gitignoreStack` (the actual implementation).

## Solution (Option C — Hybrid: Summary + Graph)

Pre-compute LLM-generated summaries for code symbols at index time, store as searchable content, combine with existing graph traversal at query time.

### Key Constraint: Batching

LLM providers have low request-per-day limits (e.g., 3000 RPD on Gemma 4) but unlimited tokens. Solution: batch 20-40 symbols per LLM request, receive structured JSON array output.

Math: 3000 RPD ÷ 1 request per 30 symbols = 90,000 symbols/day max throughput.

## Approach

1. **Index time**: After symbol extraction, batch symbols → LLM → store summaries
2. **Storage**: New `summary` field on symbol chunks (or separate documents in `code` collection)
3. **Search**: Summaries participate in BM25 + vector search naturally
4. **Query time**: Retrieve matching summaries → expand via graph (callers/callees) → return structured answer

## Config

```yaml
code_summarization:
  enabled: false                    # opt-in
  provider_url: ""                  # OpenAI-compatible endpoint
  api_key: ""                       # or NANO_BRAIN_CODE_SUMMARIZE_API_KEY
  model: "gemini/gemini-2.5-flash"
  batch_size: 30                    # symbols per LLM request
  max_output_tokens: 8000           # per batch request
  concurrency: 2                    # parallel batch requests
  max_requests_per_day: 0           # 0 = unlimited, >0 = daily budget cap
  fallback_model: ""                # used when primary fails
```

## Non-Goals (this change)

- Query-time LLM synthesis (future work)
- Re-ranking search results using summaries (future work)
- Multi-language support beyond Go/TS/Python/JS (future work)

## Acceptance Criteria

- [ ] Symbols get LLM-generated 2-4 sentence summaries at index time
- [ ] Batching: configurable batch_size (default 30 symbols per request)
- [ ] Summaries searchable via memory_query/memory_search/memory_vsearch
- [ ] Incremental: only new/changed symbols get re-summarized
- [ ] Config section in config.yml with sensible defaults
- [ ] Integration with watcher pipeline (post-symbol-extraction hook)
- [ ] Daily budget cap to prevent runaway costs
- [ ] Graceful degradation when LLM unavailable (skip summarization, log warning)
