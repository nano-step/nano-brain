# Design — Import Target Resolution

## Problem recap

Extraction stores raw specifiers; storage stores them verbatim; lookup queries
with a canonicalized node. Three representations, no intersection. Fix =
canonicalize the import edge `target_node` at index time so it equals the node
identity that `resolveNodeAgainstWorkspace` produces for the same file.

## Code map (from grounding investigation)

| Concern | Location |
|---|---|
| Edge struct (`TargetNode string`) | `internal/graph/edge.go:15-26` |
| Raw spec stored — TS | `typescript_extractor.go` `extractImports` (~175-182), `walkRequire` (~213-220) |
| Raw spec stored — JS | `javascript_extractor.go` `extractImports` (~139-146), `walkRequireJS` (~171-178) |
| Dedup (no normalization) | `registry.go` `extractWith` (~178-197) |
| Upsert convergence point | `internal/watcher/watcher.go` upsert loop (~250+) → `UpsertGraphEdge` |
| Reverse-lookup match (literal) | `graph.sql` `GetIncomingEdges`, `GetImpactorsByTargets` |
| Query-node canonicalizer | `internal/mcp/tools.go` `resolveNodeAgainstWorkspace` (~1669, ~1876) |

## Approach

Resolve in **one place** (watcher upsert loop), not the 4 extractor sites, so
every language/extractor benefits and the extractors stay dumb (raw spec → edge).

```
for each edge:
  if edge.Kind == EdgeImports:
     edge.TargetNode = ResolveImportPath(edge.TargetNode, edge.SourceFile, workspaceRoot)  // fallback: raw on error
  UpsertGraphEdge(... TargetNode ...)
```

### `internal/graph/import_resolver.go`

```go
func ResolveImportPath(spec, sourceFile, workspaceRoot string) (string, error)
func IsBarePackage(spec string) bool      // no leading ./ ../ ~ @/  → npm pkg, return as-is
func IsRelativeImport(spec string) bool   // ./ or ../
func IsAliasImport(spec string) bool      // ~ , ~/ , @/  (framework alias)
```

Resolution steps:
1. **Bare package** → return `spec` unchanged (not a workspace file).
2. **Relative** → `filepath.Join(dir(sourceFile), spec)`, then canonicalize.
3. **Alias** → map prefix via alias table to a base dir, join remainder.
4. **Extension/index probing** → a spec rarely has an extension. Resolve to the
   real file by probing `.ts .js .vue .mjs .ts→index.ts …` against the known file
   set (prefer the in-memory/index file list over disk stat for speed). If no
   match, fall back to the joined path without extension (still better than raw).
5. **Canonicalize** to the exact form `resolveNodeAgainstWorkspace` emits.

### Alias table source

- Nuxt: `~` and `@` → project root; under Nuxt 4 `srcDir`, → `app/`. Read from
  `nuxt.config.ts` (or `.nuxt/tsconfig.json` `compilerOptions.paths`, which Nuxt
  generates and which already encodes the correct mapping).
- Generic Vite/TS: `tsconfig.json` / `jsconfig.json` `compilerOptions.paths`.
- Resolve once per workspace (cache), not per edge.

## CRITICAL decision — canonical form (linchpin)

The resolved `target_node` MUST byte-equal the node identity used elsewhere:
- the same file's `source_node` form, and
- the output of `resolveNodeAgainstWorkspace(node)`.

The in-flight change `fix-edge-source-file-relative-paths` normalizes
`source_node`/`source_file` to **workspace-relative**. Strong signal: canonical
form = **workspace-relative** (e.g. `utils/enums.js`, or `app/utils/enums.js`
under Nuxt 4). **Implementer MUST read `resolveNodeAgainstWorkspace` and match its
output exactly** — a mismatch silently leaves reverse lookup broken (the failure
mode looks identical to "no dependents").

## Open questions for deep-design

1. Workspace-relative vs absolute — confirm against `resolveNodeAgainstWorkspace` and the sibling change. (Leaning workspace-relative.)
2. Extension probing against disk vs the indexed file set — perf + correctness tradeoff.
3. Nuxt 4 `srcDir=app/` mapping — does `~` point at root or `app/`? Derive from `.nuxt/tsconfig.json` rather than hard-coding.
4. Reindex mechanism — reuse existing reindex endpoint/path; full vs incremental.
5. Coordination/merge order with `fix-edge-source-file-relative-paths` to avoid canonical-form drift.

## Perf

Resolution is string ops + a cached alias table + a file-set lookup — O(1) per
edge. No network. Acceptable at index time; does not touch query latency.
