## ADDED Requirements

### Requirement: memory_get enforces default line cap
The `memory_get` MCP tool SHALL default `maxLines` to 200 when the caller does not provide a value. The response SHALL include a truncation indicator when output is capped.

#### Scenario: memory_get without maxLines on large document
- **WHEN** `memory_get` is called with `{"id": "large-session"}` and no `maxLines` parameter
- **THEN** the response SHALL contain at most 200 lines of the document body
- **THEN** the response SHALL append `\n... (truncated, showing 200 of N total lines. Use maxLines to see more)` if the document exceeds 200 lines

#### Scenario: memory_get with explicit maxLines override
- **WHEN** `memory_get` is called with `{"id": "large-session", "maxLines": 500}`
- **THEN** the response SHALL contain at most 500 lines of the document body

#### Scenario: memory_get on small document
- **WHEN** `memory_get` is called on a document with fewer than 200 lines
- **THEN** the full document body SHALL be returned without truncation indicator

### Requirement: memory_multi_get default maxBytes reduced
The `memory_multi_get` MCP tool SHALL default `maxBytes` to 30000 (reduced from 50000).

#### Scenario: memory_multi_get without maxBytes
- **WHEN** `memory_multi_get` is called with `{"pattern": "session-*"}` and no `maxBytes` parameter
- **THEN** the response SHALL stop accumulating document bodies after 30000 bytes total
- **THEN** the response SHALL include a truncation warning when the limit is reached

### Requirement: code_impact enforces depth and entry limits
The `code_impact` MCP tool response SHALL be truncated to a maximum of 3 depth levels and 50 total entries.

#### Scenario: code_impact on central function with deep dependency tree
- **WHEN** `code_impact` is called for a function with 5 levels of dependents and 200+ total affected symbols
- **THEN** the response SHALL show at most depth levels 1, 2, and 3
- **THEN** the response SHALL show at most 50 total dependency entries
- **THEN** the response SHALL append `... and N more at depth 4+` if deeper levels exist

### Requirement: code_context enforces list caps
The `code_context` MCP tool response SHALL cap callers at 20, callees at 20, and flows at 10.

#### Scenario: code_context on highly-connected symbol
- **WHEN** `code_context` is called for a symbol with 80 callers, 40 callees, and 25 flows
- **THEN** the response SHALL show at most 20 incoming callers with `... and 60 more`
- **THEN** the response SHALL show at most 20 outgoing callees with `... and 20 more`
- **THEN** the response SHALL show at most 10 flows with `... and 15 more`

### Requirement: memory_focus enforces dependency caps
The `memory_focus` MCP tool response SHALL cap dependencies at 30 and dependents at 30.

#### Scenario: memory_focus on central file
- **WHEN** `memory_focus` is called for a file with 100 dependencies and 80 dependents
- **THEN** the response SHALL show at most 30 dependencies with `... and 70 more`
- **THEN** the response SHALL show at most 30 dependents with `... and 50 more`

### Requirement: memory_symbols and memory_impact enforce result caps
The `memory_symbols` and `memory_impact` MCP tool responses SHALL cap results at 50 entries.

#### Scenario: memory_symbols with many matches
- **WHEN** `memory_symbols` is called and finds 120 matching symbols
- **THEN** the response SHALL show at most 50 symbol entries
- **THEN** the response SHALL include `... and 70 more symbols` indicator

### Requirement: code_detect_changes enforces flow cap
The `code_detect_changes` MCP tool response SHALL cap affected flows at 20 (matching existing file and symbol caps).

#### Scenario: code_detect_changes with many affected flows
- **WHEN** `code_detect_changes` detects 45 affected flows
- **THEN** the response SHALL show at most 20 flows with `... and 25 more`

### Requirement: Empty-body documents excluded from embedding queue
The `embedPendingCodebase()` function SHALL skip documents that produce 0 chunks and mark them as processed to prevent infinite retry loops.

#### Scenario: Empty-body document in pending queue
- **WHEN** `embedPendingCodebase()` encounters a document with empty body (0 chunks after chunking)
- **THEN** a sentinel row SHALL be inserted in `content_vectors` (seq=-1) to mark it as processed
- **THEN** the document SHALL NOT appear in subsequent `getHashesNeedingEmbedding()` results

#### Scenario: Embedding cycle with only empty-body documents pending
- **WHEN** all pending documents produce 0 chunks
- **THEN** the embedding cycle SHALL complete without spinning (no retry loop)
- **THEN** the adaptive backoff SHALL correctly detect 0 actual embeddings and slow down

### Requirement: Harvester re-harvest retry limit
The session harvester SHALL stop retrying sessions after 3 consecutive failed re-harvest attempts (output file still missing).

#### Scenario: Session with persistently missing output file
- **WHEN** a session's output file is missing for 3 consecutive harvest cycles
- **THEN** the session SHALL be marked as `skipped` in the harvest state
- **THEN** the session SHALL NOT be re-harvested in subsequent cycles

#### Scenario: Skipped session can be force-re-harvested
- **WHEN** `memory_update` is called manually
- **THEN** skipped sessions SHALL be eligible for re-harvesting (retry counter reset)

### Requirement: npm package contains only runtime files
The `package.json` SHALL include a `"files"` field that whitelists only runtime-necessary files.

#### Scenario: npm pack includes only runtime files
- **WHEN** `npm pack --dry-run` is run
- **THEN** the tarball SHALL NOT contain `test/`, `openspec/`, `site/`, `ai/`, `docs/`, `commands/` directories
- **THEN** the tarball SHALL NOT contain `src/eval/`, `src/bench.ts`
- **THEN** the tarball SHALL contain `src/` (minus exclusions), `bin/`, `.opencode/`, `SKILL.md`, `AGENTS.md`, `AGENTS_SNIPPET.md`, `opencode-mcp.json`
