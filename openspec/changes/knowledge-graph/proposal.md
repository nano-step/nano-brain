# Knowledge Graph — Proposal

**Status**: proposed  
**Lane**: high-risk  
**GitHub Issue**: #175  
**Date**: 2026-05-25  

## Problem

nano-brain stores symbols (definitions) but has no model for **relationships between symbols and files**. Users cannot answer:

- "What files import `internal/symbol`?"
- "What functions does `processFile` call?"
- "What symbols are defined in `watcher.go`?"

The symbol index is a flat list; the knowledge graph adds the edges that connect it into a queryable dependency model.

## Proposed Solution

Add a `graph_edges` table tracking three relationship types:

| Edge Type | Example |
|-----------|---------|
| `contains` | `watcher.go` → `processFile` (file defines symbol) |
| `imports` | `watcher.go` → `internal/symbol` (file imports package) |
| `calls` | `watcher.go::processFile` → `extractSymbols` (function calls function, best-effort) |

Extraction runs in the watcher pipeline immediately after symbol extraction — same file event, same transaction (delete stale edges, insert new edges). A new `internal/graph/` package provides the extraction interface and Go language extractor. MCP tool `memory_graph` and REST `POST /api/v1/graph/query` expose the data.

## Scope (v1)

**In scope:**
- `graph_edges` PostgreSQL table (migration 00008)
- Go language extractor: `contains` (from existing symbol data), `imports` (raw import strings, no path resolution), `calls` (best-effort callee name matching)
- Watcher integration: `WithGraphRegistry()`, `extractAndUpsertEdges()` after symbols
- MCP tool: `memory_graph` (query by node + direction + edge_type)
- REST: `POST /api/v1/graph/query`
- Tests: unit (Go extractor) + integration (watcher→DB round-trip)

**Out of scope (v2+):**
- TS/Python/JS extractors
- Import path resolution (raw strings only in v1)
- Recursive/transitive call chain traversal
- Graph visualization
- Type-aware method call resolution

## Risk Classification

High-risk lane triggers:
- New data model (`graph_edges` table — migration required)
- New public API contract (REST + MCP surface)
- Watcher pipeline modification (touches core ingestion path)

## Success Criteria

1. `go build ./...` passes with CGO_ENABLED=0
2. `go test -short ./...` passes, including new graph extractor tests
3. After watcher processes nano-brain's own source:
   - `memory_graph(node="internal/watcher/watcher.go", edge_type="imports", direction="out")` returns ≥5 edges
   - `memory_graph(node="internal/watcher/watcher.go", edge_type="contains", direction="out")` returns ≥10 edges
4. No regressions in existing MCP tool tests
