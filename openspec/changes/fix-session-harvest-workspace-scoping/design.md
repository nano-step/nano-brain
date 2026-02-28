## Context

The `workspace-scoped-memory-and-storage-limits` change (archived 2026-02-23) introduced workspace-scoped search by adding a `project_hash` column to the `documents` table and filtering search results by `currentProjectHash`. The spec requires that session documents be tagged with the projectHash **extracted from their file path** (`sessions/{hash}/*.md`).

However, the implementation has a bug in four code paths where session documents are indexed with the **wrong** projectHash:

1. **`watcher.ts` → `triggerReindex()`** (line 122): Passes the watcher's own `projectHash` (current workspace) to `indexDocument()` for ALL collection files, including session files from other workspaces.
2. **`index.ts` → `handleInit()`** (line 429): Indexes session collection files with no `projectHash`, defaulting to `undefined` → `'global'`.
3. **`index.ts` → `handleUpdate()`** (line 539): Same issue as `handleInit`.
4. **`server.ts` → `memory_update` tool** (line 460): Indexes all collection files without projectHash extraction.

The session files on disk are correctly organized by projectHash (`~/.nano-brain/sessions/{projectHash}/*.md`), and the harvester correctly writes them there. The bug is purely in the indexing step that reads these files into the database.

## Goals / Non-Goals

**Goals:**
- Fix all four indexing code paths to extract projectHash from session file paths
- Create a shared utility function for projectHash extraction
- Ensure existing incorrectly-tagged documents get re-tagged on next reindex
- Maintain backward compatibility — no API or config changes

**Non-Goals:**
- Changing the harvester itself (it already works correctly)
- Modifying the search layer (already correctly filters by projectHash)
- Adding new MCP tools or parameters
- Changing the session file format or directory structure

## Decisions

### Decision 1: Shared `extractProjectHashFromPath()` utility

**Choice**: Create a function in `store.ts` (or a new `utils.ts`) that extracts projectHash from a file path.

```typescript
export function extractProjectHashFromPath(filePath: string, sessionsDir: string): string | undefined {
  // If filePath is under sessionsDir, extract the projectHash from the subdirectory name
  // e.g., ~/.nano-brain/sessions/abc123def456/2026-02-16-session.md → 'abc123def456'
  // Returns undefined for non-session files (caller defaults to their own projectHash or 'global')
}
```

**Rationale**: All four bug sites need the same logic. A shared function prevents divergence and is easy to test.

**Alternative considered**: Parsing YAML frontmatter from session files to read `projectHash`. Rejected because:
- Slower (requires reading and parsing file content)
- The directory structure is the canonical source of truth (set by the harvester)
- Frontmatter could be missing or malformed

### Decision 2: Collection-aware indexing in watcher

**Choice**: In `triggerReindex()`, check if the collection being indexed is `sessions`. If so, extract projectHash from each file's path. Otherwise, use the watcher's own `projectHash`.

```typescript
for (const filePath of files) {
  const effectiveProjectHash = collection.name === 'sessions'
    ? extractProjectHashFromPath(filePath, outputDir) ?? projectHash
    : projectHash;
  indexDocument(store, collection.name, filePath, content, title, effectiveProjectHash);
}
```

**Rationale**: Only session files have per-project scoping. Memory files, codebase files, and custom collections should continue using the watcher's projectHash.

**Alternative considered**: Always extracting from path for all collections. Rejected because non-session collections don't have projectHash in their directory structure.

### Decision 3: Self-healing via reindex

**Choice**: No explicit migration for existing incorrectly-tagged documents. The fix naturally corrects tags on the next reindex cycle because:
- The watcher periodically reindexes all collections
- `indexDocument()` uses `INSERT OR REPLACE` (UPSERT), so re-indexing a file updates its `project_hash`
- Running `memory_update` or restarting the MCP server triggers a full reindex

**Rationale**: Simpler than writing a one-time migration. The data self-heals within one reindex cycle (typically < 2 minutes after server restart).

**Alternative considered**: Adding a startup migration that scans all session documents and fixes their `project_hash`. Rejected because:
- Adds complexity for a one-time fix
- The reindex already handles it
- Users can trigger immediate fix via `memory_update` tool

## Risks / Trade-offs

- **[Risk] Existing sessions tagged with wrong projectHash until reindex** → Mitigation: First reindex after the fix corrects all tags. Users can force immediate reindex via `memory_update`. Document this in release notes.
- **[Risk] Collection name `sessions` is hardcoded in the check** → Mitigation: The sessions collection name is already hardcoded in `handleInit()` and is a core convention. If collection naming changes, this would need updating — but that's a broader refactor.
- **[Risk] Path parsing assumes `sessions/{hash}/` directory structure** → Mitigation: The harvester is the only writer to this directory, and it always uses this structure. The extraction function validates the hash format (12-char hex) as a safety check.
