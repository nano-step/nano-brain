---
name: nano-brain code intelligence
description: Use nano-brain MCP tools (memory_graph, memory_impact, memory_trace, memory_symbols) for symbol-level analysis and impact checks.
---

# Code Intelligence (nano-brain)

## Overview

Code intelligence answers symbol-level questions: callers, callees, blast radius of a change, and how a function call chain unfolds. All four tools require the workspace to be indexed — if the graph is empty, the daemon's watcher hasn't picked up source files yet (check `memory_status.queue_pending`).

All MCP tools take `workspace` (the SHA-256 hash returned by `memory_workspaces_resolve` or `POST /api/v1/init`). Node identifiers can be:
- A file path: `/abs/path/to/file.go`
- A symbol within a file: `/abs/path/to/file.go::FunctionName`
- A bare symbol name: `FunctionName` (graph returns ALL matches across files — disambiguate via `memory_symbols` first)


## memory_graph — 1-hop symbol neighbors

Returns direct callers, callees, imports, or containment edges for a node.

```
required: workspace, node
optional: direction ("out" | "in" | "both", default "out")
          edge_type ("calls" | "imports" | "contains" | empty for all)
returns:  {node, direction, edges: [{source, target, edge_type}]}
```

- `direction="out"` → "what does this node call/import?" (callees, dependencies)
- `direction="in"` → "what calls/imports this node?" (callers, consumers)
- `direction="both"` → union of in + out

Source: `internal/mcp/tools.go:916-1007`. Use this for quick "who touches this?" lookups before deciding whether to do a deeper impact analysis.


## memory_impact — reverse impact analysis

"What breaks if I change this symbol?" — walks INCOMING edges (callers, importers) breadth-first.

```
required: workspace, node
optional: edge_type ("calls" | "imports" | empty for all)
          max_depth (1-3, server-clamped, default 1)
returns:  {node, impacted: [{node, depth, edge_type}]}
```

Read the `impacted` array length as a rough risk signal:
- `<5` → LOW (localized change)
- `5-15` → MEDIUM (touches multiple call sites; review carefully)
- `>15` → HIGH (treat as breaking change; needs migration plan)

Source: `internal/mcp/tools.go:1087-1157`. Run BEFORE any refactor of a heavily-used symbol.


## memory_trace — forward call chain

"What does this entry point eventually call?" — walks OUTGOING edges with cycle detection.

```
required: workspace, node
optional: max_depth (1-10, server-clamped, default 5)
returns:  {entry, chain: [{node, depth, via}]}
```

Source: `internal/mcp/tools.go:1009-1085`. Use to understand execution flow from an HTTP handler down to its leaf dependencies, or from any entry point down to actual business logic.


## memory_symbols — find symbols by name/kind

Disambiguation tool: when multiple symbols share a name, list all matches first.

```
required: workspace
optional: query (substring filter), kind ("function" | "method" | "type" | "interface" | "struct" | "const" | "var"),
          limit (default 50, capped 200)
returns:  {symbols: [{name, kind, language, signature, source_path}], count}
```

Source: `internal/mcp/tools.go:1159-1223`. Pick the right `source_path` from the result, then pass `<source_path>::<name>` as the `node` to `memory_graph` / `memory_impact` / `memory_trace`.


## When to use code intelligence vs memory vs native tools

| Question | Tool |
|---|---|
| "What calls function X?" | `memory_graph(node="X", direction="in")` |
| "What does X call?" | `memory_graph(node="X", direction="out")` |
| "What breaks if I change X?" | `memory_impact(node="X", max_depth=2)` |
| "Trace the full call chain from entry point X" | `memory_trace(node="X", max_depth=5)` |
| "Where is symbol named X defined?" | `memory_symbols(query="X")` |
| "Have we done this before / past decisions on X?" | `memory_query("X decision")` |
| "Find exact string in current files" | grep / ast-grep |
| "How does auth work conceptually?" | `memory_vsearch("auth flow")` |


## HTTP equivalents

For scripts, dashboards, or non-MCP clients:

| MCP tool | HTTP endpoint | Body shape |
|---|---|---|
| `memory_graph` | `POST /api/v1/graph/query` | `{"workspace":"<hash>","node":"<name>","direction":"out","edge_type":"calls"}` |
| `memory_impact` | `POST /api/v1/graph/impact` | `{"workspace":"<hash>","node":"<name>","edge_type":"calls","max_depth":2}` |
| `memory_trace` | `POST /api/v1/graph/trace` | `{"workspace":"<hash>","node":"<name>","max_depth":5}` |
| `memory_symbols` | `POST /api/v1/symbols` | `{"workspace":"<hash>","query":"<substr>","kind":"function","limit":50}` |

Workspace overview (top-N most-connected nodes) is HTTP-only: `POST /api/v1/graph/overview` with `{"workspace":"<hash>","mode":"code","limit":50}`.


## Limits

- `max_depth` on `memory_impact` is clamped to `[1, 3]` server-side. Deeper traversals are too expensive for hot-path use.
- `max_depth` on `memory_trace` is clamped to `[1, 10]` with cycle detection.
- Graph is built from Tree-sitter AST extractors for: Go, TypeScript, JavaScript, Python (as of PR #197). Other languages return empty result sets.
- Cross-file graph edges require the source file to be in the workspace's indexed scope (respects `.gitignore`).
- If the graph is empty for a recently-added file, the watcher may still be processing it — wait, or trigger a reindex via `POST /api/v1/reindex`.


## Symbol disambiguation

When multiple symbols share a name (`init`, `handler`, `New`, etc.), `memory_graph` with a bare name returns ALL matches. Two strategies:

**1. Enumerate first, then target:**
```
memory_symbols(workspace=$WS, query="init", kind="function")
// → pick the right {source_path, name} from results
memory_graph(workspace=$WS, node="/path/to/specific/file.go::init", direction="in")
```

**2. Filter client-side:**
Run `memory_graph` with the bare name, then filter `edges` by `source_path` prefix matching the file you care about.

Prefer strategy 1 — it's cheaper and the intent is explicit.
