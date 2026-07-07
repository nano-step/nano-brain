## Review Verdict: PASS

Reviewer: oh-my-claudecode:code-reviewer (R88 independent correctness gate, spawned; ≠ author).
Date: 2026-07-07
Branch: `fix/flow-builtins` · Issue #567 (split from #542 F8)

Change: `memory_flow` drops `RoleExternal` nodes that resolve to no workspace
symbol (JS builtins/keywords), via `dropExternalBuiltins` + `include_external`
param; mirrors `memory_trace`. Read-path only, `BuildFlow` untouched.

| Concern | Verdict |
|---|---|
| Drops only unresolved externals | PASS (HIGH) — real leaves keep their symbol doc → `len(matches)>0` → kept; builtins have no doc → dropped. Symbol-doc existence is the correct discriminator. |
| Role-gated removal / same-name collision | PASS (HIGH) — removal is `Role==External && dropName[Name]`; a non-external node sharing a name is never dropped. External IDs dedup by `nodeMap`. |
| Error handling | PASS (HIGH) — on query error `dropName=false` (kept); presence-check avoids re-query. |
| Slice rebuild `[:0:0]` | PASS (HIGH) — cap-0 forces fresh backing, no aliasing while ranging (the correct idiom). |
| Edge removal consistency | PASS (HIGH) — edge dropped iff an endpoint is a dropped external; edges between kept nodes survive; externals are leaves (no outgoing edges). |
| No regression | PASS (HIGH) — `include_external=true` skips the drop; stitched nodes are `RoleIntegration` (never dropped); runs before response mapping so #563/#564 behavior intact. |

Reviewer ran `go build ./...` + `go vet ./internal/mcp/` (clean). **0 blocking
issues.** Integration test confirmed PASS by author against nanobrain_test/:3199.

### Non-blocking findings
- **[LOW] N+1 resolver calls** (one per distinct external name, bounded by
  maxFanout×maxDepth). Acceptable for the flow read-path; a `title = ANY($names)`
  batch could collapse it if flow latency ever matters. Not changed.
- **[LOW] indexing completeness** — a real symbol not yet indexed resolves to 0
  and is dropped until indexing catches up (mirrors trace). **Addressed** —
  caveat added to the `include_external` tool description.
- **[LOW] test edge-removal gap** — **Addressed**: the test now asserts the edge
  to the dropped builtin is removed AND the edge to the kept leaf survives.
- **[LOW] wasted copy on no-builtin path** / test-file-untracked — noted; the
  test file is `git add`ed with this commit.

### smoke:e2e — PASS
`docs/evidence/smoke-e2e-flow-builtins.md` — MCP-over-HTTP on :3199 /
nanobrain_test: default → builtin `Number` dropped, real leaf `realHelper` kept;
`include_external:true` → `Number` returns. Dev DB never touched.
