## Why

Debugging workflow requires searching code + sessions + config in parallel, but agents currently have to call 3+ tools manually and synthesize results. This slows down debugging and often misses important context. A debugging-aware search mode and agent skill would let agents find debugging context in one call.

## What Changes

- **New MCP parameter**: `memory_search` and `memory_query` gain a `mode` parameter
  - `mode=debugging` runs parallel searches across code, sessions, and config
  - Returns merged results with source labels (`code`, `session`, `config`)
  - Default behavior unchanged when `mode` is omitted

- **New debugging skill**: teaches agents the optimal debugging workflow
  - Detects debugging intent from user queries
  - Suggests tool sequence for debugging scenarios
  - No server changes — agent-level guidance only

## Capabilities

### New Capabilities
- `debugging-search-mode`: Parallel code + sessions + config search with source labels
- `debugging-skill`: Agent skill that teaches debugging workflow with nano-brain tools

### Modified Capabilities
- `search-pipeline`: New `mode` parameter on memory_search/memory_query (non-breaking addition)

## Impact

- `internal/mcp/tools.go` — add `mode` parameter to memory_search/memory_query
- `internal/search/service.go` — add parallel search orchestration for debugging mode
- `internal/server/handlers/` — no changes (MCP handles it)
- `benchmarks/llm-quality/run_debug.sh` — re-benchmark after implementation
- `.opencode/skills/` — new debugging skill file
- No breaking changes — `mode` is optional, defaults to current behavior
