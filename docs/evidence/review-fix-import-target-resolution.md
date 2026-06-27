# Review Gate — fix-import-target-resolution (US-501)

## Review Verdict: PASS

Reviewer: verify-501-final (independent verifier sub-agent; not the author/implementer impl-501)
Review panel (independent, R88): rev-correctness, rev-security, rev-tests, rev-regression — all distinct from author impl-501.
Author/implementer: impl-501
Date: 2026-06-27
Lane: high-risk (data-model hard gate)

All checks re-run against a real DB (`nanobrain_test`), not cached.

| Acceptance Criterion | Evidence | Status |
|---|---|---|
| AC1 canonicalizer → relative | graph_paths.go diff + `graph_paths_test.go` (relative); mcp integration `...MatchesRelativeStorage`/`...OutputIsRelative` pass | ✓ |
| AC2 target_node byte-matches source_node | `import_resolver_canonical_test.go` drives real extractor + real LoadAliasMap + os.Stat; asserts `utils/enums.ts` == stored source_node | ✓ |
| AC3 reverse lookup 0→N after reindex | `TestReindexResolvesImportEdges_EndToEnd` PASS; smoke:e2e `graph/impact utils/enums.ts` → `[composables/useThing.ts]` | ✓ |
| AC4 scoped/bare passthrough | `TestResolveImportPath/scoped_npm_package_passes_through` + classifier tests | ✓ |
| AC5 unresolved → raw, warn, no drop | resolver raw-fallback branches + unit cases (miss/ambiguous/extensionless/`..`) | ✓ |
| AC6 relative `./`/`../` resolve | unit tests pass | ✓ |
| AC7 reindex idempotent | `TestReindexResolvesImportEdges_EndToEnd` second run count stable; raw alias row gone; smoke `~/utils/enums` → null | ✓ |
| AC8 build+short+integration+smoke green | build OK; `-short` all pass; integration mcp+watcher+graph-import pass; smoke:e2e through HTTP pass | ✓ |

## Prior findings — resolved
- C1 (CRITICAL, stale absolute-seed integration test) — re-seeded relative + renamed; mcp integration green.
- H1 (HIGH, AC3/AC7 unproven) — new watcher integration test proves 0→N + idempotency.
- Security MED (bare `".."` traversal guard) — guarded at import_resolver.go:92 + unit case.
- M1 (dead `.vue` fixture) — removed.
- L1/L2/L3 — accepted as documented limitations.

## Known / out of scope (not regressions)
- 6 `internal/graph` integration tests (Express/Rails/Ruby flow) fail with `dial tcp localhost:3199 connection refused` — pre-existing, require a live server; unrelated to #501 (diff does not touch those paths).
- `export … from` / dynamic `import()` extraction, Nuxt auto-imports — explicitly out of scope (separate follow-ups).

## Rollout note (mandatory)
Existing workspaces must be migrated via **force-wipe `POST /api/v1/reindex`** or **`POST /api/v1/update`** per workspace; a plain incremental reindex on an unchanged tree is a no-op. See design.md.

`Review Verdict: PASS`
