## Context

Debugging requires finding context across multiple collections: code (error paths, config values), sessions (past debugging history), and config (thresholds, TTLs). Currently agents call `memory_search` 3-5 times with different queries, then manually synthesize results. This is slow and often misses context.

Baseline debugging benchmark: code=0.625, session=0.667, combined=0.880. Session search adds significant value (+0.255 over code alone), but requires separate tool calls.

## Goals / Non-Goals

**Goals:**
- Single tool call returns debugging context from code + sessions + config
- Results labeled with source (`code`, `session`, `config`) so agent knows where each came from
- Zero impact on existing behavior when `mode` is omitted
- Agent skill teaches optimal debugging workflow

**Non-Goals:**
- Auto-detection of debugging intent (agent decides, not server)
- Debugging-specific LLM summarization
- Storing debugging state between queries
- Modifying the existing search pipeline scoring

## Decisions

### Decision 1: Parallel search with RRF merge (chosen)

Add `mode=debugging` parameter. When set, server runs 3 parallel searches:
1. `memory_search(query)` — code results
2. `memory_search(query + " debug session error")` — session results  
3. `memory_search(query, tags=["config"])` — config results (if tag-based filtering available)

Merge results using existing RRF fusion, add `source` label to each result.

**Alternative A: Single enhanced query** — rewrite query to include debugging terms. Rejected because it changes BM25 scoring behavior.

**Alternative B: New MCP tool `memory_debug`** — separate tool for debugging. Rejected because it fragments the API surface. Parameter addition is simpler.

### Decision 2: Source labeling via metadata (chosen)

Each result gets a `source` field: `"code"`, `"session"`, or `"config"`.

**Alternative: Separate response sections** — group results by source. Rejected because it breaks the existing response contract and makes RRF scoring impossible.

### Decision 3: Agent skill as separate file (chosen)

Debugging skill is a `.opencode/skills/debugging/SKILL.md` file that teaches the agent workflow.

**Alternative: Embed in AGENTS.md** — too broad. Skill file is focused and can be loaded on-demand.

## Risks / Trade-offs

- **[Latency]** 3 parallel searches may be slower than 1 → Mitigate with timeout per sub-search (2s)
- **[Score dilution]** Adding session/config results may lower average BM25 scores → Mitigate by keeping source label so agent can filter
- **[Query pollution]** Appending "debug session error" may hurt some queries → Mitigate by testing against benchmark
- **[Config complexity]** New parameter adds cognitive load → Mitigate by making it optional with clear default
