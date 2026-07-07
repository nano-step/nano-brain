# smoke:e2e — Issue #575 (#542 F2: trace bare-call proximity)

Change-type: bug-fix. Verified over the real MCP-over-HTTP transport on an
isolated **:3199 / nanobrain_test** server (dev DB / :3100 never touched).

## Setup (isolated)

Seeded into nanobrain_test: a workspace (64-hex hash), two symbol docs both
titled `foo` (`metadata.source_type=symbol`) — `backend/svc.go` and
`frontend/util.js` (the cross-repo collision) — and a `calls` edge
`backend/ctrl.go::Main → foo` with `source_file="backend/ctrl.go"` (the caller).
Started a standalone server on :3199 (`NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1`,
`NANO_BRAIN_DATABASE_URL=…/nanobrain_test`).

## MCP streamable-HTTP handshake + tool call

```
HTTP/1.1 200 OK   initialize            (Mcp-Session-Id issued)
HTTP/1.1 200 OK   tools/call memory_trace  node="backend/ctrl.go::Main" max_depth=2
```

Decoded `foo` chain nodes:

```
[ {node:"backend/svc.go::foo", ambiguous:false} ]
```

**Result:** the bare call `foo` — defined in BOTH `backend/svc.go` and
`frontend/util.js` — resolves to the **backend** definition only (shares the
`backend/` prefix with the caller), with `ambiguous:false`. The frontend
collision is dropped. **Before this fix** both `backend/svc.go::foo` and
`frontend/util.js::foo` were emitted, each `ambiguous:true` — the cross-repo
pollution reported in #542 F2.

## Isolation / cleanup

Server on :3199 / nanobrain_test only; killed by captured PID (no broad kill).
Seeded workspace/docs/edge deleted, temp binary removed.
