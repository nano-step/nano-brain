# Phase 1: PREPARE — wake-up-context-briefing

## Feature Summary
The wake-up feature generates a compact context briefing (~2000 chars) for AI session start. It surfaces workspace identity (L0) and critical facts (L1: top-accessed docs + recent decisions).

## Surfaces
1. **CLI**: `npx nano-brain wake-up [--json] [--workspace=<path>]`
2. **MCP tool**: `memory_wake_up` with workspace, limit, json params
3. **HTTP API**: GET/POST `/api/wake-up` with workspace, json, limit params

## Changed Files
| File | Change | Lines |
|------|--------|-------|
| src/types.ts | 2 new Store interface methods | ~10 |
| src/store.ts | 2 prepared statements + implementations | ~20 |
| src/wake-up.ts | NEW core briefing logic | 122 |
| src/index.ts | CLI handler + help text | ~15 |
| src/server.ts | MCP tool + HTTP routes | ~60 |

## Core Function
`generateBriefing(store, configPath, projectHash, options?)` → `BriefingResult`

## New Store Methods
- `getTopAccessedDocuments(limit, projectHash?)` — top docs by access_count DESC
- `getRecentDocumentsByTags(tags[], limit, projectHash?)` — docs by tag, ordered modified_at DESC

## Key Specs (from spec.md)
- Returns BriefingResult with l0, l1_memories, l1_decisions, formatted
- L0: collection names + top topics from WorkspaceProfile
- L1: up to 10 top-accessed non-superseded docs + up to 5 recent decision-tagged docs
- Output capped at 2000 characters
- Empty workspace returns "no memories yet" message
- Superseded documents excluded from all queries

## Test Scope
7 dimensions: UI/UX, API, Performance, Security, Data Integrity, Infrastructure, Edge Cases

## Output Directory
`/Users/tamlh/workspaces/self/AI/Tools/nano-brain/ai/test-case/rri-t/wake-up-context-briefing/`
