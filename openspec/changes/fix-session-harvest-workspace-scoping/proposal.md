## Why

Session harvesting collects ALL sessions from every workspace into a single `~/.nano-brain/sessions/` directory, and the watcher's reindex stamps every session document with the **current workspace's** `projectHash` instead of extracting it from the session file's actual path. This means if nano-brain runs in workspace A, sessions from workspaces B, C, D all get tagged as belonging to A — defeating the workspace-scoped search that was implemented in the previous `workspace-scoped-memory-and-storage-limits` change. The result: searching in any project returns sessions from all projects, polluting context and wasting tokens.

## What Changes

- **Fix projectHash extraction during session indexing**: The watcher's `triggerReindex()` currently passes its own `projectHash` to `indexDocument()` for ALL files including session files. For the `sessions` collection, the projectHash must be extracted from the file's directory structure (`sessions/{projectHash}/*.md`) instead of using the watcher's workspace hash.
- **Fix `handleInit` session indexing**: The `init` command indexes session collection files with no projectHash, defaulting to `'global'`. It should also extract projectHash from the file path.
- **Fix `handleUpdate` session indexing**: Same issue — indexes all collection files without workspace-aware projectHash extraction.
- **Fix `memory_update` tool**: The MCP tool's reindex handler indexes all collection files without extracting projectHash from session paths.
- **Add projectHash extraction utility**: Create a shared helper that extracts projectHash from a session file path by matching the `sessions/{hash}/` directory pattern, returning `'global'` for non-session files.

## Capabilities

### New Capabilities

### Modified Capabilities
- `workspace-scoping`: The "Document-level project tagging" requirement is already specified correctly (extract from file path), but the implementation violates it. This change fixes the implementation to match the spec. No spec text changes needed — only implementation fixes.

## Impact

- **Files affected**: `src/watcher.ts` (triggerReindex projectHash logic), `src/index.ts` (handleInit, handleUpdate session indexing), `src/server.ts` (memory_update tool), `src/store.ts` or new utility (projectHash extraction helper)
- **Database**: Existing documents with incorrect `project_hash` values need re-tagging. A one-time migration or re-harvest will fix stale data.
- **No API changes**: MCP tool interfaces remain unchanged.
- **No new dependencies**: Pure logic fix using existing path parsing.
- **Risk**: Low — this is a bug fix aligning implementation with existing spec. All search tools already support workspace filtering; they just receive wrong data today.
