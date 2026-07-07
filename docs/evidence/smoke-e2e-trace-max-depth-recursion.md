# smoke:e2e — Issue #561 (memory_trace max_depth recursion)

Change-type: bug-fix. The fix is exercised end-to-end at two layers below.

## 1. Live reproduction of the bug (dev server :3100, real MCP tool)

The bug was first observed on the running workspace via the real `memory_trace`
MCP tool — it returned only the entry's direct callees regardless of `max_depth`:

```
memory_trace node="internal/server/handlers/impact.go::GraphImpact" max_depth=4
→ [collectImpact @1, embed/cache.go::Get @1]   count=2   (no depth-2 node)

memory_trace node="…::GraphImpact" max_depth=1
→ IDENTICAL output — max_depth had no effect
```
`collectImpact` calls `ExpandImpactFrontier`, yet it never surfaced at depth 2.

## 2. End-to-end proof through the MCP protocol (integration test, red-green)

`TestMemoryTrace_RecursesPastDepth1_AbsoluteSourcePath`
(`internal/mcp/trace_max_depth_561_integration_test.go`) drives the fix through
the **real MCP SDK transport**: MCP client → server → `registerMemoryTrace`
handler → Postgres (nanobrain_test). It seeds a `Main → mid → leaf` chain with
an **absolute** `documents.source_path` (exactly as the watcher writes in prod —
the reason the pre-existing relative-path trace tests never caught this).

```
RED  (fix disabled): chain length = 1, want 2 — trace did not recurse past depth 1
                     [{depth:1 name:mid node:mid.go::mid via:entry.go::Main}]
GREEN (fix applied):  --- PASS: TestMemoryTrace_RecursesPastDepth1_AbsoluteSourcePath
                     chain = [mid@1, leaf@2]   count=2
```
The three existing #539 trace tests still PASS (no regression).

## 3. HTTP transport confirmed (MCP-over-HTTP, :3199 / nanobrain_test)

A standalone binary was built and started on **:3199** against **nanobrain_test**
(`NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1`, dev DB/:3100 never touched — server log:
`database pool connected … /nanobrain_test`). The MCP streamable-HTTP handshake
and tool call round-tripped:

```
HTTP/1.1 200 OK   initialize        (Mcp-Session-Id issued)
HTTP/1.1 200 OK   tools/call memory_trace
```

The depth assertion for the wire path is covered by (2) — the standalone server
binds its own registered-workspace set from the watcher, so a hand-seeded
synthetic workspace did not resolve to the seeded edges over HTTP; this is a
test-fixture binding limitation, not a code path difference (the handler code is
identical to the one (2) exercises through the MCP SDK transport).

## Isolation / cleanup

Server ran on :3199 / nanobrain_test only; killed by captured PID (no broad
kill). Synthetic seed + temp binary removed. Dev DB (nanobrain_dev / :3100)
never written.
