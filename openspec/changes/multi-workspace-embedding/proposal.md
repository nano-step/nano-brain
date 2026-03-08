## Why

When nano-brain runs as a remote SSE daemon via `npx nano-brain serve`, a single process serves all connected OpenCode sessions. However, the background embedding interval only processes pending documents for the workspace determined at startup (from cwd). Other configured workspaces accumulate pending embeddings indefinitely — currently 4,741 pending docs across 5 workspaces that will never be embedded until a local MCP instance is spawned in each directory.

This defeats the purpose of running nano-brain on the host to reduce container resource usage — if embeddings still require per-workspace local instances, the consolidation is incomplete.

## What Changes

- The background embedding interval iterates ALL configured workspaces from `config.yml` instead of only the current workspace.
- For each workspace, the server temporarily opens (or reuses) that workspace's SQLite DB, checks for pending embeddings, processes them, then moves on.
- The file watcher is extended to watch codebase directories for all configured workspaces, not just the current one.
- Session harvesting runs across all workspaces during each harvest cycle.

## Capabilities

### New Capabilities
- `multi-workspace-embed`: Background embedding interval processes pending documents across all configured workspaces in `config.yml`, not just the startup workspace.

### Modified Capabilities
- `workspace-scoping`: The server's workspace resolution must support operating on multiple workspace DBs within a single process, rather than binding to one at startup.

## Impact

- `src/server.ts` — embed interval, harvest interval, file watcher setup
- `src/store.ts` — may need a store pool or factory to open/close per-workspace DBs
- `src/codebase.ts` — `embedPendingCodebase()` called per workspace
- `src/watcher.ts` — watch directories for all configured workspaces
- `src/collections.ts` — `getWorkspaceConfig()` used to iterate workspaces
- No new dependencies. No breaking changes. No API changes to MCP tools.
