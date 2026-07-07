# smoke:e2e — Issue #563 (#542 F1: memory_flow full mounted URL)

Change-type: bug-fix. Verified over the real MCP-over-HTTP transport on an
isolated **:3199 / nanobrain_test** server (dev DB / :3100 never touched).

## Setup (isolated)

Built the binary, seeded a workspace (64-hex hash) + a router-local HTTP edge
`POST /payment-intent → createPaymentIntent` into **nanobrain_test**, started a
standalone flow-enabled server on :3199 (`NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1`,
`NANO_BRAIN_FLOW_ENABLED=true`, `NANO_BRAIN_DATABASE_URL=…/nanobrain_test`).
Server log confirmed `database pool connected … /nanobrain_test`.

## MCP streamable-HTTP handshake + tool calls

```
HTTP/1.1 200 OK   initialize            (Mcp-Session-Id issued)
HTTP/1.1 200 OK   tools/call memory_flow  entry="POST /api/payments/payment-intent"
HTTP/1.1 200 OK   tools/call memory_flow  entry="POST /payment-intent"
```

Decoded results:

```
FULL URL     "POST /api/payments/payment-intent"
  → found: true | entry: "POST /payment-intent" | resolved_via: "suffix-match"
    | requested_entry: "POST /api/payments/payment-intent"
ROUTER-LOCAL "POST /payment-intent"
  → found: true | entry: "POST /payment-intent" | (exact, no resolved_via)
```

**Result:** the full mounted URL an agent actually has now resolves to the
stored router-local key end-to-end over MCP HTTP (`found:true`), reporting the
resolved key + how it matched. Router-local entries still match exactly (no
regression). **Before this fix** the full URL returned `found:false, "entry not
found among flow edges"`.

## Isolation / cleanup

Server on :3199 / nanobrain_test only; killed by captured PID (no broad kill).
Seeded workspace + edge deleted, temp binary + fixture dir removed.
