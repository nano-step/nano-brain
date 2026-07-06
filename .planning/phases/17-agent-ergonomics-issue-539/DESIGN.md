# Phase 17 — Agent-ergonomics fixes (issue #539)

Lane: high-risk · Change type: user-feature (agent-facing MCP surface) + bug-fix
Issue: nano-step/nano-brain#539 (agent field report, 4 findings)

## Goal

Make nano-brain's graph/navigation + retrieval MCP surface usable by an autonomous
agent without falling back to raw grep/Read. Address all four findings from #539.

## Finding 1 — memory_trace nodes unaddressable / builtin-polluted / collision-prone

**Root cause (confirmed in code):** every language extractor stores the *target* of a
`calls` edge as a **bare identifier** (`internal/graph/javascript_extractor.go:241`
`TargetNode: callee`; `internal/graph/go_extractor.go:212` same). SourceNode is
qualified (`file::symbol`) but TargetNode is not. There is **no builtin/third-party
filter** anywhere in `internal/graph`. `memory_trace`
(`internal/mcp/tools.go:1833-1844`) emits `e.TargetNode` verbatim, so:
- 1a nodes are bare names, not addressable back into memory_get/graph/impact;
- 1b builtins (`push`, `all`, `Math.min`, `JSON.stringify`, ramda helpers) appear as nodes;
- 1c bare names collide — `GetOutgoingEdgesBySymbol` matches by name suffix, inventing
  phantom callers/cycles across files that share a method name.

**Fix (query-time resolution, retroactive, no re-index):** in `memory_trace`, resolve
each bare target name to workspace symbol(s) via a symbol-name lookup, then:
- resolves to ≥1 intra-workspace symbol → emit fully-qualified `file::symbol` node(s)
  (traversal continues via the `::` path — `GetOutgoingEdges`, not the name-suffix
  query — which also kills the collision/phantom-cycle bug);
- resolves to 0 → the target is external (language builtin or third-party) → **drop it**
  unless `include_external=true` (new optional param, default false);
- when a name resolves to multiple files, emit each candidate as its own node with an
  `ambiguous: true` marker so the agent sees the fan-out instead of a false single edge.
- Each emitted chain item gains `name` (bare) alongside the qualified `node`, and honors
  the existing `paths=relative` styling.

Deeper scope-aware call resolution at *extraction* time (so the stored edge itself is
qualified) is #501's domain (import resolution feeds call resolution) — noted as the
follow-up, not attempted here. Query-time resolution fully addresses what the agent
observes without a re-index.

### Acceptance criteria (F1)
- AC-1: `memory_trace` result `chain[].node` is `file::symbol` (or workspace-relative
  when `paths=relative`) for every intra-workspace target; re-feedable into memory_get/graph.
- AC-2: builtins/third-party absent from the chain by default; present only with
  `include_external=true`.
- AC-3: two same-named methods in different files produce distinct nodes; no phantom
  re-entry of the entry symbol via a name-only edge.

## Finding 2 — memory_get "#<uuid>" only resolves document_id; chunk id → leaked SQL error

**Root cause:** `internal/mcp/tools.go:1174-1183` — the `#` branch calls only
`GetDocumentByID`. Search results lead with `id` = **chunk** id (struct at
`tools.go:212-213`). Passing it fails at `tools.go:1200`
`fmt.Sprintf("document not found: %v", err)` — leaks `sql: no rows in result set`.
`GetChunkByID` (`internal/storage/queries/chunks.sql:33`) already exists.

**Fix:** in the `#` branch, when `GetDocumentByID` misses, fall back to `GetChunkByID`
→ resolve `chunk.DocumentID` → `GetDocumentByID`. If both miss, return a clear,
non-SQL error: `"#<id> matched no document or chunk in workspace <ws>"`. Never surface
the raw `sql:` string.

### Acceptance criteria (F2)
- AC-4: `memory_get({path:"#<chunk-id>"})` returns the parent document.
- AC-5: `memory_get({path:"#<doc-id>"})` still returns the document (no regression).
- AC-6: an unknown `#<uuid>` returns an actionable message with no leaked `sql:` text.

## Finding 3 — symbol body not retrievable; memory_symbols lacks line span

**Root cause:** symbols are persisted with `Content: s.Signature` only
(`internal/watcher/watcher.go:961`); the `symbol.Symbol` struct
(`internal/symbol/symbol.go:17`) has only `Line` (start), no end line, and the persisted
metadata (`watcher.go:949-953`) omits line info. `memory_symbols`
(`tools.go:2020-2027`) therefore can emit neither body nor span.

**Fix:**
- Add `EndLine int` to `symbol.Symbol`; every extractor in `internal/symbol/*` populates
  `Line` (start) and `EndLine` from the tree-sitter node span.
- Persist `line`/`end_line` into the symbol document metadata (`watcher.go` symbol upsert).
- `memory_symbols` emits `start_line`/`end_line`.
- `memory_get` on a `file?symbol=Name&kind=K` path (or a plain file path with
  start/end) returns the **full body span** from the parent file document, not just the
  signature. Implementation: detect the `?symbol=` doc, read `line`/`end_line` from its
  metadata, look up the parent file document by the path before `?symbol=`, slice
  `[line,end_line]`.

Existing workspaces need a re-index for the new metadata to appear (watcher re-extracts
on file change or via `memory_update`); documented, not a blocker.

### Acceptance criteria (F3)
- AC-7: `memory_symbols` entries include numeric `start_line` and `end_line`.
- AC-8: `memory_get` on a symbol path returns the full definition body (> signature line)
  for a freshly-indexed symbol.

## Finding 4 — hybrid memory_query latency 18–58s vs 82ms BM25

**Root cause hypothesis:** the per-query embedding round-trip to Ollama dominates (empty
queue rules out embed backlog; BM25-only path is 82ms). Fix = a small in-process
query-embedding cache (LRU keyed by provider+model+text) so repeated/similar orientation
queries skip the Ollama round-trip, plus verify the embed HTTP client reuses connections
and has a sane timeout.

Cannot reproduce the reporter's 19k-doc/Ollama environment locally; deliver the cache +
a unit test proving cache hits skip the provider, and document the profiling method.
Deeper pooling/batching folds into the #372/#373 perf track.

### Acceptance criteria (F4)
- AC-9: a query-embedding cache exists; a repeated identical query embeds once
  (unit test asserts provider called once across two identical queries).
- AC-10: no behavior regression in search results (existing search tests green).

## Non-goals / deferred (tracked, commented on #539)
- Extraction-time scope-aware call resolution (→ #501).
- pgvector/Ollama connection-pool + batch tuning (→ #372/#373).
