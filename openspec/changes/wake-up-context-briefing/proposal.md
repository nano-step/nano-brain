## Why

Every AI coding session starts cold. The agent has no context about the workspace, recent decisions, or what matters most — it must manually query memory to rebuild awareness. This wastes tokens and time on every session start.

mempalace solves this with a 4-layer memory stack that loads ~170 tokens on wake-up. nano-brain has all the underlying data (importance scores, tags, collections, workspace profiles) but no way to compose them into a single compact briefing. Adding a `wake-up` command gives agents instant context at session start.

## What Changes

- New CLI command: `nano-brain wake-up` — generates a compact context briefing (~200-500 tokens)
- New MCP tool: `memory_wake_up` — same briefing available to MCP-connected AI tools
- New HTTP endpoint: `GET/POST /api/wake-up` — for container/programmatic access
- New core module: `src/wake-up.ts` — shared `generateBriefing()` function
- New Store methods: `getTopAccessedDocuments()` and `getRecentDocumentsByTags()` — targeted queries for briefing data
- Template-based output optimized for system prompt injection (no LLM dependency)

## Capabilities

### New Capabilities
- `wake-up-briefing`: Core briefing generation logic u2014 composing workspace identity (L0) from collections/profile + critical facts (L1) from top-accessed documents and decision-tagged memories into a token-budgeted text output
- `wake-up-store-queries`: Two new Store interface methods for targeted document retrieval by access count and by tags

### Modified Capabilities

## Impact

- **src/types.ts**: 2 new methods added to Store interface
- **src/store.ts**: 2 new prepared statements + method implementations (~30 lines)
- **src/wake-up.ts**: New file (~80-100 lines)
- **src/index.ts**: New `handleWakeUp()` handler + switch case (~50 lines)
- **src/server.ts**: New MCP tool + HTTP route (~50 lines)
- **Total**: ~220 new lines across 5 files
- **No breaking changes**
- **No new dependencies**