# Design — Import Target Resolution + Canonical Node Form

## Deep-design synthesis (2026-06-26)

Reviewed by two independent agents (architecture + scope/risk) plus orchestrator
verification against the code. Verdict: the original design was **half wrong**
(canonical form specified backwards) and is revised here. Decision confirmed with
the user: **workspace-relative node identity, end-to-end.**

## Two root causes (both verified in code)

1. **Raw targets.** Import edges store the raw specifier in `Edge.TargetNode`
   (`internal/graph/edge.go:15-26`), created at 4 sites:
   `typescript_extractor.go` `extractImports` (~175) + `walkRequire` (~213),
   `javascript_extractor.go` `extractImports` (~139) + `walkRequireJS` (~171).
   Only `strings.Trim(raw, "\"'`")` is applied.
2. **Canonical-form contradiction.** Query canonicalizer
   `resolveNodeAgainstWorkspace` (`internal/mcp/graph_paths.go:33-45`) returns
   **absolute** (`path.Join(ws.Path, filePart)`) for any extensioned relative
   node; extensionless nodes pass through unchanged. Storage is **relative**
   since #450 (merged + archived; `watcher.go:907-909` passes `relFile`).
   `graph_paths_test.go:30` still asserts absolute. The output layer
   (`stripWorkspacePrefix`, used at tools.go:1731-1961) strips absolute→relative,
   revealing the system was *built around absolute storage* and #450 only
   half-migrated it. Literal SQL match (`graph.sql` `GetIncomingEdges` /
   `GetImpactorsByTargets`) therefore never intersects raw/absolute vs relative.

## Decision: workspace-relative, end-to-end

Make stored `source_node`, stored (resolved) `target_node`, and the query
canonicalizer all agree on **workspace-relative**. Rationale: #450 already moved
storage to relative (merged precedent); relative removes the absolute leakage of
user home paths into node ids; and it makes `stripWorkspacePrefix` on output a
no-op instead of a correction.

### Change 1 — canonicalizer to relative (`internal/mcp/graph_paths.go`)

`resolveNodeAgainstWorkspace` currently joins to absolute. Change it to return the
workspace-relative form: absolute input → strip root (reuse `stripWorkspacePrefix`
logic); already-relative → unchanged; non-path / extensionless tokens → unchanged
(import specifiers like `context` stay as-is). Update `graph_paths_test.go`
(flip expected from absolute to relative). Audit the 3 call sites (tools.go
1669/1771/1876) and confirm `stripWorkspacePrefix` on output is now redundant but
harmless. This fixes the latent forward-lookup break too.

## Code map (verified)

| Concern | Location |
|---|---|
| Edge struct (`TargetNode string`) | `internal/graph/edge.go:15-26` |
| Raw spec stored (TS) | `typescript_extractor.go` ~175, ~213 |
| Raw spec stored (JS) | `javascript_extractor.go` ~139, ~171 |
| Dedup (no normalization) | `registry.go` `extractWith` ~178-197 |
| Upsert convergence + delete-by-source_file tx | `watcher.go` `extractAndUpsertEdges` (~750 live, ~1099 bulk), upsert ~935, delete ~922-946 |
| Only non-watcher edge writers | Ruby resolver (calls/reconcile only, ~1308-1347) — not imports |
| Reverse-lookup match (literal) | `graph.sql` `GetIncomingEdges`, `GetImpactorsByTargets` |
| Query canonicalizer (→ absolute, the bug) | `internal/mcp/graph_paths.go:33-45` |
| Output stripper (→ relative) | `stripWorkspacePrefix`, tools.go:1731-1961 |
| Reindex entry (re-extracts edges) | `POST /api/v1/reindex` → `ReextractEdgesForWorkspace` (`watcher.go:1069` ← `reindex.go:375`) |

> `reindex-cfg` writes ONLY flowcharts (`reindex_cfg.go` processFile 234-265), zero `graph_edges`. Do not use it for this migration.

### Change 2 — `internal/graph/import_resolver.go`

```go
// exists lets tests inject a file-set; prod uses an os.Stat-backed closure.
func ResolveImportPath(spec, sourceFile, workspaceRoot string, alias AliasMap, exists func(relPath string) bool) string
func IsBarePackage(spec string) bool
func IsRelativeImport(spec string) bool
func IsAliasImport(spec string) bool
```

Classification decision table (order matters):

| Spec shape | Class | Action |
|---|---|---|
| `./x`, `../x` | relative | join against `dir(sourceFile)`, probe ext, → workspace-relative |
| `~/x`, `~x`, `@/x` | alias | map prefix via alias table → base dir, join remainder, probe ext |
| `@org/pkg`, `@org/pkg/sub` | scoped npm | **bare passthrough** (NOT alias — the rule: `@/` is alias, `@<word>/` is a scoped package) |
| `lodash`, `lodash/fp`, `vue` | bare npm | passthrough |
| anything unresolved after probing | — | **raw passthrough + warn** |

Extension probing (no in-memory file set exists at upsert; N1): try in order
`.ts .tsx .js .jsx .vue .mjs .cjs`, then `<spec>/index.{ts,tsx,js,jsx,vue}`,
using the injected `exists` predicate (prod: `os.Stat` under `workspaceRoot`,
memoized per reindex). **On probe miss → return the raw spec, never an
extensionless half-path** (an extensionless node also mismatches the canonicalizer
— graph_paths.go:38-40 — so a half-resolved path silently breaks again).

### Alias map source (N3, precedence)

1. `tsconfig.json` / `jsconfig.json` `compilerOptions.paths` (committed, authoritative).
2. `.nuxt/tsconfig.json` if present (generated; often absent at index time — git-ignored).
3. Nuxt convention fallback: `~`/`@` → `srcDir` (Nuxt 4 `app/`, Nuxt 3 root). Derive `srcDir` from the paths map, do not hard-code.

Resolve the alias map once per workspace (cache), not per edge.

## Resolution flow (watcher)

```
for each edge in extractAndUpsertEdges:
  if edge.Kind == EdgeImports:
     edge.TargetNode = ResolveImportPath(edge.TargetNode, edge.SourceFile, root, aliasCache, exists)  // raw on miss
  UpsertGraphEdge(... TargetNode ...)   // existing delete-by-source_file tx handles idempotency
```

## Perf (corrected — N1)

No in-memory file set exists at the upsert point (`watchedCollection` holds only
dirPath/filter). Probing is `os.Stat` (≈6–12 stats per *unresolved* edge),
memoized per reindex via the `exists` closure. Bare/scoped packages and
already-resolved specs skip probing. Acceptable at index time; zero query-path
cost. The earlier "O(1) / in-memory list" claim was wrong and is removed.

## Out of scope (explicit)

- `export … from` re-exports and dynamic `import()`: **not extracted today** →
  no edge to resolve. Extending extraction is a separate follow-up (note the gap;
  don't let the 0→N gate be read as covering these).
- Nuxt auto-imported components/composables: no import statement → belongs to the
  Vue SFC work, not #501.
- Monorepo workspace packages (`@org/ui` → `packages/ui`): left raw (known limit).

## Risks

- False resolution > raw: guarded by deterministic config-driven mapping + raw
  fallback on any ambiguity/miss. Never disk-guess a "plausible" path.
- Migration window: until a **full** re-extraction runs per workspace, reverse
  lookup stays broken; after, old rows are replaced (delete-by-source_file purges
  both relative+absolute variants). Migration is **manual**, no auto-migration.

  **Rollout note (mandatory — verified by regression review).** Not every reindex
  mode migrates: a plain *incremental* `POST /api/v1/reindex` on an unchanged tree
  is a **no-op** (it only rescans content-hash-changed collections). Operators MUST
  migrate each existing workspace via **force-wipe `POST /api/v1/reindex`** (sets
  dirty → full walk) **or `POST /api/v1/update`** (`ReextractEdgesForWorkspace`,
  walks all files unconditionally). Both route through `extractAndUpsertEdges` →
  the resolver.
