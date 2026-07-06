# Self-Review — Issue #501 (JS/TS/Vue import resolution, Phase 2 PR-B)

## Actions Taken

- New `internal/graph/import_resolver.go`: `ResolveImportTarget` + `AliasIndex` /
  `BuildAliasIndex` / `ImportContext`. Resolves relative (`./`,`../`) and aliased
  (`~/`,`@/`, tsconfig/jsconfig `compilerOptions.paths`) JS/TS/Vue import
  specifiers to workspace-relative paths; bare packages pass through unchanged.
- G3: nearest-ancestor config resolution — `AliasIndex` maps each source file to
  its nearest tsconfig/jsconfig, so per-repo `~/` roots don't cross repo
  boundaries (verified with two-config fixture repo-a/repo-b).
- G2: resolved path canonicalized to the stored `source_node` format + existence
  check with `.ts/.js/.vue/.tsx/.jsx` + `/index.*` fallback; unresolved →
  fall back to raw specifier; path-escape clamp drops paths above the root. Raw
  specifier retained in `Edge.Metadata`.
- Threaded via `registry.go` (`ExtractEdgesForFrameworksWithImports` + `ImportContext`,
  old `ExtractEdgesForFrameworks` preserved) and `watcher.go` (per-collection
  `sync.Map` alias-index cache) into the 3 JS/TS/Vue extractors — the shared
  `Extractor` interface for the other ~17 extractors was NOT changed.

## Files Changed

- new: `internal/graph/import_resolver.go` (+ `_test.go`), `internal/mcp/import_resolution_501_integration_test.go`, `internal/graph/testdata/import-fixture/{repo-a,repo-b}/**`.
- modified: `internal/graph/{javascript,typescript,vue_sfc}_extractor.go`, `internal/graph/registry.go`, `internal/watcher/watcher.go`.

## Findings Summary

- Root cause #501: extractors stored the raw specifier as `imports` target →
  reverse-impact on the real file found nothing. Fixed at extraction time.
- No schema change (`target_node TEXT` fits). **Backfill of existing rows is a
  POST-MERGE ops step** (`memory_update` / `ReextractEdgesForWorkspace` per
  workspace), documented — not code in this PR; watcher self-heals on file change.
- Out of scope (flagged): worktree/path duplication (#501 related finding);
  cross-repo bare-name collision (root-cause C).

## Resolution Status

- All in-scope items resolved; no unresolved critical/major.
- PR-B's own tests PASS: 7 `ResolveImportTarget_*` unit tests + import-resolution
  integration test. build exit 0; `go test -race -short ./...` 31 pkgs ok.
- Pre-existing integration failures (9, identical on master: Express/Rail/Ruby*,
  MemoryGraph_Relative*/MemoryTrace_Relative*) are NOT introduced by this PR —
  verified by diffing the FAIL set against `origin/master`. Repo-health item,
  out of scope. Independent review: `docs/evidence/review-501.md`. Smoke:
  `docs/evidence/smoke-e2e-js-ts-import-resolution.md`.

## Gemini Verification Triage

PR #555. Gemini review COMMENTED (non-blocking state); 5 inline comments, all
legitimate — all fixed in commit f16ef75.

| Comment ref | Agent verdict | Reasoning | Action |
| --- | --- | --- | --- |
| PR#555 `import_resolver.go:124` | VALID:high | Real tsconfig semantics: non-wildcard `paths` keys must match the specifier EXACTLY, only wildcard-derived keys prefix-match. `longestAliasMatch` used `HasPrefix` for all → wrong for exact mappings (fixture only had wildcards so tests missed it). | fixed in commit f16ef75 (+ `TestResolveImportTarget_ExactVsWildcardAliasMatch`) |
| PR#555 `import_resolver.go:395` | VALID:high | `stripTrailingCommas` didn't track string literals → could corrupt a tsconfig string value containing `,}`/`,]`. Low real-world trigger but a genuine JSONC-parser flaw; fix is cheap+correct. | fixed in commit f16ef75 (+ `TestBuildAliasIndex_TolerantJSONCPreservesCommaInsideString`) |
| PR#555 `watcher.go:1025` | VALID:medium | Cold-cache stampede: concurrent callers all run heavy `BuildAliasIndex` before `LoadOrStore`. Switched to `sync.Once`-per-entry so it builds exactly once per dir. | fixed in commit f16ef75 |
| PR#555 `import_resolver.go:312` | VALID:medium | `parseTSConfigPaths` target join didn't normalize backslashes (Windows-authored tsconfig) before `path.Join`. | fixed in commit f16ef75 |
| PR#555 `import_resolver.go:256` | VALID:low | Reading a nil map doesn't panic in Go (Gemini agrees), so defensive-only; but the `idx.aliasMaps == nil` early return is 2 lines and closes the comment cleanly. | fixed in commit f16ef75 |
