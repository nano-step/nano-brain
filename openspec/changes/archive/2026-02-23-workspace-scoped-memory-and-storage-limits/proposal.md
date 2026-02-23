## Why

All sessions from every workspace are harvested into a single flat index. Searching in project A returns results from unrelated projects B, C, D — polluting context and wasting tokens. Additionally, storage grows unbounded: a heavy user accumulates GBs of sessions over months with no eviction, eventually hitting OOM or disk-full crashes. Both problems must be solved before the auto-persistence pipeline is production-ready.

## What Changes

- **Workspace-scoped search by default**: The MCP server detects the current workspace from `PWD` at startup, computes its projectHash, and filters all search results to that workspace. Cross-workspace search remains available via explicit parameter.
- **Per-workspace session collections**: Harvested sessions are organized by projectHash (already the case on disk). The indexer tags each document with its projectHash so filtering is efficient at the database level.
- **Configurable storage limits**: New `storage` section in `config.yml` with `maxSize`, `retention`, and `minFreeDisk` options.
- **Automatic eviction**: When storage exceeds `maxSize`, oldest sessions are evicted first (by date). Sessions older than `retention` period are cleaned up on each harvest cycle.
- **Disk safety guard**: Before any write operation (harvest, reindex, embed), check available disk space. If below `minFreeDisk`, stop all writes and log a warning.
- **Global memory preserved**: `MEMORY.md` and daily logs remain unscoped — they are the user's personal cross-project notes.

## Capabilities

### New Capabilities
- `workspace-scoping`: Workspace detection from PWD, projectHash computation, per-workspace search filtering, cross-workspace search opt-in, document-level project tagging
- `storage-limits`: Configurable maxSize/retention/minFreeDisk, automatic eviction of oldest sessions, disk space safety checks, storage config parsing and validation

### Modified Capabilities
- `mcp-server`: Search tools (`memory_search`, `memory_vsearch`, `memory_query`) gain default workspace filtering and optional cross-workspace parameter. `memory_status` reports per-workspace and total storage usage.

## Impact

- **Config schema**: New `storage` section in `config.yml` (backward-compatible — all fields optional with safe defaults)
- **Database schema**: Documents table needs a `project_hash` column (migration required for existing DBs)
- **MCP tool API**: Search tools gain optional `workspace` parameter (`"current"` default, `"all"` for cross-workspace). Non-breaking — existing calls work unchanged.
- **Files affected**: `src/server.ts`, `src/store.ts`, `src/watcher.ts`, `src/harvester.ts`, `src/collections.ts`, `src/types.ts`, `config.yml` schema
- **Disk I/O**: Eviction adds periodic delete operations. Disk space check adds `statfs` call before writes.
- **No new dependencies required**
