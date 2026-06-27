# Self-Review: fix-import-target-resolution (US-501)

Date: 2026-06-27
Author (implementer): impl-501 (executor sub-agent)
Self-review by: orchestrator + 4-agent review panel (rev-correctness, rev-security, rev-tests, rev-regression) — all independent of the author.

## Deep-design (pre-implementation)

Two independent reviewers (architecture `oracle-501`, scope/risk `metis-501`) + orchestrator verification found the original design's canonical form was backwards (query→absolute via `graph_paths.go:45`, storage→relative since #450). Decision (user-confirmed): **workspace-relative end-to-end**. Design revised before implementation.

## Review panel findings & resolution

| # | Severity | File | Description | Status |
|---|----------|------|-------------|--------|
| C1 | CRITICAL | internal/mcp/graph_paths_integration_test.go | Integration suite seeded ABSOLUTE node form → failed after canonicalizer flip (count=0, want 2) | FIXED — re-seeded relative; tests renamed (`...MatchesRelativeStorage`, `...OutputIsRelative`); assertions flipped. mcp integration green. |
| H1 | HIGH | (new) internal/watcher/import_resolution_integration_test.go | AC3 (0→N) + AC7 (idempotency) had no DB-level test | FIXED — `TestReindexResolvesImportEdges_EndToEnd`: extract→resolve→UpsertGraphEdge→GetIncomingEdges; asserts importer found, raw alias row gone, count stable on re-run. |
| S-MED | MEDIUM | internal/graph/import_resolver.go | Traversal guard missed bare `".."` (probes one dir above root) | FIXED — guard blocks `basePath == ".."`; unit case added. |
| M1 | MEDIUM | testdata/alias-import/pages/Thing.vue | Dead fixture (no Vue extractor) implying coverage | FIXED — file + empty dir removed. |
| L1 | LOW | import_resolver.go | `.js`-specifier→`.ts` (NodeNext) not resolved | ACCEPTED — documented known limitation; safe raw fallback. |
| L2 | LOW | import_resolver.go | `~~weird` enters alias path then raw-falls-back | ACCEPTED — harmless; comment added. |
| L3 | LOW | graph_paths.go | absolute-not-under-root passes through unchanged | ACCEPTED — intentional + tested. |

## Reviewer verdicts (production code)
- correctness: production code CORRECT (canonical byte-equality, alias `@/`-vs-`@org/`, traversal guard, probe determinism, watcher wiring all PASS); FAIL was test-level only (C1/H1) → resolved.
- security: PASS (path traversal blocked, DoS bounded, tsconfig parse safe, mutex safe); 1 MED (`..`) → resolved.
- tests: FAIL on AC3/AC7 coverage → resolved by H1.
- regression: PASS; canonicalizer flip has no consumer needing absolute; all 3 write paths route through resolver; mandatory rollout note added to design.md/proposal.md.

## Verification run (orchestrator, against nanobrain_test)
- `go build ./...` → OK
- `go test -race -short ./...` → all packages ok
- `go test -race -tags=integration ./internal/mcp/... ./internal/watcher/...` → ok (C1 + H1 pass)
- `go test -race -tags=integration ./internal/graph/...` → import/resolve/canonical pass; only the 6 Express/Rails/Ruby flow tests fail with `dial tcp localhost:3199 connection refused` (pre-existing, need a live server; unrelated to #501 — diff does not touch those paths).
- smoke:e2e (server :3199, nanobrain_test, isolated): `POST /api/v1/graph/impact node="utils/enums.ts"` → `impacted:[composables/useThing.ts (imports)]` (0→N); `node="~/utils/enums"` → `impacted:null` (no duplicate raw edge). Server torn down, production :3100 untouched.

## Summary
- Critical: 1 found, 1 fixed
- High: 1 found, 1 fixed
- Medium: 2 found, 2 fixed
- Low: 3 found, 0 fixed (3 accepted as documented limitations)
- All AC1–AC8 satisfied with evidence.
