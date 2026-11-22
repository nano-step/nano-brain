---
name: nano-brain code intelligence
description: Use nano-brain CLI context, code-impact, and detect-changes for symbol-level analysis, impact checks, and diff mapping.
---

# Code Intelligence (nano-brain)

## Overview

Run code intelligence when symbol relationships, impact analysis, or diff-to-symbol mapping is needed. Ensure the workspace is registered (`POST /api/v1/init` or `nano-brain init --root <path>`) and indexed — request an async watcher scan with `npx @nano-step/nano-brain reindex --root <path>` (space-separated `--root <path>` is required; verify ingestion via `GET /api/v1/stats?workspace=<hash>`). To re-extract symbols, graph edges, and flowcharts on already-indexed files after an upgrade without re-embedding, call `POST /api/v1/update` (`{"workspace":"<hash>"}`).

```bash
# ✅ Correct (space-separated --root <path>)
npx @nano-step/nano-brain reindex --root /data/workspaces/next-app

# ❌ Wrong (--root=<path> is rejected as an unknown flag)
npx @nano-step/nano-brain reindex --root=/data/workspaces/next-app
```

## code_context — 360-degree symbol view

Return callers, callees, cluster membership, execution flows, and infrastructure connections for any function/class/method.

```
npx @nano-step/nano-brain context "handleRequest"
npx @nano-step/nano-brain context "helper" --file=/src/utils.ts
```

Use `file_path` to disambiguate when multiple symbols share the same name.

## code_impact — dependency analysis

Traverse the symbol graph to find affected symbols and flows. Return a risk level (LOW/MEDIUM/HIGH/CRITICAL).

```
npx @nano-step/nano-brain code-impact "DatabaseClient" --direction=upstream
npx @nano-step/nano-brain code-impact "processOrder" --direction=downstream --max-depth=3
```

- `upstream` = "who calls this?" (callers, consumers)
- `downstream` = "what does this call?" (callees, dependencies)

## code_detect_changes — git diff to symbol mapping

Map current git changes to affected symbols and execution flows.

```
npx @nano-step/nano-brain detect-changes --scope=staged
npx @nano-step/nano-brain detect-changes --scope=all
```

Scopes: `unstaged`, `staged`, `all` (default).

## When to Use Code Intelligence vs Memory vs Native Tools

| Question | Tool |
|----------|------|
| "What calls function X?" | `npx @nano-step/nano-brain context <name>` |
| "What breaks if I change X?" | `npx @nano-step/nano-brain code-impact <name> --direction=...` |
| "What did I change and what's affected?" | `npx @nano-step/nano-brain detect-changes --scope=...` |
| "Have we done this before?" | `npx @nano-step/nano-brain query "..."` |
| "Find exact string in code" | grep / ast-grep |
| "How does auth work conceptually?" | `npx @nano-step/nano-brain vsearch "..."` |

Code intelligence requires indexing. Run `npx @nano-step/nano-brain reindex --root <path>` first if the workspace has not been indexed, and poll `GET /api/v1/stats?workspace=<hash>` until `docs_total` and `graph_edges_by_type` populate.

## HTTP API equivalents

| CLI | HTTP endpoint | Body shape |
|---|---|---|
| `context <name>` | `POST /api/v1/graph/query` | `{"workspace":"<hash>","node":"<name>","direction":"out","edge_type":"calls"}` |
| `code-impact <name>` | `POST /api/v1/graph/impact` | `{"workspace":"<hash>","node":"<name>","edge_type":"calls","max_depth":2}` → `{node, impacted: [], node_found}` (`impacted` is never `null`; `node_found` distinguishes a leaf node from an unknown/unindexed node, #625) |
| `detect-changes` | Client-side git diff + parallel `graph/query` calls | (computed by CLI) |

`direction`: `in` (callers, who depends on this) or `out` (callees, what this depends on). The CLI maps `--direction=upstream` → `direction=in` and `--direction=downstream` → `direction=out`.

`edge_type` values seen in production: `calls`, `imports`, `references`, `extends`, `implements`. Omit to match any.

## Limits

- `max_depth` is clamped to `[1, 3]` server-side. Deeper traversals are too expensive for hot-path use.
- Graph is built from Tree-sitter AST extractors for: Go, TypeScript, JavaScript, Python (as of PR #197). Other languages return empty result sets.
- Cross-file graph edges require the source file to be in the workspace's indexed scope (respects `.gitignore`).

## Symbol disambiguation

When multiple symbols share a name (`init`, `handler`, etc.), the graph query returns ALL matches. Filter client-side via `file_path` matching, or use `memory_symbols` to enumerate options first:

```bash
npx @nano-step/nano-brain symbols --type=function --name=init --workspace="$WS"
# Pick the one you want from the returned (file_path, line) tuples
npx @nano-step/nano-brain context init --file=/path/to/specific/file.go
```
