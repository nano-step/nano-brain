## Why

Import-graph edges store the **raw module specifier** as `target_node`
(`~/utils/enums`, `./foo`, `ramda`) and never resolve it to a workspace file.
Reverse-lookup SQL matches `target_node` **literally**
(`graph.sql` `GetIncomingEdges` / `GetImpactorsByTargets`), while the query node
is first canonicalized by `resolveNodeAgainstWorkspace` (`internal/mcp/tools.go`).
The two forms never intersect, so `memory_impact` and `memory_graph(direction=in)`
return **0** for every alias/relative intra-repo import.

Proven on a live workspace (see `docs/evidence/vue-sfc-design/agent-validation.md`,
issue #501): the alias node `~/utils/enums` has **274** incoming edges; the real
file's canonical path has **0**. For an agent this is a dangerous false-negative:
"what breaks if I change this?" answers "nothing" while 274 files depend on it.
This blocks the Vue SFC feature (its Vue import edges would inherit the same bug).

## What Changes

- **New `internal/graph/import_resolver.go`** — `ResolveImportPath(spec, sourceFile, workspaceRoot)` plus predicates `IsBarePackage` / `IsRelativeImport` / `IsAliasImport`.
  - Bare packages (`vue`, `ramda`, `@babel/core`) pass through unchanged (not workspace files).
  - Relative (`./`, `../`) resolved by joining against the source file's directory.
  - Alias (`~`, `@/`) resolved via the project alias map (Nuxt `~`/`@` → root / `srcDir`; `tsconfig`/`jsconfig` `paths`).
  - Output is the **canonical node form produced by `resolveNodeAgainstWorkspace`** (workspace-relative; coordinate with the in-flight `fix-edge-source-file-relative-paths` change normalizing `source_node`).
- **Resolve at the single convergence point** — the watcher upsert loop, before `UpsertGraphEdge`, for `EdgeImports` edges. Graceful fallback to the raw spec on resolution failure (logged).
- **Reindex** existing edges so stored `target_node` values become resolved.

## Capabilities

### Modified Capabilities

- `multi-language-graph-extractors`: import edge targets are resolved to canonical node identities so reverse traversal (`memory_impact`, `memory_graph(in)`) returns real dependents.

## Impact

- **Files**: new `internal/graph/import_resolver.go`; `internal/watcher/watcher.go` (upsert loop). Coordinate canonical form with `fix-edge-source-file-relative-paths` (source side).
- **Data**: existing `graph_edges.target_node` are raw specifiers → reindex required.
- **API**: no contract change — `memory_impact` / `memory_graph(in)` simply start returning correct results.
- **Risk**: (a) per-edge resolution cost at index time (perf); (b) canonical-form mismatch with `resolveNodeAgainstWorkspace` would silently keep reverse lookup broken — the implementer MUST read that function and match its output exactly; (c) extension/`index` probing ambiguity. These are deep-design items.
- **Breaking**: No — bare-package and unresolved specs fall back to current behavior.
