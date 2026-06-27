## 1. Canonicalizer → workspace-relative (foundation)

- [ ] 1.1 In `internal/mcp/graph_paths.go`, change `resolveNodeAgainstWorkspace` to return the workspace-relative form (absolute input → strip root via `stripWorkspacePrefix` logic; relative → unchanged; extensionless/non-path → unchanged)
- [ ] 1.2 Update `graph_paths_test.go` (`TestResolveNodeAgainstWorkspace_*`) to expect relative output
- [ ] 1.3 Audit the 3 call sites (tools.go 1669/1771/1876) + confirm `stripWorkspacePrefix` on output is now a harmless no-op (no double-strip bug)
- [ ] 1.4 `go test -race -short ./internal/mcp/` green

## 2. Resolver helper

- [ ] 2.1 Create `internal/graph/import_resolver.go`: `IsBarePackage`, `IsRelativeImport`, `IsAliasImport` per the decision table (incl. `@/` alias vs `@org/` scoped-pkg rule)
- [ ] 2.2 `ResolveImportPath(spec, sourceFile, workspaceRoot, alias, exists)` → workspace-relative; bare/scoped passthrough; relative joined against source dir
- [ ] 2.3 Alias map loader with precedence: `tsconfig`/`jsconfig` `paths` → `.nuxt/tsconfig.json` (if present) → Nuxt `srcDir` convention; cache per workspace
- [ ] 2.4 Extension/index probing via injected `exists` predicate; **raw passthrough on any miss/ambiguity** (never extensionless half-path)
- [ ] 2.5 Unit tests (table-driven): `./`,`../`, `~/`, `@/`, `@org/pkg` (passthrough), bare pkg, `lodash/fp`, miss→raw, ambiguous→raw, extensionless→raw

## 3. Wire into indexing

- [ ] 3.1 Build + cache per-workspace alias map at index start
- [ ] 3.2 In `extractAndUpsertEdges` (watcher), resolve `EdgeImports` `target_node` before upsert, with an `os.Stat`-backed memoized `exists` closure rooted at the workspace
- [ ] 3.3 Confirm non-import edges + bare/scoped specs are untouched; confirm both live-watch (~750) and bulk (~1099) paths go through the resolver

## 4. Reindex + verification

- [ ] 4.1 Reindex a fixture workspace via `POST /api/v1/reindex` (`ReextractEdgesForWorkspace`) — NOT reindex-cfg
- [ ] 4.2 Reverse-lookup assertion: a fixture file imported via `~/` alias returns its importer through `GetIncomingEdges` for the **workspace-relative** path (0 → N); confirm no raw/resolved duplicates (delete-by-source_file tx)
- [ ] 4.3 Unit test pinning canonical form: `resolveNodeAgainstWorkspace` output == stored `source_node` form == resolver output (all relative), byte-equal
- [ ] 4.4 `go build ./... && go test -race -short ./...`
- [ ] 4.5 `go test -race -tags=integration ./...`
- [ ] 4.6 smoke:e2e — server on :3199 (nanobrain_test), index `internal/graph/testdata/alias-import/` fixture, `memory_graph(node=<rel file>, direction=in)` returns the importer; paste curl evidence

## 5. Review + ship

- [ ] 5.1 Self-review evidence (`docs/evidence/self-review-fix-import-target-resolution.md`)
- [ ] 5.2 Independent review gate (R88) — spawned reviewer ≠ author, verdict PASS (`docs/evidence/review-fix-import-target-resolution.md`)
- [ ] 5.3 PR → `master`, `Closes #501`; triage Gemini comments

## Fixtures (committed, gate-3.11 safe — no real workspace names)

- `internal/graph/testdata/alias-import/`: `tsconfig.json` (paths `~/*`→`./*`), `utils/enums.ts`, `composables/useThing.ts` importing `~/utils/enums`, a `.vue` importing `~/composables/useThing` — proves 0→N without any private workspace.
