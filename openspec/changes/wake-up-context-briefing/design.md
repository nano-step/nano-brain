## Architecture

Single `generateBriefing(store, configPath, projectHash, options?)` function in `src/wake-up.ts`. All three surfaces (CLI, MCP, HTTP) call this one function. Returns a `BriefingResult` with structured sections that get formatted into text.

## Data Flow

```
CLI/MCP/HTTP request
    u2193
generateBriefing(store, configPath, projectHash)
    u2193
u250cu2500 L0: Workspace Identity u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2510
u2502  loadCollectionConfig() u2192 collection names u2502
u2502  WorkspaceProfile.loadProfile() u2192 topics   u2502
u2502  Budget: ~100 tokens                       u2502
u2514u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2518
    u2193
u250cu2500 L1: Critical Facts u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2510
u2502  store.getTopAccessedDocuments(10)         u2502
u2502  store.getRecentDocumentsByTags(           u2502
u2502    ['decision'], 5)                        u2502
u2502  Budget: ~300 tokens                       u2502
u2514u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2518
    u2193
Format into compact markdown (~2000 chars max)
    u2193
Return to caller
```

## Key Design Decisions

1. **access_count as importance proxy** u2014 ImportanceScorer requires warm in-memory cache (only available in daemon). access_count is the primary signal in importance scoring anyway, and it's persisted in SQLite. Zero-cost, always available.

2. **Template-based output, no LLM** u2014 Briefing must be fast (<50ms). LLM summarization adds 250ms-2s latency + provider dependency. Template with truncation is sufficient for v1.

3. **Exclude superseded documents** u2014 All queries include `WHERE superseded_by IS NULL` to avoid surfacing stale/replaced information.

4. **Hard character cap (2000 chars u2248 500 tokens)** u2014 Prevents unbounded output. Each section gets a budget: L0 ~400 chars, L1 key memories ~800 chars, L1 decisions ~600 chars, buffer ~200 chars.

5. **Empty workspace handling** u2014 Returns a minimal briefing with just workspace path and "no memories yet" message rather than empty string.

## New Store Methods

```typescript
// In types.ts Store interface:
getTopAccessedDocuments(limit: number, projectHash?: string): Array<{
  id: number; path: string; collection: string; title: string;
  access_count: number; last_accessed_at: string;
}>;

getRecentDocumentsByTags(tags: string[], limit: number, projectHash?: string): Array<{
  id: number; path: string; collection: string; title: string;
  modified_at: string; tags: string[];
}>;
```

SQL for getTopAccessedDocuments:
```sql
SELECT id, path, collection, title, access_count, last_accessed_at
FROM documents
WHERE active = 1 AND superseded_by IS NULL
  AND project_hash IN (?, 'global')
ORDER BY access_count DESC
LIMIT ?
```

SQL for getRecentDocumentsByTags:
```sql
SELECT d.id, d.path, d.collection, d.title, d.modified_at
FROM documents d
JOIN document_tags dt ON dt.document_id = d.id
WHERE d.active = 1 AND d.superseded_by IS NULL
  AND dt.tag IN (?)
  AND d.project_hash IN (?, 'global')
GROUP BY d.id
ORDER BY d.modified_at DESC
LIMIT ?
```

## Output Format

Compact markdown optimized for system prompt injection:
```
## Context Briefing — {workspace_name}
**Collections:** {collection_names} | **Topics:** {top_topics}

### Key Memories
- {title} ({collection}) — {1-line snippet}
- ...

### Recent Decisions
- {title} ({date}) — {1-line snippet}
- ...
```

## Risks & Mitigations

| Risk | Severity | Mitigation |
|------|----------|------------|
| Large workspace slow queries | Low | Fixed LIMIT clauses, indexed columns |
| Stale access_count data | Low | Acceptable for advisory output; full importance in v2 |
| Three-surface sync drift | Medium | Single generateBriefing() function |
| Empty workspace blank output | Low | Explicit empty-state messages |

## Future (out of scope for v1)
- L0 auto-detection from package.json/Cargo.toml/etc.
- L2 focused context via --collection/--tag filter using hybridSearch
- LLM summarization for more compact output
- Manual identity file (~/.nano-brain/workspaces/<hash>/identity.md)
- Result caching with TTL
- Configurable token budget via config.yml
- "What changed since last session" diff