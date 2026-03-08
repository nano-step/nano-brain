## Why

nano-brain stores memories indefinitely with equal weight — a 6-month-old unused debugging note ranks the same as yesterday's critical architecture decision. There is no automatic organization, no relevance scoring, and no way to distinguish signal from noise as memory accumulates over time. Competitive memory systems (Mem0, memU) achieve 26-74% higher accuracy on benchmarks partly through intelligent memory lifecycle management. Phase 1 addresses the three lowest-effort, highest-impact gaps: relevance decay, automatic categorization, and usage-based ranking.

## What Changes

- **Memory relevance decay**: Add `access_count` and `last_accessed_at` tracking to documents. Introduce a configurable decay function that deprioritizes stale, unused memories in search results. Memories accessed frequently stay prominent; neglected ones fade gracefully without deletion.
- **Auto-categorization on write**: When `memory_write` is called, classify the content into predefined categories (architecture-decision, debugging-insight, tool-config, pattern, preference, context) using lightweight keyword/heuristic matching. Populate the existing `tags` field automatically. No LLM dependency for Phase 1 — keep it fast and local.
- **Usage-based search boosting**: Integrate access frequency and recency into the hybrid search scoring pipeline. Frequently retrieved memories get a configurable boost in RRF fusion, complementing the existing BM25 + vector + rerank pipeline.

## Capabilities

### New Capabilities
- `memory-relevance-decay`: Track memory access patterns and apply time-based relevance decay to search scoring
- `auto-categorization`: Automatically classify and tag memories on write using heuristic rules
- `usage-based-boosting`: Boost frequently accessed memories in hybrid search results

### Modified Capabilities
- `search-pipeline`: Search scoring now incorporates access frequency and recency as additional ranking signals
- `storage-limits`: Decay metadata (access_count, last_accessed_at) added to document schema; retention eviction can optionally prioritize low-access documents

## Impact

- **Schema**: New columns on `documents` table (`access_count INTEGER DEFAULT 0`, `last_accessed_at TEXT`)
- **Search pipeline** (`search.ts`): Additional scoring factor in RRF fusion for access-based boosting
- **Store** (`store.ts`): Track access on every search result retrieval; auto-tag on document insert
- **MCP server** (`server.ts`): `memory_write` gains auto-categorization; search tools update access tracking
- **Config** (`config.yml`): New `decay` section with `halfLife`, `boostWeight`, `enabled` fields
- **No new dependencies**: Heuristic categorization avoids LLM calls; decay is pure math
- **Backward compatible**: Existing memories get `access_count=0`, `last_accessed_at=NULL`; decay defaults to disabled until configured
