# smoke:e2e — Issue #569 (#542 F6: memory_graph reverse route edge)

Change-type: bug-fix. Verified over the real MCP-over-HTTP transport on an
isolated **:3199 / nanobrain_test** server (dev DB / :3100 never touched).

## Setup (isolated)

Built the binary, seeded into **nanobrain_test**: a workspace (64-hex hash), a
route→handler `http` edge (`POST /payment-intent → createPaymentIntent`, bare
target) and the `contains` edge (`controllers/pay.js → controllers/pay.js::createPaymentIntent`,
qualified target). Started a standalone server on :3199
(`NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1`, `NANO_BRAIN_DATABASE_URL=…/nanobrain_test`).

## MCP streamable-HTTP handshake + tool call

```
HTTP/1.1 200 OK   initialize            (Mcp-Session-Id issued)
HTTP/1.1 200 OK   tools/call memory_graph  node="controllers/pay.js::createPaymentIntent" direction="in"
```

Decoded incoming edges:

```
[ {source:"controllers/pay.js", target:"controllers/pay.js::createPaymentIntent", edge_type:"contains"},
  {source:"POST /payment-intent", target:"createPaymentIntent",                    edge_type:"http"} ]
```

**Result:** `direction:"in"` on the qualified handler node now returns the
route→handler `http` edge (bare stored target matched against the qualified
query) alongside the `contains` edge — end-to-end over MCP HTTP. **Before this
fix** only the `contains` edge was returned; the `http` edge was dropped by the
qualified-vs-bare mismatch.

## Isolation / cleanup

Server on :3199 / nanobrain_test only; killed by captured PID (no broad kill).
Seeded workspace/edges deleted, temp binary removed.
