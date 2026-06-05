# Design: Code Symbol Summarization

> **Revised**: 2026-06-05 after Oracle + Metis deep-design gap analysis.

## Context

Two existing subsystems inform this design:

1. **Session summarization** (`internal/summarize/`) — LLM map-reduce pipeline for sessions. Proves: provider abstraction, concurrent LLM calls, idempotent storage pattern.
2. **Symbol extraction** (`internal/symbol/`) — Tree-sitter based extraction producing `Symbol{Name, Kind, Signature, File, Line, Language}`. Currently stored in chunks with `chunk_type=symbol`, `embedding_strategy=raw_code`.
3. **Harvester** (`internal/harvest/`) — Independent background goroutine that polls for new sessions periodically. Proves: decoupled service pattern.

## Architecture Overview

```
                    ┌─────────────────────────────────┐
                    │  Watcher (existing, unchanged)   │
                    │  scanCollection() → processFile()│
                    │  → chunks (raw + symbol)         │
                    │  → embedQueue.Enqueue(chunk_ids) │
                    └─────────────┬───────────────────┘
                                  │ creates symbol chunks in DB
                                  ▼
┌─────────────────────────────────────────────────────────────┐
│  CodeSummarizer (NEW — independent background service)      │
│                                                             │
│  Polling loop (every poll_interval seconds):                │
│  1. Query DB: symbol chunks without matching summary doc    │
│  2. Check daily budget (DB-persisted counter)               │
│  3. Batch into groups of batch_size                         │
│  4. For each batch (concurrency workers):                   │
│     a. Build prompt (multi-symbol, structured JSON output)  │
│     b. Call LLM provider                                    │
│     c. Parse JSON array response                            │
│     d. Upsert summary documents                            │
│     e. Enqueue summary chunk_ids for embedding              │
│  5. Increment daily budget counter                          │
└─────────────────────────────────────────────────────────────┘
```

**Key architectural decision**: CodeSummarizer is an **independent background service** (like harvester), NOT hooked into watcher. This prevents watcher coupling, allows independent scaling/configuration, and survives watcher restarts.

## Decision Log

### D1 — Storage: Where to store summaries?

**Options:**
| Option | Pro | Con |
|--------|-----|-----|
| A. `chunks.metadata` JSONB field | No schema change | Hard to search independently |
| B. New column `chunks.summary TEXT` | Clean, queryable | Migration required |
| C. Separate documents (collection=`code-summary`) | Searchable via existing pipeline, independent lifecycle | Extra docs, join needed |

**Chosen: C — Separate documents in `code` collection with tag `symbol-summary`.**

Rationale:
- Summaries participate in search naturally (BM25 + vector) without schema changes
- Can be regenerated independently without touching source chunks
- Same pattern as session-summary (proven)
- `source_path` = `<file_path>?symbol=<name>&kind=<kind>&hash=<content_hash[:8]>&summary=true` (dedup key includes hash to handle same-name symbols)
- `metadata.symbol_name`, `metadata.symbol_kind`, `metadata.source_file`, `metadata.source_content_hash`, `metadata.model_version`
- `embedding_strategy=summary` (distinct from `raw_code`) for future differentiation

### D2 — Batching: How to minimize requests?

**Chosen: Multi-symbol per prompt with structured JSON output.**

Prompt template:
```
You are a code documentation expert. For each function/type below, write a 2-4 sentence summary explaining:
- What it does (behavior, not just structure)
- Key inputs/outputs
- How it relates to the broader system

Respond with a JSON array. Each element: {"name": "<symbol_name>", "file": "<file_path>", "summary": "<2-4 sentences>"}

## Symbols

### 1. [file: internal/watcher/filter.go, kind: struct]
```go
type gitignoreStack struct {
    entries []gitignoreEntry
}
```

### 2. [file: internal/watcher/filter.go, kind: method]
```go
func (s *gitignoreStack) Push(dirPath string, matcher *gitignore.GitIgnore) {
    ...
}
```

... (N symbols)
```

**Token budget per batch:**
- Input: ~200-500 tokens per symbol × 30 symbols = ~6,000-15,000 input tokens
- Output: ~50-100 tokens per summary × 30 symbols = ~1,500-3,000 output tokens
- Well within Gemini 2.5 Flash limits (1M input, 64K output)

**Response matching algorithm:**
- Match by composite key: `(name, file, kind)` — NOT name alone
- Detect missing symbols via set difference between sent and received
- Strict equality (no fuzzy matching)
- If count mismatch: store matched results, unmatched symbols retried next cycle

### D3 — Trigger: Independent background service (revised from post-scan hook)

**Chosen: Independent polling service (like harvester), NOT watcher hook.**

Flow:
1. Background goroutine starts at server boot (if `code_summarization.enabled=true`)
2. Every `poll_interval` seconds (default: 60):
   a. Query DB for symbol chunks without corresponding summary docs
   b. Check daily budget counter (DB-persisted)
   c. If budget exhausted → sleep until next day, log info
   d. Batch unsummarized symbols into groups of `batch_size`
   e. Process batches with `concurrency` parallel workers
   f. After each batch: upsert summary docs + enqueue for embedding
   g. Increment budget counter
3. Skip cycle if no unsummarized symbols found

**Why NOT watcher hook (Oracle recommendation):**
- Watcher is already 761 lines with 3 extraction pipelines
- LLM calls are slow (seconds) — would block watcher scan
- Separate service allows independent restart/configuration
- No circular dependency risk
- Can rate-limit independently of file processing

### D4 — Incremental: How to avoid re-summarizing?

**Chosen: Content-hash tracking via existing infrastructure.**

- Chunks already have `content_hash` (SHA-256) — confirmed in DB schema
- Summary document stores `metadata.source_content_hash`
- Finding unsummarized symbols: LEFT JOIN query
  ```sql
  SELECT c.* FROM chunks c
  WHERE c.workspace_hash = $1
    AND c.chunk_type = 'symbol'
    AND c.symbol_name IS NOT NULL
    AND NOT EXISTS (
      SELECT 1 FROM documents d
      WHERE d.workspace_hash = c.workspace_hash
        AND d.source_path = format('%s?symbol=%s&kind=%s&hash=%s&summary=true',
                                   ..., c.content_hash[:8])
    )
  LIMIT $2
  ```
- On symbol code change: content_hash changes → old summary no longer matches → new summary generated
- Old summary: superseded via existing `supersedes_id` mechanism OR left as orphan (cleaned by periodic GC)

### D5 — Provider interface

**Chosen: Reuse OpenAI-compatible chat completions API** (same as session summarization).

```go
type CodeSummarizer interface {
    SummarizeBatch(ctx context.Context, symbols []SymbolForSummary) ([]SymbolSummary, error)
}

type SymbolForSummary struct {
    Name        string
    Kind        string
    File        string
    Language    string
    Code        string // raw source code of the symbol
    ContentHash string // for dedup tracking
}

type SymbolSummary struct {
    Name    string `json:"name"`
    File    string `json:"file"`
    Summary string `json:"summary"`
}
```

Provider config:
```yaml
code_summarization:
  enabled: false
  provider_url: ""                    # OpenAI-compatible endpoint
  api_key: ""                         # env: NANO_BRAIN_CODE_SUMMARIZE_API_KEY
  model: "gemini/gemini-2.5-flash"
  batch_size: 30                      # symbols per LLM request
  max_output_tokens: 8000
  concurrency: 2                      # parallel batch workers
  max_requests_per_day: 0             # 0 = unlimited
  max_symbol_lines: 500               # skip symbols exceeding this (WARNING log)
  poll_interval_seconds: 60           # how often to check for unsummarized symbols
  max_summaries_per_cycle: 300        # cap per poll cycle to avoid queue saturation
  fallback_model: ""
```

### D6 — Daily budget counter (DB-persisted)

**Chosen: PostgreSQL table for persistence across restarts.**

```sql
CREATE TABLE IF NOT EXISTS code_summarization_usage (
    workspace_hash TEXT NOT NULL,
    usage_date DATE NOT NULL DEFAULT CURRENT_DATE,
    request_count INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (workspace_hash, usage_date)
);
```

- On each LLM call: `INSERT ... ON CONFLICT DO UPDATE SET request_count = request_count + 1`
- Budget check: `SELECT request_count FROM ... WHERE usage_date = CURRENT_DATE`
- If `request_count >= max_requests_per_day` → skip, log info
- Timezone: **UTC** (server-side `CURRENT_DATE` in UTC)
- No explicit reset needed — new rows created daily
- Old rows auto-cleaned after 30 days (periodic GC)

### D7 — Error handling

- LLM timeout/error → log warning, skip batch, retry next poll cycle
- Partial JSON response (some symbols missing) → store matched results, unmatched retried next cycle (implicit via re-query)
- Daily budget exhausted → stop summarization for today, log info, resume tomorrow
- Invalid JSON → retry once with same prompt, then skip batch
- Symbol >500 lines → skip with WARNING log (never sent to LLM)
- Embedding queue >1000 pending → pause summarization until queue drains below 500

### D8 — Search integration

No changes needed to search pipeline. Summaries stored as regular documents:
- BM25 matches on summary text naturally
- Vector embeddings generated via existing embed queue (enqueued explicitly after summary creation)
- `embedding_strategy=summary` distinguishes from raw code chunks
- `memory_query("recursive gitignore loading")` → matches summary containing "stack-based approach for nested .gitignore discovery during directory traversal"
- NOT exposed in `memory_graph` for MVP (only in search tools)

### D9 — Orphan cleanup

When a symbol chunk is deleted (file removed/changed):
- Old summary document becomes orphan (source_path no longer matches any chunk)
- Periodic GC (same cycle as poll): query summaries whose `source_content_hash` doesn't match any current chunk → delete
- Frequency: once per hour (not every poll cycle)

## Package Layout

```
internal/codesummarize/
├── summarize.go          # CodeSummarizer interface + polling orchestrator
├── prompt.go             # Prompt template builder
├── provider.go           # OpenAI-compatible LLM client (reuse pattern from internal/summarize)
├── budget.go             # DB-persisted daily budget counter
├── provider_test.go
├── budget_test.go
└── summarize_test.go
```

## Migration

One small migration needed for budget tracking:

```sql
-- 000XX_code_summarization_usage.sql
CREATE TABLE IF NOT EXISTS code_summarization_usage (
    workspace_hash TEXT NOT NULL,
    usage_date DATE NOT NULL DEFAULT CURRENT_DATE,
    request_count INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (workspace_hash, usage_date)
);
```

No changes to existing `documents` or `chunks` tables — summaries use existing `UpsertDocument` flow with `embedding_strategy=summary`.

## PR Split (2 PRs)

### PR1: Infrastructure + Manual API (normal lane within high-risk feature)
- `internal/codesummarize/` package (interface, provider, prompt, budget)
- Config struct `CodeSummarizationConfig` in `internal/config/`
- Migration: `code_summarization_usage` table
- New endpoint: `POST /api/v1/code/summarize` (manual trigger, takes workspace_hash)
- Unit tests + integration test (manual trigger → verify summary docs created)

### PR2: Background Service Integration
- Polling goroutine started at server boot
- Integration with embed queue (explicit enqueue after summary creation)
- Embedding queue depth check (pause if >1000 pending)
- Orphan cleanup GC
- Integration test: register workspace → scan files → wait → verify summaries auto-created

## Risks

| Risk | Mitigation |
|------|-----------|
| LLM generates hallucinated summaries | Prompt includes actual code; structured JSON schema constrains output |
| Rate limit exceeded | DB-persisted daily budget cap + exponential backoff |
| Stale summaries after code change | Content-hash invalidation (new hash = new summary) |
| Orphan summaries after file deletion | Periodic GC (hourly) removes unmatched summaries |
| Large codebases overwhelm budget | `max_summaries_per_cycle=300` caps per-cycle work |
| Oversized symbols (500+ lines) | `max_symbol_lines=500`, skip with WARNING |
| Embedding queue saturation | Pause if queue >1000 pending |
| Inconsistent summary quality across models | Single fixed model (no rotation) + `metadata.model_version` tracking |
| Provider downtime | Exponential backoff; skip cycle, retry next |
| Server restart mid-day loses budget | DB-persisted counter survives restart |

## Explicit Exclusions (NOT in scope)

- ❌ Priority-based symbol ordering (all symbols equal in MVP)
- ❌ Query-time LLM synthesis ("explain how X works" using summaries as context)
- ❌ Re-ranking search results using summaries
- ❌ Exposing summaries in `memory_graph` tool results
- ❌ Multi-language support beyond Go/TS/Python/JS
- ❌ Fuzzy matching for LLM response names
- ❌ Batch API (native Gemini async batch) — uses sync OpenAI-compatible API

## Future Work

- Query-time LLM synthesis: "explain how X works" using retrieved summaries as context
- Symbol prioritization: summarize most-queried symbols first
- Multi-file feature summaries: group related symbols into feature-level descriptions
- Model upgrade path: bulk-regenerate when model changes (using `metadata.model_version`)
- Expose summaries in `memory_graph` alongside symbol nodes
