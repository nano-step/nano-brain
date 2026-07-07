# Self-Review — Issue #556 (9 red integration tests on master)

Change-type: test-only / infrastructure · Lane: normal · Branch: `fix/red-integration-suite`
Author: kokorolx.

## Root causes (3 independent clusters, verified by direct reproduction)

- **Cluster A (5 tests, `internal/graph/ruby_integration_test.go`)** —
  `TestRailRouteExtraction`, `TestRubyCrossFileResolution`, `TestRubyReconcileEdges`,
  `TestRubyClassIndex`, `TestRubyFlowEndToEnd`. These are acceptance tests: they
  HTTP into a **live server on `:3199` pre-indexed with the rails-app fixture**
  under a hardcoded workspace hash — not a self-contained integration fixture.
  With no server running they fail with `connection refused`, which looks like
  a regression but isn't one. Fixed with a `skipIfServerUnreachable(t)` guard
  (probes `/health` with a 2s timeout) so they skip honestly instead of failing
  when no live server is present, matching the existing `testing.Short()` skip
  pattern already in each test.
- **Cluster B (1 test, `internal/graph/express_extractor.go`)** —
  `TestExpressExtractor_Integration` expected ≥2 middleware edges, got 1. Real
  extractor bug: `app.use(corsMiddleware)` / `app.use(loggerMiddleware)` (a
  single-argument global middleware registration) was misread by `tsExtractPath`
  as an unresolvable path argument (`"<var:corsMiddleware>"`) and the whole call
  was dropped — no HTTP edge, no middleware edge. Fixed by special-casing
  single-arg `use()` calls in `extractHTTP`: when the lone argument isn't a
  string/template literal, resolve it as a middleware name and emit
  `EdgeMiddleware` (`mw -> <receiver>`) instead of falling through to path
  extraction. `router.post('/posts', authMiddleware, postController.create)`
  (multi-arg, existing path) is untouched.
- **Cluster C (3 tests, `internal/mcp/graph_paths_integration_test.go`)** —
  `TestMemoryGraph_RelativeNodeInputResolvesToAbsolute`,
  `TestMemoryGraph_RelativeOutputStripsPrefix`,
  `TestMemoryTrace_RelativeInputAndOutput` (the last panics the whole test
  binary: `interface conversion: interface{} is nil`). Root cause: the shared
  `setupGraphMCP` helper seeded graph edges with **absolute** (workspace-prefixed)
  node IDs, but `normalizeNodeForQuery` always normalizes an agent's input node
  to **workspace-relative** before querying — the old resolve-to-absolute
  behavior is explicitly deprecated in `graph_paths.go`. A relative query can
  never match an absolute-stored edge, so results were empty. Fixed the test
  helper to seed workspace-relative node IDs, matching the real watcher
  convention already documented and used by the sibling `setupFindingsMCP`
  (`internal/mcp/agent_ergonomics_539_integration_test.go:24-28`).
  `TestMemoryGraph_RelativeOutputStripsPrefix` specifically exercises the
  `paths=relative` prefix-strip feature; since the query node is always
  normalized to relative, an edge's *source* can never be absolute-stored and
  reachable — only a *target* can carry a legacy absolute prefix (e.g. data
  indexed before the relative convention). Rewrote that one test to seed
  exactly that asymmetric shape (relative source, one absolute-prefixed target)
  so the strip logic is exercised realistically instead of vacuously.

## Files Changed

- `internal/graph/express_extractor.go` — single-arg `use()` middleware fix.
- `internal/graph/ruby_integration_test.go` — `skipIfServerUnreachable` guard,
  applied to all 5 acceptance tests.
- `internal/mcp/graph_paths_integration_test.go` — `setupGraphMCP` seeds
  workspace-relative edges; `TestMemoryGraph_RelativeOutputStripsPrefix`
  rewritten with a realistic legacy-absolute-target seed; all 6 call sites
  updated for the new `(ctx, q, wsHash, wsPath, session, callTool)` signature.

## Red-green proof

- Cluster A: reproduced `connection refused` on all 5 before the fix; after the
  fix they `SKIP` cleanly with a descriptive reason (no live server in this run).
- Cluster B: reproduced `expected at least 2 middleware edges, got 1` before the
  fix; after the fix, 3 middleware edges extracted
  (`corsMiddleware`, `loggerMiddleware`, `authMiddleware`) — test PASS.
- Cluster C: reproduced `count=0` / `edges=0` / panic before the fix (matches
  #556 exactly); after the fix all 3 PASS, no panic, and the 6 sibling tests in
  the same file (`TestMemoryGraph_AbsoluteNodeInputUnchanged`,
  `TestMemoryGraph_InvalidWorkspaceHashErrorsClearly`,
  `TestMemoryImpact_RelativeInputAndOutput`, plus 3 unrelated trace tests) still
  pass unchanged.

## New finding (out of scope, filed separately)

While running the full `internal/mcp` integration package, found a 10th
pre-existing red test not in #556's original list:
`TestMemoryWakeUp_OnlyReturnsMemoryAndSessionSummaryDocs`
(`recent_memories len = 2, want 3`). Confirmed pre-existing and unrelated by
stashing this branch's changes and re-running against clean master — identical
failure. Not touched here (different root cause, different file, would blur
this PR's `Closes #556` scope); will file as a new issue.

## Resolution Status

- All 9 tests named in #556 now behave correctly: 5 skip honestly (no live
  server), 4 pass (real fixes).
- `go build ./...` clean; `go test -race -short ./...` all green (see below).
- `go test -tags=integration ./internal/graph/... ./internal/mcp/...` green
  (excluding the pre-existing, out-of-scope #10 above).
- No regression: full graph package unit tests pass; all other tests in
  `graph_paths_integration_test.go` unaffected.

## Gemini Verification Triage

| Comment ref | Agent verdict | Reasoning | Action |
| --- | --- | --- | --- |
| ruby_integration_test.go:51 (MEDIUM) — `resp.Body.Close()` should be `defer`red immediately after the error check, per idiomatic Go | VALID | Trivial style nit, no behavioral bug in this specific function (single call site, no other exit paths), but the idiomatic form is correct and cheap to apply. | FIXED — changed to `defer resp.Body.Close()`. |
