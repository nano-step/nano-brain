## Why

There is no way to cleanly remove a workspace from nano-brain. The closest option is `init --force` which clears and re-initializes, or manual deletion of `.sqlite` files and config entries. Users need a simple `nano-brain rm <workspace>` command to completely remove all data associated with a workspace — database records, config entries, and optionally the per-workspace database file itself.

## What Changes

- Add a new `rm` CLI command that accepts a workspace identifier (path or hash prefix) and removes all associated data
- Extend `store.clearWorkspace()` to also delete file_edges, symbols, code_symbols, symbol_edges, execution_flows, and flow_steps scoped to the workspace (currently it only handles documents, embeddings, content, and cache)
- Remove the workspace entry from `config.yml` under the `workspaces:` key
- Add a `--dry-run` flag to preview what would be deleted without actually deleting
- Add a `list` subcommand or integrate with `status --all` so users can see available workspaces before removing

## Capabilities

### New Capabilities
- `workspace-remove`: CLI command to identify and completely remove a workspace — database records (documents, embeddings, content, cache, file_edges, symbols, code_symbols, symbol_edges, execution_flows, flow_steps), config entry, and optionally the per-workspace database file

### Modified Capabilities
- `workspace-scoping`: The existing `clearWorkspace` store method needs to be extended to cover all workspace-scoped tables (file_edges, symbols, code_symbols, symbol_edges, execution_flows, flow_steps) that were added after the original implementation

## Impact

- **Code**: `src/index.ts` (new CLI handler), `src/store.ts` (extended clearWorkspace), `src/collections.ts` (workspace config removal)
- **Config**: `~/.nano-brain/config.yml` — workspace entries will be removed
- **Database**: All workspace-scoped rows across all tables will be deleted; per-workspace `.sqlite` files may be deleted
- **No breaking changes**: This is a new additive command. Existing commands are unaffected.
