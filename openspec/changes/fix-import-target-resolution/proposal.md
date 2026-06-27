## Why

Reverse graph traversal (`memory_impact`, `memory_graph direction=in`) returns
**0** for alias/relative intra-repo imports. Two independent root causes, both
confirmed by reading the code and a live workspace (issue #501,
`docs/evidence/vue-sfc-design/agent-validation.md`):

1. **Unresolved targets** — import edges store the raw module specifier in
   `Edge.TargetNode` (`~/utils/enums`, `./foo`) at 4 extractor sites; nothing
   resolves them to a workspace file.
2. **Canonical-form contradiction** — the query path canonicalizes a node to an
   **absolute** path (`resolveNodeAgainstWorkspace`, `internal/mcp/graph_paths.go:45`
   → `path.Join(ws.Path, filePart)`), but edge storage is **workspace-relative**
   since #450 (merged). The literal SQL match (`graph.sql`
   `target_node = $2 OR split_part(target_node,'::',2)=$2`) never intersects.
   This latently breaks *forward* lookups on freshly-indexed data too.

Proof: alias node `~/utils/enums` has 274 incoming edges; the real file's
canonical (absolute) query form has 0. For an agent this is a dangerous
false-negative. It also blocks the Vue SFC feature (Vue import edges inherit it).

## What Changes

Unify the graph node identity to **workspace-relative, end-to-end**, and resolve
import targets to that form.

- **Canonicalizer → relative** (`internal/mcp/graph_paths.go`): change
  `resolveNodeAgainstWorkspace` to return the workspace-relative form (strip the
  root via the existing `stripWorkspacePrefix` logic) instead of joining to
  absolute. Update `graph_paths_test.go` (currently asserts absolute). Completes
  the #450 migration; `stripWorkspacePrefix` on output becomes a harmless no-op.
- **New `internal/graph/import_resolver.go`** — `ResolveImportPath(spec, sourceFile, workspaceRoot, exists func(string) bool)` plus `IsBarePackage` / `IsRelativeImport` / `IsAliasImport`. Output is **workspace-relative**, byte-matching stored `source_node`.
  - Bare / scoped npm packages (`ramda`, `@org/pkg`) pass through unchanged.
  - Relative (`./`,`../`) resolved against the source file's dir.
  - Alias (`~`,`~/`,`@/`) resolved via a cached per-workspace alias map.
  - **On any ambiguity or miss → keep the raw spec. Never guess** (a wrong edge is worse than a raw one).
- **Resolve at the single convergence point** — `extractAndUpsertEdges` (watcher), for `EdgeImports` edges, before upsert.
- **Reindex** via `POST /api/v1/reindex` (`ReextractEdgesForWorkspace`) so stored targets become resolved. (NOT `/reindex-cfg`, which writes only flowcharts.)

## Capabilities

### Modified Capabilities

- `multi-language-graph-extractors`: import edge targets are resolved to canonical workspace-relative node identities, and the query canonicalizer uses the same relative form, so reverse traversal returns real dependents.

## Impact

- **Files**: `internal/mcp/graph_paths.go` (+ test); new `internal/graph/import_resolver.go`; `internal/watcher/watcher.go` (`extractAndUpsertEdges`).
- **Data**: existing `graph_edges` carry raw/absolute forms → **reindex required**, manual. Use **force-wipe `POST /api/v1/reindex` or `POST /api/v1/update`** per workspace (a plain *incremental* reindex on an unchanged tree is a no-op — see design.md rollout note). Idempotent: edges are deleted by `source_file` (both rel+abs variants) then re-upserted in one tx — no raw/resolved duplicates.
- **Blast radius**: the canonicalizer is used by every graph tool (`memory_graph`/`impact`/`trace`, tools.go:1669/1771/1876) → all benefit; all must be reverified.
- **Out of scope (stated, not silently)**: extraction of `export … from` re-exports and dynamic `import()` (no edge created today → resolution can't help; separate follow-up); Nuxt auto-imported components/composables (no import statement → the Vue SFC work, not this).
- **Breaking**: no API contract change. Behavior change: reverse lookups start returning correct results after reindex.
