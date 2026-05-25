# Knowledge Graph — Design

## Architecture

### New table: `graph_edges`

Migration `migrations/00008_knowledge_graph.sql`:

```sql
-- +goose Up
CREATE TABLE graph_edges (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_hash TEXT       NOT NULL,
    source_node  TEXT        NOT NULL,
    target_node  TEXT        NOT NULL,
    edge_type    TEXT        NOT NULL CHECK (edge_type IN ('contains', 'imports', 'calls')),
    source_file  TEXT        NOT NULL,
    metadata     JSONB       NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_graph_edge UNIQUE (workspace_hash, source_node, target_node, edge_type)
);

CREATE INDEX idx_graph_edges_source ON graph_edges(workspace_hash, source_node, edge_type);
CREATE INDEX idx_graph_edges_target ON graph_edges(workspace_hash, target_node, edge_type);
CREATE INDEX idx_graph_edges_file   ON graph_edges(workspace_hash, source_file);

-- +goose Down
DROP TABLE IF EXISTS graph_edges;
```

**Node naming convention:**

| Node type | Format | Example |
|-----------|--------|---------|
| File | workspace-relative path | `internal/watcher/watcher.go` |
| Import target | raw import string | `github.com/nano-brain/nano-brain/internal/symbol` |
| Symbol (call source/target) | `file::name` | `watcher.go::processFile` |

**`source_file` column**: enables `DELETE WHERE source_file = $1` to atomically invalidate all outgoing edges when a file changes — no need to diff which edges changed.

**`metadata` JSONB**: stores `{"line": 42, "language": "go"}` per edge.

### Package: `internal/graph/`

```
internal/graph/
  edge.go        — Edge struct, EdgeKind constants, Extractor interface
  registry.go    — Registry: holds extractors, dispatches by language
  go_extractor.go — Go: contains + imports + calls via gotreesitter
```

**Edge struct:**
```go
type EdgeKind string

const (
    EdgeContains EdgeKind = "contains"
    EdgeImports  EdgeKind = "imports"
    EdgeCalls    EdgeKind = "calls"
)

type Edge struct {
    SourceNode string
    TargetNode string
    Kind       EdgeKind
    SourceFile string   // workspace-relative path
    Line       int
    Language   string
}

type Extractor interface {
    ExtractEdges(filePath string, content []byte) ([]Edge, error)
    Supports(ext string) bool
}
```

**Registry:**
```go
type Registry struct {
    extractors []Extractor
    log        *slog.Logger
}

func (r *Registry) ExtractEdges(filePath string, content []byte) ([]Edge, error)
```

### Go extractor: three passes

1. **Contains**: read from existing symbol documents in DB (no tree-sitter needed) — query `ListSymbolsByWorkspace` filtered to `source_file`, emit `file → file::symbol` edges.
2. **Imports**: tree-sitter query `(import_spec path: (interpreted_string_literal) @path)` on the file → emit `file → raw_import_path` edges.
3. **Calls**: tree-sitter query — collect all `function_declaration` ranges, collect all `call_expression` callee identifiers with byte positions, map each call to its enclosing function by byte range overlap → emit `file::func → callee_name` edges (best-effort).

### Watcher integration

In `internal/watcher/watcher.go`, after `extractAndUpsertSymbols()`:

```go
func (w *Watcher) WithGraphRegistry(r *graph.Registry, store storage.GraphStore) *Watcher
```

In `scanCollection()`, after the existing symbol extraction call:

```go
if w.graphRegistry != nil {
    if err := w.extractAndUpsertEdges(ctx, tx, workspaceHash, filePath, content); err != nil {
        slog.Warn("graph edge extraction failed", "file", filePath, "err", err)
        // non-fatal: log and continue
    }
}
```

`extractAndUpsertEdges` within the same transaction:
1. `DELETE FROM graph_edges WHERE workspace_hash=$1 AND source_file=$2`
2. Call `graphRegistry.ExtractEdges(filePath, content)`
3. Batch INSERT new edges

Edge extraction failures are **non-fatal** — logged as warnings, watcher continues.

### sqlc queries: `internal/storage/queries/graph.sql`

```sql
-- name: UpsertGraphEdge :exec
INSERT INTO graph_edges (workspace_hash, source_node, target_node, edge_type, source_file, metadata)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (workspace_hash, source_node, target_node, edge_type) DO UPDATE
    SET metadata = EXCLUDED.metadata, created_at = now();

-- name: DeleteGraphEdgesByFile :exec
DELETE FROM graph_edges WHERE workspace_hash = $1 AND source_file = $2;

-- name: GetOutgoingEdges :many
SELECT * FROM graph_edges
WHERE workspace_hash = $1 AND source_node = $2
  AND ($3::text = '' OR edge_type = $3)
ORDER BY edge_type, target_node;

-- name: GetIncomingEdges :many
SELECT * FROM graph_edges
WHERE workspace_hash = $1 AND target_node = $2
  AND ($3::text = '' OR edge_type = $3)
ORDER BY edge_type, source_node;

-- name: GraphStats :one
SELECT
    COUNT(*) FILTER (WHERE edge_type = 'contains') AS contains_count,
    COUNT(*) FILTER (WHERE edge_type = 'imports')  AS imports_count,
    COUNT(*) FILTER (WHERE edge_type = 'calls')    AS calls_count
FROM graph_edges WHERE workspace_hash = $1;
```

### MCP tool: `memory_graph`

Input:
```json
{
  "workspace": "hash",
  "node": "internal/watcher/watcher.go",
  "direction": "out",
  "edge_type": ""
}
```

`direction`: `"out"` (outgoing), `"in"` (incoming), `"both"`.  
`edge_type`: optional filter (`"contains"`, `"imports"`, `"calls"`, or `""` for all).

### REST endpoint: `POST /api/v1/graph/query`

Same parameters as MCP tool. Returns:
```json
{
  "node": "internal/watcher/watcher.go",
  "direction": "out",
  "edges": [
    {"source": "internal/watcher/watcher.go", "target": "internal/symbol", "type": "imports", "line": 12},
    ...
  ]
}
```

## Performance

- All graph queries are single-hop (no recursive CTEs in v1)
- Indexes on `(workspace_hash, source_node, edge_type)` and `(workspace_hash, target_node, edge_type)` ensure O(log n) lookups
- Default result limit: 200 edges per query
- Edge extraction adds ~5-20ms per file (tree-sitter parse + DB write) — acceptable for watcher pipeline

## Error Handling

- Edge extraction failures: non-fatal warning log, watcher continues
- Tree-sitter parse errors: skip file, log warning
- DB upsert conflicts: handled by `ON CONFLICT DO UPDATE`
- All actions logged per `mọi action đều cần có log` constraint
