# smoke:e2e — Issue #577 (#546: Redis pub/sub flow linking)

Change-type: user-feature. Verified over the real MCP-over-HTTP transport on an
isolated **:3199 / nanobrain_test** server (dev DB / :3100 never touched).

## Setup (isolated)

Seeded into nanobrain_test: a workspace (64-hex hash), an HTTP route
`POST /trade → createTrade`, a publish edge
`svc.js::createTrade → publish:trade.created` (`metadata.topic=trade.created`),
and a consumer entry `CONSUME trade.created → TradeCreatedHandler`
(`metadata.kind=queue_consumer`, as `redis.subscribe` now emits). Started a
flow-enabled server on :3199 (`NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1`,
`NANO_BRAIN_FLOW_ENABLED=true`, `NANO_BRAIN_DATABASE_URL=…/nanobrain_test`).

## MCP streamable-HTTP handshake + tool call

```
HTTP/1.1 200 OK   initialize            (Mcp-Session-Id issued)
HTTP/1.1 200 OK   tools/call memory_flow  entry="POST /trade"   (NO stitch_workspaces arg)
```

Decoded:

```
nodes:              ["POST /trade", "createTrade", "CONSUME trade.created"]
cross_service edge: svc.js::createTrade -> CONSUME trade.created
```

**Result:** `memory_flow` on the route now crosses the message bus — the
publisher `createTrade` is linked to the `CONSUME trade.created` consumer entry
by topic, **without** the caller passing `stitch_workspaces`. **Before this fix**
the flow stopped at `createTrade` (no consumer link), and `redis.subscribe` was
extracted as an unmatchable `subscribe:<topic>` leaf.

(An initial run also showed a `CONSUME → CONSUME` self-edge — the consumer entry
was wrongly treated as a publisher. That was fixed in the same PR: `filterPublishEdges`
now excludes consumer entries, stitching is scoped to flow-reachable publishers,
and both edge endpoints are registered as nodes. The integration test asserts no
self-edge and no dangling endpoint.)

## Isolation / cleanup

Server on :3199 / nanobrain_test only; killed by captured PID (no broad kill).
Seeded workspace/edges deleted, temp binary removed.
