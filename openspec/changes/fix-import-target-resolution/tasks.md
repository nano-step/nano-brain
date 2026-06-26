## 1. Confirm canonical form (do FIRST — linchpin)

- [ ] 1.1 Read `internal/mcp/tools.go` `resolveNodeAgainstWorkspace`; document the exact node form it emits (workspace-relative? extension included?)
- [ ] 1.2 Read `fix-edge-source-file-relative-paths` change + current `source_node` values; confirm canonical form is consistent
- [ ] 1.3 Write the canonical form into design.md "CRITICAL decision" as resolved

## 2. Resolver helper

- [ ] 2.1 Create `internal/graph/import_resolver.go` with `IsBarePackage`, `IsRelativeImport`, `IsAliasImport`
- [ ] 2.2 Implement relative resolution (`filepath.Join(dir(sourceFile), spec)` → canonical)
- [ ] 2.3 Implement alias resolution from cached alias table (Nuxt `~`/`@`, tsconfig/jsconfig `paths`); read `.nuxt/tsconfig.json` when present
- [ ] 2.4 Implement extension/`index` probing against the workspace file set; fallback to joined path on miss
- [ ] 2.5 `ResolveImportPath` returns raw spec unchanged for bare packages and on any error
- [ ] 2.6 Unit tests (table-driven): bare pkg passthrough, `./`/`../`, `~/`, `@/`, missing-extension probe, unresolvable→raw fallback

## 3. Wire into indexing

- [ ] 3.1 Build + cache the per-workspace alias table at watch/index start
- [ ] 3.2 In the watcher upsert loop, resolve `EdgeImports` `target_node` before `UpsertGraphEdge` (graceful fallback + warn log)
- [ ] 3.3 Confirm bare-package and non-import edges are untouched

## 4. Reindex + verification

- [ ] 4.1 Reindex a test workspace so `target_node` values become resolved
- [ ] 4.2 Reverse-lookup assertion: an aliased file's canonical path returns its dependents (0 → N) via `GetIncomingEdges` / `memory_impact`
- [ ] 4.3 `go build ./... && go test -race -short ./...`
- [ ] 4.4 `go test -race -tags=integration ./...`
- [ ] 4.5 smoke:e2e — start server on :3199 (nanobrain_test), index a small fixture with an alias import, `memory_graph(node=<real file>, direction=in)` returns the importer

## 5. Review + ship

- [ ] 5.1 Self-review evidence (`docs/evidence/self-review-fix-import-target-resolution.md`)
- [ ] 5.2 Independent review gate (R88) — spawned reviewer, verdict PASS (`docs/evidence/review-fix-import-target-resolution.md`)
- [ ] 5.3 PR → `master`, `Closes #501`; triage Gemini comments
