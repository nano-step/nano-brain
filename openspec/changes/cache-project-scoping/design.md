## Context

nano-brain's `llm_cache` table is a flat key-value store (`hash TEXT PRIMARY KEY, result TEXT, created_at TEXT`) used by three cache types: query embedding vectors, query expansion variants, and reranking scores. All entries share one namespace with no workspace isolation. The store serves multiple workspaces simultaneously via the MCP server, each identified by a `projectHash` (first 12 chars of SHA-256 of workspace root path).

Current cache callers:
- `search.ts` — expansion cache (`getCachedResult`/`setCachedResult`), reranking cache (same), embedding cache (`getQueryEmbeddingCache`/`setQueryEmbeddingCache`)
- `server.ts` — embedding cache in `memory_vsearch` tool
- `store.ts` — `clearQueryEmbeddingCache()` does `DELETE FROM llm_cache`

## Goals / Non-Goals

**Goals:**
- Expansion and reranking cache entries are isolated per workspace
- Query embedding cache remains global (text→vector is project-independent)
- `clearQueryEmbeddingCache()` only deletes embedding-type entries, not expansion/reranking
- CLI `cache clear` scoped to current workspace by default, `--all` for global wipe
- CLI `cache stats` shows entry counts by type and workspace
- Backward-compatible migration: existing cache entries get `project_hash='global'` and `type='general'`

**Non-Goals:**
- Cache TTL or eviction policies (future work)
- Per-collection cache scoping (workspace-level is sufficient)
- Changing the cache key hashing strategy

## Decisions

### 1. Add columns to existing table vs. separate tables

**Decision**: Add `project_hash TEXT` and `type TEXT` columns to `llm_cache`.

**Alternatives considered**:
- Separate tables (`embedding_cache`, `expansion_cache`, `rerank_cache`) — cleaner separation but triples the prepared statements and requires more migration work. The cache access pattern is identical across types (get by hash, set by hash), so a single table with type discrimination is simpler.

**Rationale**: Single table with discriminator columns keeps the code DRY. The `type` column enables selective clearing. The `project_hash` column enables workspace isolation.

### 2. Composite primary key

**Decision**: Change primary key from `hash` to `(hash, project_hash)`.

**Rationale**: The same query text in different workspaces should be able to have different expansion results. For embedding cache (`project_hash='global'`), the hash alone is still unique since all entries share the same project_hash value.

### 3. Embedding cache stays global

**Decision**: Query embedding entries use `project_hash='global'` regardless of calling workspace.

**Rationale**: `embed("how does auth work")` produces identical vectors regardless of which project is active. Sharing these entries across workspaces saves Ollama round-trips (~69ms each). The workspace filtering happens downstream in `searchVec()`, not at the embedding level.

### 4. Migration strategy

**Decision**: In-place ALTER TABLE with defaults for existing rows.

SQLite supports `ALTER TABLE ADD COLUMN` with defaults. Existing rows get `project_hash='global'` and `type='general'`. No data loss, no temp table needed.

### 5. Store interface changes

**Decision**: Add optional `projectHash` parameter to `getCachedResult`, `setCachedResult`, and `clearQueryEmbeddingCache`. Callers that don't pass it get `'global'` behavior (backward compatible).

### 6. CLI cache command

**Decision**: Add `cache` command with subcommands:
- `cache clear` — clears current workspace cache (uses `resolveDbPath` + workspace projectHash)
- `cache clear --all` — clears all cache entries
- `cache clear --type=embed|expand|rerank` — clears specific type
- `cache stats` — shows counts by type and workspace

## Risks / Trade-offs

- **[Risk] Migration on large databases** → Mitigation: `ALTER TABLE ADD COLUMN` is O(1) in SQLite (metadata-only change). No data rewrite needed.
- **[Risk] Existing cache entries have `type='general'`** → Mitigation: These will be gradually replaced as new scoped entries are written. Old entries won't match scoped lookups (different project_hash), so they'll be cache misses — functionally correct, just slightly wasteful until they age out.
- **[Trade-off] Global embedding cache means model switch affects all workspaces** → Acceptable: model switches are rare and `clearQueryEmbeddingCache()` should clear all embedding entries globally regardless.
