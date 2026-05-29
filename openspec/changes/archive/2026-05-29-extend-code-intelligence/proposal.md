# Extend Code Intelligence — Proposal

**Status**: proposed
**Lane**: normal
**GitHub Issue**: #196
**Date**: 2026-05-29
**Supersedes**: #174 (closed — symbol extraction already shipped), partially #153

## Problem

nano-brain's code intelligence is Go-only for graph extraction. Symbol extraction works for Go, TypeScript, JavaScript, and Python — but only Go has knowledge graph edges (imports, calls, contains). This means:

- `memory_impact` returns nothing for TypeScript/Python projects
- `memory_graph` has no edges for JS/TS/Python files
- `memory_trace` can't trace call chains in non-Go code
- CLI commands (`context`, `code-impact`, `detect-changes`) from V1 don't exist in V2

Most nano-brain users work on polyglot codebases (Go + TypeScript + Python). Graph extraction for Go-only covers ~30% of typical workspaces.

## Proposed Solution

### Part 1: Multi-language graph extractors

Add `TypeScriptGraphExtractor`, `JavaScriptGraphExtractor`, and `PythonGraphExtractor` to `internal/graph/`, following the exact pattern of `GoGraphExtractor`:

- **contains**: file → symbol (reuse existing symbol data)
- **imports**: file → module (ES imports, Python imports, require())
- **calls**: function → callee (best-effort name matching via tree-sitter queries)

All extractors use `gotreesitter` (already in go.mod, pure Go, CGO_ENABLED=0).
All write to existing `graph_edges` table — no migration needed.

### Part 2: CLI commands

Add 3 CLI commands wrapping existing REST endpoints:

| Command | Wraps | Description |
|---|---|---|
| `nano-brain context <symbol>` | `/api/v1/graph` + `/api/v1/symbols` | 360° view: callers, callees, containing file, imports |
| `nano-brain code-impact <symbol>` | `/api/v1/impact` | Upstream/downstream impact analysis |
| `nano-brain detect-changes [--staged\|--all]` | git diff + `/api/v1/symbols` + `/api/v1/impact` | Map changed lines → affected symbols → impact |

## Scope

**In scope:**
- TypeScript graph extractor (`.ts`, `.tsx`)
- JavaScript graph extractor (`.js`, `.jsx`)
- Python graph extractor (`.py`)
- Register all 3 in `cmd/nano-brain/main.go` alongside existing Go extractor
- CLI: `context`, `code-impact`, `detect-changes`
- Unit tests per-language extractor
- Integration test: watcher → DB round-trip for each language

**Out of scope:**
- Rust, C, Java, or other language extractors
- Import path resolution (raw strings only, matching Go extractor pattern)
- Type-aware method resolution
- Cross-workspace graph queries
- Graph visualization

## Risk Classification

3 risk flags (normal lane):
- **Public contracts**: new CLI commands
- **Existing behavior**: extending watcher pipeline with new extractors
- **Multi-domain**: graph, CLI, watcher packages

No hard gates — `graph_edges` table already exists, no migration needed.

## Success Criteria

1. `CGO_ENABLED=0 go build ./...` passes
2. `go test -race -short ./...` passes, including new extractor tests
3. After watcher processes a TypeScript project:
   - `memory_graph(node="src/index.ts", edge_type="imports", direction="out")` returns ≥1 edge
   - `memory_graph(node="src/index.ts", edge_type="contains", direction="out")` returns ≥1 edge
4. After watcher processes a Python project:
   - `memory_graph(node="main.py", edge_type="imports", direction="out")` returns ≥1 edge
5. `nano-brain context <symbol>` prints callers + callees + file
6. `nano-brain code-impact <symbol>` prints affected nodes
7. `nano-brain detect-changes` maps git diff to symbols
8. No regressions in existing tests
