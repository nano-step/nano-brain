# Design: Memory Intelligence Phase 1

## Context

nano-brain currently treats all memories equally regardless of age or usage. A 6-month-old debugging note ranks the same as yesterday's architecture decision. The search pipeline (RRF fusion → top rank bonus → centrality boost → supersede demotion → position-aware blending) has no awareness of memory lifecycle. This design adds three lightweight intelligence features to the existing pipeline without introducing LLM dependencies or blocking operations.

The search scoring pipeline lives in `search.ts` and follows this flow:
1. `rrfFuse` — combines BM25 (FTS) and vector search results
2. `applyTopRankBonus` — boosts top-ranked results (not currently in use)
3. `applyCentralityBoost` — multiplies score by `(1 + centralityWeight * centrality)`
4. `applySupersedeDemotion` — multiplies score by `demotionFactor` for superseded docs
5. `positionAwareBlend` — blends RRF and rerank scores based on position (top3/mid/tail)

Schema is in `store.ts` with tables: `documents`, `content`, `document_tags`, `content_vectors`, `documents_fts`. The `documents` table currently tracks `created_at`, `modified_at`, and `active` but has no access tracking.

## Goals

**In Scope:**
- Track memory access patterns (count and timestamp) without performance overhead
- Apply time-based relevance decay to search scoring using a configurable half-life model
- Auto-categorize memories on write using fast heuristic rules (no LLM)
- Boost frequently accessed memories in search results
- Maintain backward compatibility with existing memories and config

**Out of Scope (Phase 2+):**
- LLM-based categorization or summarization
- Automatic memory deletion or archival
- User-facing UI for memory management
- Cross-collection memory relationships
- Memory consolidation or deduplication

## Decisions

### 1. Memory Relevance Decay

**What:** Add `access_count INTEGER DEFAULT 0` and `last_accessed_at TEXT` columns to the `documents` table. Track access on every search result returned to the user. Apply exponential decay to search scores based on time since last access.

**Why:** Memories that haven't been accessed in months are less likely to be relevant now. Exponential decay with a configurable half-life (default 30 days) provides intuitive control: memory is "half as relevant" after N days of no access. This is gentler than LRU eviction (which deletes) and more flexible than fixed TTL (which is binary).

**How:**
- Schema migration: `ALTER TABLE documents ADD COLUMN access_count INTEGER DEFAULT 0; ALTER TABLE documents ADD COLUMN last_accessed_at TEXT;`
- On search result return (in `hybridSearch` after final scoring), increment `access_count` and update `last_accessed_at` for each result shown to the user
- Decay function: `decayScore = 1 / (1 + daysSinceAccess / halfLife)` where `daysSinceAccess = (now - last_accessed_at) / 86400000`
- Applied as a multiplier in the scoring pipeline after RRF fusion but before position-aware blending
- New config section in `config.yml`:
  ```yaml
  decay:
    enabled: true
    halfLife: "30d"  # parsed to days
    boostWeight: 0.15  # how much decay affects final score
  ```
- Backward compatible: existing memories get `access_count=0`, `last_accessed_at=NULL`; decay treats NULL as "never accessed" (maximum decay)

**Alternatives considered:**
- **LRU eviction:** Too aggressive. Deletes memories permanently. Users lose context.
- **Fixed TTL:** Too rigid. A 90-day-old architecture decision is still valuable; a 7-day-old typo fix is not.
- **No decay (status quo):** Loses signal. Old noise drowns out recent signal as memory grows.

**Trade-offs:**
- Adds 2 columns to `documents` table (minimal storage overhead: ~16 bytes per doc)
- Adds 1 UPDATE per search result returned (negligible for typical result counts of 5-20)
- Decay calculation is pure math (no I/O, no blocking)

### 2. Auto-Categorization on Write

**What:** When `memory_write` is called, classify content into predefined categories using keyword/pattern heuristics. Populate the existing `document_tags` table with auto-generated tags prefixed with `auto:`.

**Why:** Manual tagging is tedious and inconsistent. Heuristic categorization is instant, deterministic, and requires no external dependencies. Categories help users filter searches (e.g., "show me architecture decisions") and provide structure for future features (e.g., category-specific retention policies).

**How:**
- Categories: `architecture-decision`, `debugging-insight`, `tool-config`, `pattern`, `preference`, `context`, `workflow`
- Detection rules (keyword matching, case-insensitive):
  - `architecture-decision`: "decided", "chose", "architecture", "design decision", "trade-off", "approach"
  - `debugging-insight`: "error", "fix", "bug", "stack trace", "crash", "exception", "workaround"
  - `tool-config`: "config", "setup", "install", "environment", "settings", "configuration"
  - `pattern`: "pattern", "convention", "idiom", "best practice", "anti-pattern"
  - `preference`: "prefer", "avoid", "like", "dislike", "style", "opinion"
  - `context`: "context", "background", "overview", "summary", "explanation"
  - `workflow`: "workflow", "process", "steps", "procedure", "checklist"
- Applied in `store.insertDocument` after content insertion
- Tags are prefixed with `auto:` (e.g., `auto:architecture-decision`) to distinguish from user-provided tags
- Additive: does not remove user-provided tags
- Multiple categories can apply to a single memory

**Alternatives considered:**
- **LLM-based categorization:** Too slow (100-500ms per write), costs money, requires API keys. Deferred to Phase 2.
- **No categorization (status quo):** Memories remain unstructured. Harder to filter or prioritize.
- **User-only tagging:** Requires manual effort. Users forget or skip tagging.

**Trade-offs:**
- Heuristics are imperfect. Some memories will be miscategorized or miss categories.
- Keyword matching is language-dependent (assumes English). Non-English memories may not categorize well.
- Adds ~5-10ms to write latency (negligible for background MCP server)

### 3. Usage-Based Search Boosting

**What:** Integrate `access_count` and `last_accessed_at` into the hybrid search scoring pipeline. Frequently accessed memories get a configurable boost.

**Why:** Memories that are accessed repeatedly are more likely to be relevant in the future. This is a form of implicit feedback: the user's past behavior signals importance. Combined with decay, this creates a "hot/cold" memory system where frequently accessed recent memories rank highest.

**How:**
- New function `applyUsageBoost(results, config)` similar to `applyCentralityBoost`
- Formula: `usageBoost = log2(1 + access_count) * decayScore * boostWeight`
  - `log2(1 + access_count)` provides diminishing returns (10 accesses is not 10x better than 1 access)
  - `decayScore` is the decay multiplier from Feature 1 (so old memories don't get boosted)
  - `boostWeight` is configurable (default 0.15)
- Applied in the scoring pipeline after `applyCentralityBoost`, before `applySupersedeDemotion`
- New config field in `SearchConfig` type (`types.ts`):
  ```typescript
  usage_boost_weight: number; // default 0.15
  ```
- Backward compatible: memories with `access_count=0` get no boost (log2(1) = 0)

**Alternatives considered:**
- **Replace RRF entirely with usage-based ranking:** Too risky. RRF is proven. Usage is a signal, not the only signal.
- **Separate boost index:** Over-engineered. Usage data is already in `documents` table.
- **Linear boost (not log):** Amplifies outliers too much. A memory accessed 100 times would dominate results.

**Trade-offs:**
- Adds 1 JOIN to search query (negligible: `documents` table is already joined)
- Boost calculation is pure math (no I/O, no blocking)
- Cold start problem: new memories have `access_count=0` and rank lower. Mitigated by decay (new memories have no decay penalty).

## Risks and Trade-offs

### Performance
- **Risk:** Access tracking adds 1 UPDATE per search result. For 20 results, that's 20 UPDATEs.
- **Mitigation:** SQLite handles small UPDATEs efficiently. Batch UPDATEs in a single transaction. Measure latency in testing.
- **Fallback:** If latency is unacceptable, make access tracking async (fire-and-forget).

### Accuracy
- **Risk:** Heuristic categorization is imperfect. Memories may be miscategorized.
- **Mitigation:** Use `auto:` prefix so users can distinguish auto-tags from manual tags. Users can remove incorrect auto-tags.
- **Fallback:** If categorization is too noisy, add a config flag to disable it.

### Cold Start
- **Risk:** New memories have `access_count=0` and rank lower than old frequently accessed memories.
- **Mitigation:** Decay penalizes old memories. New memories have no decay penalty, so they start with a neutral score.
- **Fallback:** Add a "recency boost" in Phase 2 to explicitly favor new memories.

### Backward Compatibility
- **Risk:** Existing memories have `access_count=0` and `last_accessed_at=NULL`.
- **Mitigation:** Decay treats NULL as "never accessed" (maximum decay). This is correct: old memories that were never accessed should rank lower.
- **Fallback:** Provide a migration script to backfill `last_accessed_at` with `created_at` for existing memories.

### Configuration Complexity
- **Risk:** Adding `decay` and `usage_boost_weight` config fields increases surface area.
- **Mitigation:** Provide sensible defaults (enabled, 30d half-life, 0.15 boost weight). Document in README.
- **Fallback:** If users find config overwhelming, hide advanced options behind a `--advanced` flag.

## Implementation Notes

### Schema Migration
```sql
ALTER TABLE documents ADD COLUMN access_count INTEGER DEFAULT 0;
ALTER TABLE documents ADD COLUMN last_accessed_at TEXT;
CREATE INDEX IF NOT EXISTS idx_documents_access ON documents(last_accessed_at);
```

### Scoring Pipeline Order
1. `rrfFuse` — combine BM25 + vector
2. `applyTopRankBoost` (if enabled)
3. `applyCentralityBoost` (existing)
4. **`applyUsageBoost` (new)** — boost frequently accessed memories
5. `applySupersedeDemotion` (existing)
6. `positionAwareBlend` — blend RRF + rerank scores
7. **Track access** (new) — increment `access_count`, update `last_accessed_at`

### Config Schema
```yaml
decay:
  enabled: true
  halfLife: "30d"  # supports "7d", "30d", "90d", etc.
  boostWeight: 0.15

search:
  usage_boost_weight: 0.15
  # ... existing fields
```

### Auto-Categorization Rules
Implemented as a simple keyword matcher in `store.ts`:
```typescript
function autoCategorizeTags(content: string): string[] {
  const tags: string[] = [];
  const lower = content.toLowerCase();
  
  if (/\b(decided|chose|architecture|design decision|trade-?off|approach)\b/.test(lower)) {
    tags.push('auto:architecture-decision');
  }
  if (/\b(error|fix|bug|stack trace|crash|exception|workaround)\b/.test(lower)) {
    tags.push('auto:debugging-insight');
  }
  // ... more rules
  
  return tags;
}
```

### Testing Strategy
- Unit tests for decay function (edge cases: NULL, 0, negative)
- Unit tests for usage boost (edge cases: 0 access_count, high access_count)
- Unit tests for auto-categorization (each category, multi-category, no match)
- Integration test: write memory → search → verify access tracking
- Integration test: search with decay enabled vs disabled
- Performance test: measure search latency with 10k memories, 20 results
