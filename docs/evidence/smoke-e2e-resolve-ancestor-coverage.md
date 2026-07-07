# smoke:e2e — Issue #565 (#542 F5: resolve ancestor coverage)

Change-type: bug-fix. Verified over the real MCP-over-HTTP transport on an
isolated **:3199 / nanobrain_test** server (dev DB / :3100 never touched).

## Setup (isolated)

Built the binary, seeded a registered ancestor workspace
(`path=/tmp/nb-res-565`, 64-hex hash) into **nanobrain_test**, started a
standalone server on :3199 (`NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1`,
`NANO_BRAIN_DATABASE_URL=…/nanobrain_test`).

## MCP streamable-HTTP handshake + tool calls

```
HTTP/1.1 200 OK   initialize            (Mcp-Session-Id issued)
HTTP/1.1 200 OK   tools/call memory_workspaces_resolve  path="/tmp/nb-res-565/backend"
HTTP/1.1 200 OK   tools/call memory_workspaces_resolve  path="/tmp/nowhere-xyz/x"
```

Decoded results:

```
CHILD     "/tmp/nb-res-565/backend"
  → registered: false | covered_by.root_path: "/tmp/nb-res-565" | covered_by.hash: 565000000000…
UNRELATED "/tmp/nowhere-xyz/x"
  → registered: false | covered_by: (absent)
```

**Result:** a path under a registered ancestor now returns `covered_by` pointing
at that ancestor end-to-end over MCP HTTP, so the agent queries the ancestor
instead of registering a redundant overlapping workspace. An unrelated path
correctly omits `covered_by`. **Before this fix** the child path returned only
`registered:false` with no ancestor pointer.

## Isolation / cleanup

Server on :3199 / nanobrain_test only; killed by captured PID (no broad kill).
Seeded workspace deleted, temp binary removed.
