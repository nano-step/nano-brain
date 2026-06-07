## Design: Search Quality Improvements

### Phased Delivery

| Phase | Scope | Effort | Risk |
|-------|-------|--------|------|
| 1a | Query Preprocessor (translate + expand) | 1-2 days | Low |
| 1b | BM25 language config + migration | 0.5 day | Low |
| 1c | Chunk overlap increase | 0.5 day | Low |
| 1d | Document-level dedup default | 0.5 day | Low |
| 2a | HyDE (conditional) | 1 day | Medium |
| 2b | Reranking | 1-2 days | Medium |
| 2c | Dynamic RRF weights | 0.5 day | Low |

### Phase 1a: Query Preprocessor

#### Interface

```go
// internal/search/preprocess/preprocessor.go

type PreprocessResult struct {
    OriginalQuery   string
    ProcessedQuery  string   // translated + expanded
    Language        string   // detected language ("en", "vi", etc.)
    Intent          Intent   // keyword, conceptual, temporal
    TimeFilter      *TimeFilter // extracted temporal constraint (if any)
    Expansions      []string // additional search terms
}

type Intent string
const (
    IntentKeyword    Intent = "keyword"
    IntentConceptual Intent = "conceptual"
    IntentTemporal   Intent = "temporal"
)

type Preprocessor struct {
    client   *http.Client
    provider string // OpenAI-compatible endpoint
    apiKey   string
    model    string
    timeout  time.Duration
    logger   zerolog.Logger
}

func (p *Preprocessor) Process(ctx context.Context, query string) (*PreprocessResult, error)
```

#### LLM Prompt Strategy

Single LLM call that returns structured JSON:

```
System: You are a search query preprocessor for a code knowledge base.
Given a user query, return JSON with:
- "language": detected language code (e.g. "en", "vi", "zh")
- "english_query": the query translated to English (if already English, return as-is)
- "intent": one of "keyword", "conceptual", "temporal"
- "expansions": 2-3 related search terms/synonyms
- "time_filter": null or {"after": "ISO8601", "before": "ISO8601"} if temporal

User query: "{query}"
```

Response parsed → `PreprocessResult`. On timeout/error → fallback to original query unchanged.

#### Integration Point

```go
// internal/search/service.go — HybridSearch()

func (s *Service) HybridSearch(ctx context.Context, query string, ...) ([]Result, error) {
    // NEW: Preprocess query if enabled
    var processed *preprocess.PreprocessResult
    if s.preprocessor != nil {
        processed, _ = s.preprocessor.Process(ctx, query)
        if processed != nil {
            query = processed.ProcessedQuery
            // Apply extracted time filters
            if processed.TimeFilter != nil {
                opts.CreatedAfter = processed.TimeFilter.After
            }
        }
    }

    // Existing: parallel BM25 + vector search
    // ...
}
```

### Phase 1b: BM25 Language Configuration

#### Migration (00015_bm25_configurable_language.sql)

```sql
-- +goose Up
-- Make tsvector language configurable via a GUC or function parameter.
-- For now: change trigger to use 'simple' which is language-agnostic.
-- Individual workspaces can override via search.bm25_language config.

-- We keep the trigger but parameterize it via a helper function.
CREATE OR REPLACE FUNCTION get_tsvector_config() RETURNS regconfig AS $$
BEGIN
    RETURN current_setting('nanobrain.tsvector_config', true)::regconfig;
EXCEPTION WHEN OTHERS THEN
    RETURN 'english'::regconfig;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

CREATE OR REPLACE FUNCTION chunks_search_vector_update() RETURNS trigger AS $$
DECLARE
    doc_title TEXT;
    tsconfig regconfig;
BEGIN
    tsconfig := get_tsvector_config();
    IF pg_trigger_depth() > 1 AND NEW.search_vector IS NOT NULL THEN
        RETURN NEW;
    END IF;
    SELECT title INTO doc_title FROM documents WHERE id = NEW.document_id;
    NEW.search_vector :=
        setweight(to_tsvector(tsconfig, coalesce(doc_title, '')), 'A') ||
        setweight(to_tsvector(tsconfig, coalesce(NEW.content, '')), 'B');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- +goose Down
-- Revert to hardcoded 'english'
CREATE OR REPLACE FUNCTION chunks_search_vector_update() RETURNS trigger AS $$
DECLARE
    doc_title TEXT;
BEGIN
    IF pg_trigger_depth() > 1 AND NEW.search_vector IS NOT NULL THEN
        RETURN NEW;
    END IF;
    SELECT title INTO doc_title FROM documents WHERE id = NEW.document_id;
    NEW.search_vector :=
        setweight(to_tsvector('english', coalesce(doc_title, '')), 'A') ||
        setweight(to_tsvector('english', coalesce(NEW.content, '')), 'B');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP FUNCTION IF EXISTS get_tsvector_config();
```

#### Search Query Update

```sql
-- Current (hardcoded):
websearch_to_tsquery('english', @query)

-- New (parameterized):
websearch_to_tsquery(current_setting('nanobrain.tsvector_config', true)::regconfig, @query)
```

Set per-connection: `SET nanobrain.tsvector_config = 'simple'` before search queries when config says so.

### Phase 1c: Chunk Overlap

Change in `internal/chunk/chunk.go`:

```go
// Current:
DefaultOverlap = 200

// New:
DefaultOverlap = 600
```

Config-driven via `watcher.chunk_overlap` (new field in `WatcherConfig`).

Only affects NEW chunks. Existing chunks keep their 200-byte overlap until re-indexed.

### Phase 1d: Document-Level Dedup Default

In MCP tools (`internal/mcp/tools.go`), when `group_by` is not specified:
- Default to `"document"` for `memory_query`, `memory_search`, `memory_vsearch`
- Explicitly passing `group_by=""` or `group_by="none"` disables dedup

This is a **behavioral change** but aligns with user expectations (1 result per document).

### Phase 2a: HyDE

```go
// internal/search/preprocess/hyde.go

func (p *Preprocessor) GenerateHypothetical(ctx context.Context, query string) (string, error) {
    prompt := fmt.Sprintf(
        "Write a short passage (2-3 sentences) that would perfectly answer this question: %s",
        query,
    )
    return p.callLLM(ctx, prompt)
}
```

Gated: only when `intent == IntentConceptual` AND `hyde.enabled == true`.

The hypothetical document is embedded instead of the raw query for vector search only. BM25 still uses the original/translated query.

### Phase 2b: Reranking

Two options (decide during implementation based on library availability):

**Option A: ONNX purego** (preferred if library works)
```go
reranker, _ := onnx.NewCrossEncoder("bge-reranker-v2-m3.onnx")
scores := reranker.Score(query, documents)
```

**Option B: LLM-based reranking** (fallback)
```go
// Use existing LLM provider to score relevance
prompt := "Rate relevance 0-10: Query: {q}, Document: {d}"
```

Option B is slower but guaranteed to work with any OpenAI-compatible provider.

### Testing Strategy

| Layer | Test Type | What |
|-------|-----------|------|
| Unit | `preprocess_test.go` | Mock LLM responses, verify parsing |
| Unit | `intent_test.go` | Classification accuracy on curated queries |
| Integration | `search_integration_test.go` | Full pipeline with test corpus |
| Benchmark | `bench/` suite | Before/after precision@10 on curated queries |

Benchmark corpus: 50 curated queries (English + Vietnamese) with expected relevant doc IDs.

### Rollout

All features behind config flags. Rollout:
1. Deploy with all disabled (zero behavior change)
2. Enable `query_preprocessing` → measure latency + quality
3. Enable `hyde` → measure precision on conceptual queries
4. Enable `reranking` → measure top-10 precision

### Non-Goals

- Changing the embedding model (keep nomic-embed-text)
- Adding a Python sidecar
- Full multilingual BM25 (just parameterize language config)
- Changing RRF k=60 constant (defer to Phase 2c)
