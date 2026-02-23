## Context

nano-brain is an MCP server providing persistent memory across OpenCode sessions. It harvests session JSON from `~/.local/share/opencode/storage/`, converts to markdown, indexes into SQLite (FTS5 + sqlite-vec), and exposes search via MCP tools.

Current state:
- **956 sessions** across 7 workspaces are mixed into one flat index (30MB SQLite DB)
- **No workspace awareness** — searching in project A returns results from projects B, C, D
- **No storage limits** — DB grows unbounded, no eviction, no disk safety checks
- The MCP server is launched per-workspace by OpenCode with `PWD` set to the workspace root
- Harvested sessions are already organized by projectHash on disk (`sessions/{hash}/*.md`)
- The `documents` table has a `collection` column but no `project_hash` column

Key constraint: The MCP server runs as a stdio process spawned by OpenCode. It knows `PWD` but receives no explicit workspace identifier.

## Goals / Non-Goals

**Goals:**
- Search results scoped to current workspace by default
- Cross-workspace search available via explicit opt-in
- Configurable storage cap with automatic eviction of oldest sessions
- Disk safety guard preventing writes when disk is critically low
- Backward-compatible config (all new fields optional with safe defaults)
- Zero data loss for recent sessions during eviction

**Non-Goals:**
- Per-workspace separate SQLite databases (too complex, breaks cross-workspace search)
- Real-time workspace switching within a session (server restarts on workspace change anyway)
- Compression or deduplication of session content
- Cloud sync or backup of memory data
- Embedding eviction (only session documents are evicted, embeddings are orphaned and cleaned lazily)

## Decisions

### D1: Workspace detection via PWD + SHA-256 hash

**Decision**: Compute `projectHash = sha256(process.cwd()).substring(0, 12)` at server startup. Store as `currentProjectHash` on the server context.

**Why**: OpenCode already sets `PWD` to the workspace root when spawning MCP servers. The harvester already uses the same `sha256(directory).substring(0, 12)` scheme for organizing session output directories. Reusing this ensures consistency.

**Alternative considered**: Use an explicit `--workspace` CLI flag. Rejected because it requires config changes in every OpenCode installation and the PWD approach works automatically.

### D2: Document-level project_hash column in SQLite

**Decision**: Add a `project_hash TEXT` column to the `documents` table. Populate it during indexing by extracting the projectHash from the file path (sessions are stored at `sessions/{projectHash}/*.md`). For non-session documents (MEMORY.md, daily logs), set `project_hash = 'global'`.

**Why**: Column-level filtering is fast (indexed), works with existing FTS5 queries, and doesn't require restructuring collections.

**Alternative considered**: Separate collections per workspace. Rejected because it would require dynamic collection creation and complicate the config.

**Migration**: `ALTER TABLE documents ADD COLUMN project_hash TEXT DEFAULT 'global'`. Then backfill from file paths for existing documents.

### D3: Search filtering with workspace parameter

**Decision**: All search MCP tools (`memory_search`, `memory_vsearch`, `memory_query`) gain an optional `workspace` parameter:
- Default: `undefined` → filter to `currentProjectHash` + `'global'` documents
- `"all"` → no filtering, search everything
- `"<specific-hash>"` → filter to that project

**Why**: Default scoping prevents cross-project pollution. The `"all"` escape hatch preserves the ability to search across workspaces when needed. Including `'global'` in default ensures MEMORY.md and daily logs are always searchable.

### D4: Storage config with safe defaults

**Decision**: New `storage` section in `config.yml`:
```yaml
storage:
  maxSize: 2GB          # Max total size (DB + sessions dir)
  retention: 90d        # Auto-evict sessions older than this
  minFreeDisk: 100MB    # Stop writes if disk free below this
```

All fields optional. Defaults: `maxSize: 2GB`, `retention: 90d`, `minFreeDisk: 100MB`.

**Why**: These defaults are safe for most users. 2GB accommodates ~100K sessions. 90 days keeps recent context. 100MB prevents disk-full crashes.

**Parsing**: `maxSize` and `minFreeDisk` accept human-readable sizes (`500MB`, `2GB`, `1TB`). `retention` accepts duration strings (`30d`, `90d`, `1y`).

### D5: Eviction strategy — oldest sessions first

**Decision**: Eviction runs during the harvest cycle (every 2 minutes). Order of operations:
1. **Retention eviction**: Delete session markdown files older than `retention` period. Remove corresponding documents from SQLite.
2. **Size eviction**: If total size still exceeds `maxSize`, delete oldest remaining sessions until under limit.
3. **Orphan cleanup**: Periodically (every 10 cycles) remove orphaned embeddings whose documents no longer exist.

**Why**: Oldest-first is simple, predictable, and preserves the most relevant (recent) context. Running during harvest avoids adding a separate eviction timer.

**Alternative considered**: LRU (least recently accessed). Rejected because tracking access timestamps adds complexity and the access pattern (search) doesn't map cleanly to document-level access.

### D6: Disk safety guard via statfs

**Decision**: Before any write operation (harvest, reindex, embed), call `os.statfs()` (Node 18+) or `child_process.execSync('df')` to check available disk space. If below `minFreeDisk`, skip the write and log `[storage] Disk space critically low (<100MB free), skipping writes`.

**Why**: Prevents the most catastrophic failure mode (disk full → SQLite corruption). The check is cheap (~1ms) and runs at most every 2 minutes.

**Fallback**: If `statfs` is unavailable (older Node), skip the check and log a warning.

## Risks / Trade-offs

**[Risk] Existing documents lack project_hash** → Migration backfills from file paths. Documents not matching `sessions/{hash}/*.md` pattern get `project_hash = 'global'`. No data loss.

**[Risk] PWD might not match session directory** → The session's `directory` field in JSON is the original workspace path. PWD is the current workspace. These should match for the current project's sessions. For harvested sessions from other projects, the projectHash from the file path is authoritative.

**[Risk] Eviction deletes data permanently** → Eviction only removes harvested markdown and index entries. The original OpenCode session JSON in `~/.local/share/opencode/storage/` is never touched. Sessions can be re-harvested if needed.

**[Risk] Size calculation is approximate** → Checking DB file size + sessions dir size via `fs.statSync` is fast but doesn't account for WAL files or pending transactions. Acceptable for a soft limit.

**[Risk] statfs not available in all environments** → Docker containers and some Node versions may not support `os.statfs`. Fallback: skip the check, rely on size-based eviction only.

## Open Questions

- Should evicted sessions be logged somewhere (e.g., `eviction.log`) for auditability?
- Should there be a `memory_evict` MCP tool for manual eviction, or is automatic-only sufficient?
