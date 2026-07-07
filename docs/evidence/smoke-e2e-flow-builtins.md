# smoke:e2e — Issue #567 (#542 F8: memory_flow builtin pollution)

Change-type: bug-fix. Verified over the real MCP-over-HTTP transport on an
isolated **:3199 / nanobrain_test** server (dev DB / :3100 never touched).

## Setup (isolated)

Built the binary, seeded into **nanobrain_test**: a workspace (64-hex hash), an
HTTP route `GET /x → ctrl`, two `calls` edges `a.js::ctrl → realHelper` and
`a.js::ctrl → Number`, and a symbol document for `realHelper`
(`metadata.source_type=symbol`, `title=realHelper`) so it resolves — while
`Number` has no symbol. Started a flow-enabled server on :3199
(`NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1`, `NANO_BRAIN_FLOW_ENABLED=true`,
`NANO_BRAIN_DATABASE_URL=…/nanobrain_test`).

## MCP streamable-HTTP handshake + tool calls

```
HTTP/1.1 200 OK   initialize            (Mcp-Session-Id issued)
HTTP/1.1 200 OK   tools/call memory_flow  entry="GET /x"                        (default)
HTTP/1.1 200 OK   tools/call memory_flow  entry="GET /x" include_external=true
```

Decoded node names:

```
DEFAULT            → ["ctrl", "realHelper", "GET /x"]        (builtin "Number" DROPPED, real leaf kept)
include_external   → ["GET /x", "ctrl", "Number", "realHelper"]   (builtin present)
```

**Result:** the builtin `Number` (a bare `calls` target that resolves to no
workspace symbol) is dropped from the flow by default, while the real leaf
`realHelper` (which resolves) is kept; `include_external:true` brings the builtin
back. **Before this fix** `Number` appeared as a `RoleExternal` node in the
default output.

## Isolation / cleanup

Server on :3199 / nanobrain_test only; killed by captured PID (no broad kill).
Seeded workspace/edges/doc deleted, temp binary + fixture dir removed.
