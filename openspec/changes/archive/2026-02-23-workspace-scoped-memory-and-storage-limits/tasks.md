## Tasks

- [x] **Task 1**: Add project_hash column and migration to store.ts
**Spec**: workspace-scoping/spec.md — Database migration for existing documents, Document-level project tagging
**Files**: `src/store.ts`, `src/types.ts`

**Steps**:
1. Add `projectHash?: string` field to the `Document` interface in `types.ts`
2. In `createStore()`, after the initial `db.exec()` schema creation, add migration logic:
   - Check if `project_hash` column exists: `PRAGMA table_info(documents)`
   - If missing: `ALTER TABLE documents ADD COLUMN project_hash TEXT DEFAULT 'global'`
   - Backfill existing rows: `UPDATE documents SET project_hash = ... WHERE path LIKE 'sessions/%'` — extract hash from path using `substr(path, instr(path, 'sessions/') + 9, 12)` pattern
3. Add `CREATE INDEX IF NOT EXISTS idx_documents_project_hash ON documents(project_hash, active)` for efficient filtering
4. Update `insertDocumentStmt` to include `project_hash` column
5. Update `insertDocument()` method to accept and store `projectHash`
6. Add `projectHash` to `findDocument()` return mapping

**Acceptance**: Migration runs on first startup, backfills correctly, subsequent startups skip migration. New documents get project_hash set.

---

- [x] **Task 2**: Add workspace-filtered search to store.ts
**Spec**: workspace-scoping/spec.md — Default search scoping, Cross-workspace search opt-in
**Files**: `src/store.ts`, `src/types.ts`

**Steps**:
1. Add `projectHash?: string` parameter to `searchFTS()` and `searchVec()` in the `Store` interface
2. Create new prepared statements for workspace-filtered FTS search:
   - `searchFTSWithWorkspaceStmt`: filters `d.project_hash IN (?, 'global')` in addition to existing conditions
   - `searchFTSWithWorkspaceAndCollectionStmt`: combines both workspace and collection filters
3. Update `searchFTS()` implementation: when `projectHash` is provided and not `'all'`, use workspace-filtered statement
4. Update `searchVec()` implementation: add `AND d.project_hash IN (?, 'global')` to the dynamic SQL when `projectHash` is provided and not `'all'`
5. Update `Store` interface signatures in `types.ts`

**Acceptance**: `searchFTS('query', 10, undefined, 'abc123')` returns only docs with `project_hash = 'abc123'` or `'global'`. `searchFTS('query', 10, undefined, 'all')` returns all docs. `searchFTS('query', 10)` (no projectHash) returns all docs (backward compatible).

---

- [x] **Task 3**: Compute currentProjectHash in server.ts and wire to search tools
**Spec**: workspace-scoping/spec.md — Workspace detection from PWD; mcp-server/spec.md (delta) — Search tools support workspace filtering
**Files**: `src/server.ts`

**Steps**:
1. In `startServer()`, compute `currentProjectHash = crypto.createHash('sha256').update(process.cwd()).digest('hex').substring(0, 12)`
2. Add `currentProjectHash` to `ServerDeps` interface
3. In `createMcpServer()`, add `workspace` parameter (optional string) to `memory_search`, `memory_vsearch`, `memory_query` tool schemas
4. In each search tool handler:
   - Resolve effective workspace: `workspace === 'all' ? 'all' : (workspace || deps.currentProjectHash)`
   - Pass resolved workspace to `store.searchFTS()` / `store.searchVec()` / `hybridSearch()`
5. Update `hybridSearch()` in `search.ts` to accept and pass through `projectHash` parameter

**Acceptance**: Search tools default to current workspace. `workspace: "all"` searches everything. Tool schemas show `workspace` parameter.

---

- [x] **Task 4**: Update memory_status to report workspace and storage info
**Spec**: mcp-server/spec.md (delta) — memory_status reports storage usage
**Files**: `src/server.ts`, `src/store.ts`, `src/types.ts`

**Steps**:
1. Add `getWorkspaceStats()` method to store: `SELECT project_hash, COUNT(*) as count FROM documents WHERE active = 1 GROUP BY project_hash`
2. Add workspace stats and storage config to `IndexHealth` interface
3. Update `formatStatus()` to include per-workspace document counts and storage limit info
4. In `memory_status` handler, pass storage config to format function

**Acceptance**: `memory_status` output shows per-workspace breakdown and storage limits.

---

- [x] **Task 5**: Add storage config parsing to types.ts and collections.ts
**Spec**: storage-limits/spec.md — Storage configuration with safe defaults, Human-readable size and duration parsing
**Files**: `src/types.ts`, `src/collections.ts`

**Steps**:
1. Add `StorageConfig` interface to `types.ts`: `{ maxSize: number; retention: number; minFreeDisk: number }`
2. Add `storage?` field to `CollectionConfig` interface: `{ maxSize?: string; retention?: string; minFreeDisk?: string }`
3. Create `parseSize(value: string): number` function — parses `500MB`, `2GB`, `1TB` to bytes
4. Create `parseDuration(value: string): number` function — parses `30d`, `90d`, `1y` to milliseconds
5. Create `parseStorageConfig(raw?: { maxSize?: string; retention?: string; minFreeDisk?: string }): StorageConfig` — applies defaults and validates
6. Export from `collections.ts` (or create new `src/storage.ts` if cleaner)

**Acceptance**: `parseSize('2GB')` returns `2147483648`. `parseDuration('90d')` returns `7776000000`. Invalid values log warning and return defaults.

---

- [x] **Task 6**: Implement disk safety guard
**Spec**: storage-limits/spec.md — Disk safety guard
**Files**: `src/storage.ts` (new) or `src/watcher.ts`

**Steps**:
1. Create `checkDiskSpace(minFreeDisk: number): { ok: boolean; freeBytes: number }` function
2. Use `fs.statfsSync()` (Node 18.15+) on the output directory to get available space
3. Wrap in try/catch — if `statfsSync` unavailable, log warning and return `{ ok: true, freeBytes: -1 }`
4. Integrate into watcher's `triggerReindex()`: check disk before harvest/reindex/embed operations
5. If disk check fails, skip all write operations and log warning

**Acceptance**: When disk is below `minFreeDisk`, writes are skipped with warning. When `statfsSync` unavailable, operations proceed with warning.

---

- [x] **Task 7**: Implement retention and size-based eviction
**Spec**: storage-limits/spec.md — Retention-based eviction, Size-based eviction, Original session JSON is never deleted
**Files**: `src/storage.ts` (new), `src/watcher.ts`, `src/store.ts`

**Steps**:
1. Create `evictExpiredSessions(sessionsDir: string, retention: number, store: Store): number` function:
   - Scan all `sessions/{hash}/*.md` files
   - Check mtime against `Date.now() - retention`
   - Delete expired files and remove corresponding documents from store
   - Return count of evicted files
2. Create `evictBySize(sessionsDir: string, dbPath: string, maxSize: number, store: Store): number` function:
   - Calculate total size: `statSync(dbPath).size` + recursive dir size of `sessionsDir`
   - If over `maxSize`, collect all session files sorted by mtime (oldest first)
   - Delete oldest files one by one until under limit
   - Remove corresponding documents from store
   - Return count of evicted files
3. Add `deleteDocumentsByPath(pathPattern: string): number` method to store for removing evicted documents
4. Integrate eviction into watcher's harvest cycle: run after harvest, before reindex
5. Never touch files outside `sessionsDir` (original JSON in `~/.local/share/opencode/` is safe)

**Acceptance**: Sessions older than retention are evicted. If still over maxSize, oldest are evicted. Original JSON untouched. Eviction count logged.

---

- [x] **Task 8**: Implement orphan embedding cleanup
**Spec**: storage-limits/spec.md — Orphan embedding cleanup
**Files**: `src/store.ts`, `src/watcher.ts`

**Steps**:
1. Add `cleanOrphanedEmbeddings(): number` method to store:
   - `DELETE FROM content_vectors WHERE hash NOT IN (SELECT hash FROM documents WHERE active = 1)`
   - If vec table exists: `DELETE FROM vectors_vec WHERE substr(hash_seq, 1, instr(hash_seq, ':') - 1) NOT IN (SELECT hash FROM documents WHERE active = 1)`
   - Return count of deleted rows
2. Add cycle counter to watcher — every 10 harvest cycles, call `cleanOrphanedEmbeddings()`
3. Log cleanup results

**Acceptance**: Orphaned embeddings are cleaned every 10 cycles. No active document embeddings are deleted.

---

- [x] **Task 9**: Wire storage config through server startup
**Spec**: storage-limits/spec.md — Storage configuration with safe defaults
**Files**: `src/server.ts`, `src/watcher.ts`

**Steps**:
1. In `startServer()`, parse storage config from loaded collection config
2. Pass `StorageConfig` to watcher via `WatcherOptions`
3. Watcher uses storage config for disk check, eviction thresholds
4. Pass storage config to `memory_status` for display

**Acceptance**: Storage config from `config.yml` is loaded and used. Missing config uses defaults. Status tool shows config values.

---

- [x] **Task 10**: Add tests for workspace scoping
**Spec**: workspace-scoping/spec.md — all requirements
**Files**: `test/store.test.ts` or new `test/workspace.test.ts`

**Steps**:
1. Test migration: create store, verify `project_hash` column exists
2. Test document tagging: index documents with session paths, verify `project_hash` extracted correctly
3. Test non-session documents get `project_hash = 'global'`
4. Test `searchFTS` with workspace filter returns only matching + global docs
5. Test `searchFTS` with `'all'` returns everything
6. Test `searchVec` with workspace filter (if vec available)
7. Test `currentProjectHash` computation matches harvester convention

**Acceptance**: All new tests pass. Existing 265 tests still pass.

---

- [x] **Task 11**: Add tests for storage limits
**Spec**: storage-limits/spec.md — all requirements
**Files**: `test/storage.test.ts` (new)

**Steps**:
1. Test `parseSize()`: valid sizes, invalid input, edge cases
2. Test `parseDuration()`: valid durations, invalid input
3. Test `parseStorageConfig()`: full config, partial config, empty config
4. Test retention eviction: create temp files with old mtimes, verify eviction
5. Test size eviction: create files exceeding maxSize, verify oldest evicted first
6. Test disk safety guard: mock `statfsSync` to test both paths
7. Test orphan cleanup: create orphaned embeddings, verify cleanup

**Acceptance**: All new tests pass. Existing tests still pass.

---

- [x] **Task 12**: Integration test for workspace-scoped search via MCP tools
**Spec**: mcp-server/spec.md (delta) — Search tool parameter schema includes workspace
**Files**: `test/server.test.ts`

**Steps**:
1. Add test: index documents with different `project_hash` values
2. Test `memory_search` without workspace param returns only current workspace + global
3. Test `memory_search` with `workspace: "all"` returns all
4. Test `memory_status` includes workspace breakdown

**Acceptance**: Integration tests verify end-to-end workspace scoping through MCP tool handlers.
