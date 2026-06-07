## Why

Tracking: #405

AI agents querying nano-brain cannot answer "when/how does X work?" questions because the system only indexes structural relationships (symbols, call graph edges), not behavioral knowledge. When an agent asks `memory_query("when are symbols indexed?")`, it gets raw code chunks — not an explanation of execution flow, trigger conditions, or calling context. This forces agents to read source files directly, defeating the purpose of a memory system.

Research comparing nano-brain to Greptile, Mem0, Aider, and Bloop identified three gaps: (1) no LLM-generated summaries of code behavior, (2) no entity-based search boosting, (3) no importance-weighted ranking. Closing these gaps enables agents to get answers from memory in seconds rather than reading code for minutes.

## What Changes

- **Auto-trigger code summarization**: Wire watcher pipeline to automatically enqueue symbols for LLM summarization after indexing. Background workers process queue asynchronously with batching, budget control, and retry.
- **Entity linking search boost**: Extract entity names (functions, types, constants) from chunks at index time, store in dedicated table, boost matching results after RRF fusion.
- **Flow-enriched summaries**: Enrich summarization prompts with caller/callee context from existing graph edges so summaries include "triggered by X when Y" behavioral information.
- **PageRank symbol importance**: Pre-compute importance scores from graph edge frequency, boost high-importance symbols in search results.
- **Benchmark suite**: Create 50+ query/expected-result pairs to measure search quality before and after each enhancement.

## Capabilities

### New Capabilities
- `auto-summarization`: Automatic LLM summarization of code symbols triggered by watcher indexing, with async worker pool, budget control, content-hash dedup, and rate limiting.
- `entity-linking`: Entity extraction from chunks at index time, storage in normalized table, and post-RRF score boosting for entity-matching results.
- `flow-enriched-summaries`: Caller/callee context injection into summarization prompts with fan-out caps, graph-hash invalidation, and cascade limits.
- `pagerank-importance`: Pre-computed PageRank scores for symbols based on graph edge frequency, with configurable boost weight and daily/threshold recomputation.
- `search-benchmark`: Regression testing suite with query/expected-result pairs, nDCG measurement, and automated quality gates.

### Modified Capabilities
- `hybrid-search`: Post-RRF entity boost signal and PageRank importance boost added to scoring pipeline. Feature-flagged with kill switch.
- `code-summarization`: Existing manual-trigger service extended with auto-trigger via watcher hook and background polling.

## Impact

- **internal/watcher/**: New hook after `extractAndUpsertSymbols()` to enqueue summarization
- **internal/codesummarize/**: Background worker pool, polling loop, content-hash dedup
- **internal/search/**: Post-RRF entity boost + PageRank boost (feature-flagged)
- **internal/graph/**: Bulk caller/callee queries, PageRank computation
- **internal/storage/migrations/**: 3-4 new migrations (summary column, chunk_entities table, importance_score column, benchmark table)
- **internal/config/**: New config sections for entity linking, PageRank, auto-summarization toggle
- **MCP tools**: No new tools — enhancements surface through existing `memory_query`/`memory_search` results
- **External dependency**: LLM provider (already configured via code_summarization config)
