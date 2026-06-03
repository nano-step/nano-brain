---
name: nano-brain code intelligence
description: Use nano-brain CLI context, code-impact, and detect-changes for symbol-level analysis, impact checks, and diff mapping.
---

# Code Intelligence (nano-brain)

## Overview

Run code intelligence when symbol relationships, impact analysis, or diff-to-symbol mapping is needed. Ensure indexing is done first via `reindex`.

The CLI's `reindex` command requires `--root=<path>` (validates non-empty at the client), but the daemon's `/api/v1/reindex` handler scopes by the request body's `workspace` field, not `root`. So always pass `--workspace` explicitly OR rely on `workdir` to pick `$PWD` and resolve the matching workspace:

```bash
# ✅ Both work
npx nano-brain reindex --root=/path/to/workspace --workspace=<hash>
bash(command="npx nano-brain reindex", workdir="/path/to/workspace")
```

## code_context — 360-degree symbol view

Return callers, callees, cluster membership, execution flows, and infrastructure connections for any function/class/method.

```
npx nano-brain context "handleRequest"
npx nano-brain context "helper" --file=/src/utils.ts
```

Use `file_path` to disambiguate when multiple symbols share the same name.

## code_impact — dependency analysis

Traverse the symbol graph to find affected symbols and flows. Return a risk level (LOW/MEDIUM/HIGH/CRITICAL).

```
npx nano-brain code-impact "DatabaseClient" --direction=upstream
npx nano-brain code-impact "processOrder" --direction=downstream --max-depth=3
```

- `upstream` = "who calls this?" (callers, consumers)
- `downstream` = "what does this call?" (callees, dependencies)

## code_detect_changes — git diff to symbol mapping

Map current git changes to affected symbols and execution flows.

```
npx nano-brain detect-changes --scope=staged
npx nano-brain detect-changes --scope=all
```

Scopes: `unstaged`, `staged`, `all` (default).

## When to Use Code Intelligence vs Memory vs Native Tools

| Question | Tool |
|----------|------|
| "What calls function X?" | `npx nano-brain context <name>` |
| "What breaks if I change X?" | `npx nano-brain code-impact <name> --direction=...` |
| "What did I change and what's affected?" | `npx nano-brain detect-changes --scope=...` |
| "Have we done this before?" | `npx nano-brain query "..."` |
| "Find exact string in code" | grep / ast-grep |
| "How does auth work conceptually?" | `npx nano-brain vsearch "..."` |

Code intelligence requires indexing. Run `npx nano-brain reindex` (with `workdir` set to the workspace) first if the workspace has not been indexed.

## HTTP API equivalents

| CLI | HTTP endpoint | Body shape |
|---|---|---|
| `context <name>` | `POST /api/v1/graph/query` | `{"workspace":"<hash>","node":"<name>","direction":"out","edge_type":"calls"}` |
| `code-impact <name>` | `POST /api/v1/graph/impact` | `{"workspace":"<hash>","node":"<name>","edge_type":"calls","max_depth":2}` |
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
npx nano-brain symbols --type=function --name=init --workspace="$WS"
# Pick the one you want from the returned (file_path, line) tuples
npx nano-brain context init --file=/path/to/specific/file.go
```
