## Why

The `graph_edges` table and all MCP graph tools (`memory_graph`, `memory_trace`, `memory_impact`) are fully implemented at the query layer but return empty results because nothing populates the table. The Go graph extractor exists and produces `contains`, `imports`, and `calls` edges — but it is not wired into the watcher pipeline, and TypeScript, JavaScript, and Python extractors are missing entirely.

## What Changes

- Wire the existing Go graph extractor into the watcher file-processing pipeline (parallel to symbol extraction)
- Add graph extractors for TypeScript, JavaScript, and Python (tree-sitter queries for contains + imports + calls edges)
- Register all new language extractors in the watcher's graph registry
- Enhance call edge resolution: map unqualified callee names to `file::symbol` format using the workspace's contains edges
- Reindex endpoint deletes and repopulates graph edges alongside symbol documents

## Capabilities

### New Capabilities

- `graph-extraction-pipeline`: Watcher extracts and upserts `contains`, `imports`, and `calls` edges into `graph_edges` for Go, TypeScript, JavaScript, and Python files on index/reindex
- `ts-js-graph-extractor`: Tree-sitter graph extractors for TypeScript and JavaScript producing contains, imports (ES module), and calls edges
- `python-graph-extractor`: Tree-sitter graph extractor for Python producing contains, imports (top-level `import`/`from…import`), and calls edges
- `call-edge-resolution`: Post-extraction step that resolves unqualified callee names to fully-qualified `file::symbol` target nodes using the contains index built from the same file pass

### Modified Capabilities

- `status-symbol-graph`: Status output graph counts (`symbol_edges`, `calls_count`, `imports_count`, `contains_count`) will now reflect real data once the pipeline is wired

## Impact

- `internal/watcher/watcher.go`: add `graphRegistry` field, call `extractAndUpsertEdges()` alongside existing `extractAndUpsertSymbols()`
- `internal/symbol/` (new files): `ts_graph_extractor.go`, `js_graph_extractor.go`, `python_graph_extractor.go` (or under `internal/graph/`)
- `internal/graph/go_extractor.go`: fix call edge target format — emit `targetFile::calleeName` instead of bare `calleeName` where resolvable
- `cmd/nano-brain/main.go`: register new language graph extractors
- `internal/server/handlers/reindex.go`: extend reindex to also call `DeleteGraphEdgesByFile` + re-extract graph edges
- No schema changes required; `graph_edges` table and all queries already exist
- No breaking API changes; MCP tools and REST endpoints already return correct structure
