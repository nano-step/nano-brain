# Design: Symbol-Aware Chunking

Tracking: #370
Date: 2026-06-04
Deep-design: Metis + Oracle + cross-critique + Momus sanity (all PASS)

## Current Architecture

```
File change (fsnotify)
  → watcher debounce
  → indexer: read file bytes
  → chunker.Chunk(content) → []Chunk (fixed-size or heading-aware)
  → embed each chunk → store in chunks table
  → BM25 tsvector update
```

Current chunker splits by ~500 chars or markdown heading boundaries. Functions get split mid-body.

## Proposed Architecture (v1)

### New package: `internal/chunker/`

```
internal/chunker/
  chunker.go      // Chunker interface + extended Chunk struct
  fixed.go        // existing fixed-size chunker (moved here)
  heading.go      // existing heading-aware chunker (moved here)
  symbol.go       // NEW: SymbolAwareChunker
  dispatcher.go   // NEW: dispatch by file extension
```

### Extended Chunk struct

```go
type Chunk struct {
    Content   string
    StartByte int
    EndByte   int

    // Symbol metadata (zero-value for non-symbol chunks)
    SymbolName string // e.g. "ExtractEdges"
    SymbolKind string // "function" | "method" | "type" | "const" | "var"
    Language   string // "go" | "typescript" | "python" | "javascript"
    LineStart  int
    LineEnd    int
    ChunkType  string // "raw" | "symbol"
}
```

### SymbolAwareChunker

Key decision **(D-BYTE):** Byte ranges extracted via Tree-sitter's native `node.StartByte()/EndByte()` **inside** SymbolAwareChunker. `graph.Edge` struct is NOT extended — it models relationships, not boundaries. File parsed ONCE; tree reused for both symbol extraction and byte slicing (atomic, no race).

```go
type SymbolAwareChunker struct {
    registry *graph.Registry
    fallback Chunker // fixed-size, used when parse fails or symbol >8KB
    maxBytes int     // default 8192
}

func (c *SymbolAwareChunker) Chunk(content []byte, sourcePath string) []Chunk {
    // 1. Parse ONCE — reuse tree for both edge extraction AND byte slicing
    // 2. Extract "contains" edges → symbol names + byte positions via node.StartByte()/EndByte()
    // 3. Skip symbols >8KB → fixed-size fallback for that symbol
    // 4. On parse failure or 0 symbols → fallback.Chunk(content, sourcePath)
}
```

### Dispatcher

```go
var symbolSupportedExts = map[string]bool{
    ".go": true, ".ts": true, ".tsx": true,
    ".js": true, ".jsx": true, ".py": true,
}

type Dispatcher struct {
    symbol  Chunker // SymbolAwareChunker
    heading Chunker // markdown
    fixed   Chunker // everything else
}

func (d *Dispatcher) Chunk(content []byte, sourcePath string) []Chunk {
    ext := strings.ToLower(filepath.Ext(sourcePath))
    switch {
    case symbolSupportedExts[ext]:
        return d.symbol.Chunk(content, sourcePath)
    case ext == ".md" || ext == ".mdx":
        return d.heading.Chunk(content, sourcePath)
    default:
        return d.fixed.Chunk(content, sourcePath)
    }
}
```

### Schema Migration (additive)

```sql
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS symbol_name        TEXT;
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS symbol_kind        TEXT;
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS language           TEXT;
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS line_start         INTEGER;
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS line_end           INTEGER;
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS chunk_type         TEXT NOT NULL DEFAULT 'raw';
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS embedding_strategy TEXT NOT NULL DEFAULT 'raw_code';

-- Explicit backfill — no NULL ambiguity for existing rows
UPDATE chunks SET chunk_type = 'raw'
    WHERE chunk_type IS NULL OR chunk_type = '';
UPDATE chunks SET embedding_strategy = 'raw_code'
    WHERE embedding_strategy IS NULL OR embedding_strategy = '';

CREATE INDEX IF NOT EXISTS idx_chunks_symbol_name
    ON chunks (workspace_hash, symbol_name)
    WHERE symbol_name IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_chunks_chunk_type
    ON chunks (workspace_hash, chunk_type);
```

### Reindex Strategy

- **Trigger**: Manual — `POST /api/v1/reindex` or `nano-brain reindex --workspace=<hash>`
- **Mode**: Per-file gradual. New symbol chunks inserted; old fixed-size chunks for same file deleted after successful insert.
- **Batching**: Reindex processes files in groups of 100, waits for embedding queue depth <1000 between batches (prevents queue overflow on large workspaces).
- **Mixed state**: Acceptable during transition. Edges to deleted chunks auto-pruned on next graph rebuild.
- **Graph rebuild trigger**: (1) `POST /api/v1/reindex`, (2) file watcher detects change in already-indexed file. Orphan edges pruned during rebuild, logged at INFO level.
- **Startup**: WARN log if workspace has stale `chunk_type='raw'` chunks and `indexing.chunking_strategy=symbol_aware` is configured.
- **No shadow table / no atomic swap** — simplicity over perfection.
- **embedding_strategy in v1**: Always `raw_code` — field added now to avoid schema change in v2 when summaries ship.

### Sequence Diagram (v1)

```
fsnotify event
  → Dispatcher.Chunk(content, path)
      → SymbolAwareChunker (if .go/.ts/.js/.py)
          → parse ONCE via gotreesitter
          → extract contains edges + byte ranges (node.StartByte/EndByte)
          → skip symbols >8KB → fixed-size fallback for that range
          → return []Chunk{ChunkType:"symbol", SymbolName, ...}
      → HeadingChunker (if .md/.mdx)
      → FixedChunker (everything else)
  → store chunks (chunk_type='raw' or 'symbol')
  → embed pipeline: embed Content (raw code in v1)
```

### MCP `memory_symbols` (v1)

Add `summary` as optional field (null in v1, backward compatible):

```json
{
  "name": "ExtractEdges",
  "kind": "function",
  "language": "go",
  "source_path": "internal/graph/go_extractor.go",
  "line_start": 81,
  "line_end": 97,
  "summary": null
}
```

## Settled Decisions

| ID | Decision | Rationale |
|----|----------|-----------|
| D-BYTE | Byte ranges via `node.StartByte()/EndByte()` in SymbolAwareChunker, NOT via graph.Edge extension | Edge models relationships not boundaries; parse once = atomic |
| D-SCHEMA | Extend `chunks` table with nullable columns + explicit backfill | Single table = no JOIN on search; additive = zero downtime |
| D-EMBED | Add `embedding_strategy TEXT DEFAULT 'raw_code'` | Prevents v2 cross-contamination when summaries added |
| D-SCOPE | v1 = symbol chunking only; summaries = v2 | Validate chunk quality before investing in LLM pipeline |
| D-8KB | Skip symbols >8KB, fall back to fixed-size for that symbol | Sub-chunking deferred to v2; reduces v1 complexity |
| D-NEST | Outermost scope only; closures stay with parent | Standard approach; consistent across languages |
| D-REINDEX | Per-file gradual; manual trigger; no shadow table | Simplicity; user controls timing and cost |
| D-FALLBACK | Parse failure or 0 symbols → fixed-size; log WARN; never block | 100% file coverage; operator visibility |
| D-BM25 | Index both raw code + summary (v2); tsvector updated on summary arrival | Best query coverage for both exact + semantic |
| D-LANG | Go, TypeScript, JavaScript, Python only in v1 | 4 extractors already exist; explicit unsupported fallback |
| D-MCP | `summary` optional field in memory_symbols (null in v1) | Backward compatible; agents ignore unknown fields |
| D-CHUNKID | Content-addressed IDs; `symbol_name+source_path` as stable lookup | Line shifts don't affect ID if content unchanged |

## Conflict Resolution Log

| Topic | Metis | Oracle | Resolution | Reasoning |
|-------|-------|--------|------------|-----------|
| 8KB sub-chunking in v1 | Skip entirely | Include with symbol_fragment naming | **Skip in v1** | Too much complexity; deferred to v2 |
| Embedding drift mitigation | Dual indexes | Workspace-level toggle | **Workspace toggle** | Simpler for v1; dual indexes in v2 if needed |
| Prompt engineering urgency | Phase 0 blocker | 2-sentence prompt, defer | **Defer** | Simple prompt hardcoded; tune on real data |
| Reindex state model | Gradual per-file | Manual with dry-run | **Both agree — manual per-file** | User controls; no shadow table |
| Byte range approach | "Extend Edge (small addition)" | Native Tree-sitter API | **Native API** | Edge is for relationships; cleaner separation |

## Open Questions Resolved

| Q | Resolution |
|---|-----------|
| memory_search filter by chunk_type? | NO default filter; expose optional `chunk_type` param |
| memory_symbols query chunks or graph_edges? | Chunks table; LEFT JOIN graph_edges for signature |
| Backward compat: auto-reindex or manual? | Manual with WARN on startup |
| Reindex transition state model? | Per-file gradual; mixed state acceptable |
| Chunk ID stability under line shifts? | Content-addressed; symbol_name+source_path as stable key |
| MCP memory_symbols contract change? | Optional `summary` field (null in v1); backward compatible |

## v2 Scope (deferred)

- LLM summary generation (async pipeline, concurrency=3)
- Summary cache by content_hash
- `embedding_strategy` toggle per workspace (`raw_code` → `summary`)
- 8KB sub-chunking with parent-child relationships
- Dual vector indexes (raw vs summary embeddings)
- BM25 weight tuning for summary vs code
- Cost estimation CLI (`nano-brain estimate-summary-cost`)
- Prompt engineering iteration
