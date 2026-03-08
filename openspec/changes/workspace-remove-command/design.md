## Context

nano-brain stores workspace data across multiple SQLite tables, all scoped by a `project_hash` column (first 12 chars of SHA-256 of the workspace root path). Each workspace also has:
- A per-workspace SQLite database file at `~/.nano-brain/data/{dirName}-{hash}.sqlite`
- A config entry under `workspaces:` in `~/.nano-brain/config.yml`

The existing `store.clearWorkspace(projectHash)` method only cleans documents, content, content_vectors, vectors_vec, and llm_cache. It does **not** clean file_edges, symbols, code_symbols, symbol_edges, execution_flows, or flow_steps — all of which have `project_hash` columns.

There is no CLI command to remove a workspace. Users must manually delete database files and edit config.yml.

## Goals / Non-Goals

**Goals:**
- Provide a `nano-brain rm <workspace>` command that completely removes all workspace data
- Accept workspace identifier as: absolute path, hash prefix, or workspace name (basename of path)
- Extend `store.clearWorkspace()` to cover all workspace-scoped tables
- Remove the workspace entry from config.yml
- Support `--dry-run` to preview what would be deleted
- Support `nano-brain rm --list` to show all known workspaces before removing

**Non-Goals:**
- Deleting the per-workspace `.sqlite` file (the database is shared across workspaces via the same file when using default paths; deletion is only safe with `init --force --all`)
- Adding an MCP tool for workspace removal (CLI-only for safety)
- Interactive confirmation prompts (keep it scriptable; `--dry-run` serves the preview role)

## Decisions

### 1. Workspace identifier resolution

**Decision**: Accept three forms of workspace identifier and resolve in order:
1. **Exact path** — if the argument looks like an absolute path, compute its hash directly
2. **Hash prefix** — if it matches `[a-f0-9]{4,12}`, search workspace stats for a matching prefix
3. **Name match** — search config.yml workspace entries where `basename(path)` matches the argument

**Why**: Users see workspace hashes in `status` output and workspace names in config. Supporting all three avoids friction. The resolution order prevents ambiguity (paths are unambiguous, hashes are near-unique, names may collide).

**Alternative considered**: Only accept exact paths — rejected because users commonly see hash prefixes in status output and would need to reverse-lookup the path.

### 2. Extended clearWorkspace

**Decision**: Add a new `removeWorkspace(projectHash)` method to the store that deletes from ALL workspace-scoped tables in a single transaction:
- `flow_steps` (via cascade from execution_flows, but explicit for safety)
- `execution_flows` WHERE project_hash = ?
- `symbol_edges` WHERE project_hash = ?
- `code_symbols` WHERE project_hash = ?
- `symbols` WHERE project_hash = ?
- `file_edges` WHERE project_hash = ?
- Then call existing `clearWorkspace()` logic for documents, embeddings, content, cache

**Why**: A single transaction ensures atomicity. Ordering respects foreign key constraints (children before parents). Separating from `clearWorkspace()` avoids breaking `init --force` which intentionally only clears document-level data before re-indexing.

**Alternative considered**: Extending `clearWorkspace()` directly — rejected because `init --force` uses it and re-indexes immediately after; deleting code_symbols/edges before re-indexing is the expected behavior there, but the method name and existing callers assume document-scope only.

### 3. Config cleanup

**Decision**: After database cleanup, remove the workspace entry from `config.yml` using the existing `loadCollectionConfig`/`saveCollectionConfig` functions. Delete the key from `config.workspaces` that matches the resolved workspace root path.

**Why**: Leaving stale config entries causes confusion in `status --all` output.

### 4. Dry-run output

**Decision**: `--dry-run` queries row counts from each workspace-scoped table and prints a summary without deleting anything.

**Why**: Gives users confidence before destructive operations. Cheap to implement since we just need COUNT queries.

## Risks / Trade-offs

- **[Risk] Name collision**: Two workspaces could have the same basename (e.g., two `api/` directories). → **Mitigation**: If multiple matches found, print all matches with their full paths and ask user to be more specific (use path or hash).
- **[Risk] Shared database file**: Multiple workspaces may share the same `.sqlite` file. Deleting the file would destroy other workspaces' data. → **Mitigation**: Never delete the database file; only delete rows scoped to the target project_hash.
- **[Risk] Stale vector store entries in Qdrant**: If using Qdrant as vector provider, workspace vectors exist in an external store. → **Mitigation**: Out of scope for v1. Document as known limitation. The sqlite-vec vectors are handled by the transaction.
