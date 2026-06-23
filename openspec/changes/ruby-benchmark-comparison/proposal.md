## Why

nano-brain is a persistent memory + code intelligence system for AI coding agents. It provides hybrid search (BM25 + vector + RRF), cross-session recall, symbol graph analysis, flow/sequence diagrams, and multi-workspace indexing. But we have no comparison against other agent memory solutions — we don't know where we lead, where we lag, and where to invest next.

The agent memory space is crowded and fast-moving: Mem0, Zep, Letta (MemGPT), Cognee, Microsoft GraphRAG, and LlamaIndex all compete for the "AI agent's brain" slot. Each takes a different approach to the same problem: **how does an agent remember, recall, and reason across sessions?**

Comparing nano-brain against these solutions gives us:
- **Competitive positioning**: Where do we lead (code intelligence, hybrid search)? Where do we lag (generality, UI)?
- **Feature gap analysis**: What capabilities do competitors have that we're missing?
- **Performance baseline**: How does our Go binary compare to Python/Node memory servers?
- **Product roadmap input**: What should we build next to stay competitive?

## What Changes

- Research and install 5+ agent memory comparison tools
- Run standardized benchmarks against real workspaces (nano-brain, express-app, rails-app)
- Measure: search quality, recall accuracy, code intelligence, latency, throughput
- Generate comparison report with competitive positioning
- No changes to nano-brain code — this is a research/benchmark task

## Capabilities Measured

### Core Memory
- `memory_search` / `memory_query` / `memory_vsearch` — hybrid, BM25, vector search
- `memory_get` / `memory_write` — document CRUD
- `memory_wake_up` — session-start briefing
- `memory_tags` — collection inventory

### Code Intelligence
- `memory_graph` — 1-hop callers/callees
- `memory_impact` — reverse impact BFS
- `memory_trace` — forward call chain
- `memory_symbols` — symbol search

### Flow & Diagrams
- `memory_flow` — HTTP entry → call chain
- `memory_flowchart` — per-function CFG

### Infrastructure
- Multi-workspace support
- File watcher auto-indexing
- Embedding queue management
- Cross-session persistence

## Impact

- **Files affected**: `benchmarks/comparison/` (new directory, scripts, results)
- **Dependencies**: Docker or local install of comparison tools
- **Risk**: Low — benchmark-only, no production code changes
- **Performance**: Runs offline against test data
