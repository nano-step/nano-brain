## Why

The server creates a SQLite database for every `--root` path it receives at startup, regardless of whether that path is declared in `config.workspaces`. This causes orphaned databases to accumulate in `~/.nano-brain/data/` (observed: 9 DBs, ~2.5GB), and stale DBs with mismatched vector dimensions silently corrupt search results.

## What Changes

- **New**: `src/server/bootstrap.ts` validates `effectiveWorkspaceRoot` against `config.workspaces` before calling `createStore()`
- **New**: When `--root` is not in configured workspaces, server logs a warning and falls back to the closest matching configured workspace instead of creating a new DB
- **New**: `workspace-config-guard` capability — the enforcement logic is extracted as a reusable guard function
- **Backward compatible**: If `config.workspaces` is empty/absent, current behavior is preserved (no restriction)

## Capabilities

### New Capabilities

- `workspace-config-guard`: Validates a workspace root path against `config.workspaces` at server startup. Returns the effective workspace to use (closest match from configured list) and whether a fallback occurred. Emits a structured warning log when fallback is triggered. No-ops when `config.workspaces` is empty.

### Modified Capabilities

- `workspace-scoping`: Adds the constraint that `createStore()` is only called for workspaces present in `config.workspaces` (when the config has any workspaces declared). Previously, workspace scoping only applied to search filtering — now it also governs DB creation.

## Impact

- **`src/server/bootstrap.ts`**: Primary change site — add guard call after config load, before `createStore()`
- **`bin/cli.js`**: Check if it calls `createStore()` directly; apply same guard if so
- **`src/config.ts`** / **`config.default.yml`**: No schema changes needed; `workspaces` key already exists
- **No API changes**: MCP tools and CLI commands are unaffected
- **No migration**: Existing orphaned DBs are not removed (cleanup is a separate concern, tracked in a future issue)
