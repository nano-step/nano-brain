## Why

The `llm_cache` table stores query expansion, reranking, and embedding cache entries with no project/workspace scoping. All entries share a single global namespace. This means expansion results generated in the context of one project leak into searches for another project, producing irrelevant query variants. Additionally, `clearQueryEmbeddingCache()` executes `DELETE FROM llm_cache` which nukes all cache types across all workspaces — there's no way to clear cache for just one project or one cache type.

## What Changes

- Add `project_hash` and `type` columns to `llm_cache` table with schema migration for existing data
- Scope expansion and reranking cache reads/writes by `project_hash`
- Keep query embedding cache global (same text produces identical vectors regardless of project)
- Scope `clearQueryEmbeddingCache()` to delete by type and optionally by project
- Add `cache` CLI command with `clear` (current workspace) and `clear --all` (everything) subcommands
- Add `cache stats` CLI subcommand to show cache entry counts by type and workspace

## Capabilities

### New Capabilities
- `cache-cli`: CLI command for cache management — clear by workspace, clear all, view stats

### Modified Capabilities
- `storage-limits`: Cache table schema changes (add `project_hash`, `type` columns) and scoped deletion
- `search-pipeline`: Expansion and reranking cache calls pass `projectHash` for scoped storage/retrieval
- `mcp-server`: Vector search cache calls pass `currentProjectHash` for consistency
- `workspace-scoping`: Cache entries participate in workspace isolation

## Impact

- **Schema**: `llm_cache` table gains two columns + composite primary key change — requires migration
- **Store interface**: `getCachedResult`, `setCachedResult`, `clearQueryEmbeddingCache` signatures change (optional `projectHash` param)
- **Search pipeline**: `hybridSearch` must thread `projectHash` through to cache calls
- **MCP server**: `memory_vsearch` tool passes workspace context to cache
- **CLI**: New `cache` command added to command router
- **Tests**: Mock stores in `search.test.ts` and `watcher.test.ts` need updated signatures
