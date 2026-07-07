# Self-Review — Issue #561 (memory_trace ignores max_depth)

Change-type: bug-fix · Lane: tiny · Branch: `fix/trace-max-depth-recursion`
Author: kokorolx.

## Actions Taken

- Fixed `registerMemoryTrace` (`internal/mcp/tools.go`) so the downstream
  call-chain BFS recurses to `max_depth` instead of stopping at depth 1.
- Root cause: resolved calls-edge targets are qualified from
  `documents.source_path`, which is **absolute** in production
  (`/root/mid.go?symbol=mid`). The BFS enqueued that absolute key and looked it
  up against `graph_edges.source_node`, which is workspace-**relative**
  (`mid.go::mid`). Exact-match `GetOutgoingEdges` missed on every hop after the
  first, so only the entry's direct callees were ever returned.
- Fix: strip the workspace root from the traversal key at query time
  (`lookupNode := stripWorkspacePrefix(wsRoot, cur.node)`), so each hop matches
  the relative stored `source_node`. `wsRoot` is now computed once before the
  loop (was computed lazily only for relative output). Display behavior is
  unchanged — chain nodes stay absolute under the default `paths=absolute` and
  are relativized only when `paths=relative`, exactly as before.

## Files Changed

- `internal/mcp/tools.go` — 3 surgical edits in `registerMemoryTrace`
  (compute `wsRoot`; strip at lookup; drop the now-redundant lazy `wsRoot`).
- `internal/mcp/trace_max_depth_561_integration_test.go` — new regression test
  seeding an **absolute** source_path (the case the pre-existing relative-path
  trace tests missed) and asserting depth-2 recursion.

## Findings Summary

- Ground-truth verified against the DB: `documents.source_path` is absolute,
  `graph_edges.source_node` is relative — the exact mismatch the fix addresses.
- **Red-green proven**: with the fix reverted the new test fails at
  `chain length = 1` (the #561 symptom); with the fix it passes at count=2
  (mid@1, leaf@2).
- No regression: the 3 existing #539 trace tests still PASS; `go test -race
  -short ./...` green across 31 packages.
- `TestMemoryTrace_RelativeInputAndOutput` panics — confirmed **pre-existing**
  (#556) by reproducing it on the clean tree with my changes stashed; unrelated
  to this fix (it uses relative fixtures the strip leaves untouched).

## Resolution Status

- In scope resolved. No critical/major issues.
- `CGO_ENABLED=0 go build ./...` → clean. `go vet ./internal/mcp/` → clean.
- Unit: `go test -race -short ./...` → 31 pkgs ok.
- Integration (trace subset, nanobrain_test): new test + 3 existing trace tests PASS.
- smoke:e2e: `docs/evidence/smoke-e2e-trace-max-depth-recursion.md`.

## Gemini Verification Triage

Gemini: COMMENTED, CI pass, MERGEABLE/CLEAN. One inline comment.

| Comment ref | Agent verdict | Reasoning | Action |
| --- | --- | --- | --- |
| tools.go:1910 [medium] — `seen`-map key asymmetry (relative entry seed vs absolute child keys) lets a cycle back to the entry re-list it | VALID | Same item R88 raised (LOW). Bounded (no loop) but a real correctness nit in the exact traversal being fixed; two reviewers flagged it. | **Fixed** — dedup now keys on the workspace-relative form (`stripWorkspacePrefix(wsRoot, qualified)`) while keeping the absolute node for display. Locked by new red-green test `TestMemoryTrace_CycleBackToEntry_NotReListed_AbsoluteSourcePath` (RED: Main re-listed at depth 2; GREEN: deduped). |
