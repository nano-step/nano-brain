## Context

nano-brain's `query` command (hybrid search) fails to find code identifiers like `instantSellPriceAdjustPercent` despite the data being indexed. Two compounding bugs:

1. **Symbol graph never populated**: `server.ts` line 662 calls `indexCodebase()` without the `db` parameter (6th arg). The guard `if (db && isTreeSitterAvailable())` at `codebase.ts` line 427 always fails → `code_symbols` table has 0 rows in production. Same bug exists in `watcher.ts` (lines 146, 164) and `index.ts` (line 820).

2. **FTS phrase wrapping kills multi-word queries**: `sanitizeFTS5Query` wraps the entire input as `"hello world"` (exact phrase). Natural-language queries containing code identifiers return 0 FTS results because the exact phrase never appears in documents.

The `search` command (BM25-only, single-term) works fine. But `query` — the primary user-facing command — is broken for code search.

## Goals

- Fix all 4 `indexCodebase()` call sites to pass `db`
- Make `sanitizeFTS5Query` split multi-word queries into per-term OR
- Add symbol name search as a third lane in hybrid search RRF fusion
- Add camelCase/snake_case splitting for partial symbol name matching

## Non-Goals

- Changing the RRF algorithm itself (it already supports N result sets)
- Adding fuzzy/Levenshtein matching for symbol names (exact sub-token matching is sufficient)
- Modifying the reranker or post-fusion pipeline
- Changing the `search` command (BM25-only path)

## Decisions

### D1: Per-term OR instead of AND for multi-word FTS queries
**Choice**: Split into `"term1" OR "term2" OR "term3"`
**Rationale**: OR maximizes recall — a query like `"instantSellPriceAdjustPercent config"` should find documents containing either term. AND would be too restrictive since code identifiers rarely appear alongside natural-language words in the same document. BM25 scoring naturally ranks documents with more matching terms higher.
**Alternative rejected**: AND logic — too restrictive, would miss documents containing only the code identifier.

### D2: In-memory camelCase splitting for symbol matching (no FTS on code_symbols)
**Choice**: Load candidate symbols via SQL `LIKE` prefix match, then filter in TypeScript using sub-token intersection.
**Rationale**: The `code_symbols` table has an index on `(name, kind)`. A SQL `LIKE 'pattern%'` query uses this index efficiently. For partial matches (e.g., query "userData" matching symbol "getUserData"), we split both into sub-tokens and check overlap. This avoids adding an FTS5 virtual table for symbols (overkill for name-only matching).
**Alternative rejected**: FTS5 on symbol names — adds schema complexity for a table that typically has <10K rows per project.

### D3: Symbol results reuse existing SearchResult format via document lookup
**Choice**: When a symbol matches, look up its containing document in the `documents` table by file path, and return that document as a `SearchResult` with the symbol's code range as the snippet.
**Rationale**: RRF fusion requires all result sets to share the same `SearchResult` format with a common `id` field for deduplication. Reusing document IDs means symbol matches naturally merge with FTS/vector matches for the same file.
**Alternative rejected**: Custom result type for symbols — would require changes to RRF fusion and all downstream consumers.

### D4: Symbol lane weight equals original query weight (2)
**Choice**: Symbol results from the original query get weight 2, from expanded queries get weight 1 (matching FTS/vector weights).
**Rationale**: Symbol name matches are high-signal (the user typed a code identifier), so they deserve equal weight to FTS. No need for a separate config knob initially.

## Risks / Trade-offs

- **Re-indexing required**: Users must run `memory_index_codebase` after upgrade to populate `code_symbols`. First query after upgrade will have no symbol results until re-index completes.
- **FTS recall increase may reduce precision**: OR logic returns more results than exact phrase. Mitigated by BM25 ranking (more matching terms = higher score) and reranking.
- **Symbol search adds latency**: One additional SQL query per search query variant. Mitigated by the `(name, kind)` index and small table size (<10K rows typical).
