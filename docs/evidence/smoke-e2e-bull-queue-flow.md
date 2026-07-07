# smoke:e2e — Issue #581 (#546 E2: Bull queue producer/consumer flow linking)

Change-type: user-feature. Verified over the real MCP-over-HTTP transport on an
isolated **:3199 / nanobrain_test** server (dev DB / :3100 never touched).

## Setup (isolated)

Seeded into nanobrain_test: a workspace (64-hex hash), an HTTP route
`POST /schedule → scheduleEmail`, a producer edge
`svc.js::scheduleEmail → produce:emailJob` (`metadata.kind=queue_publish`,
`metadata.topic=emailJob`, as `queue.add("emailJob", ...)` now emits), and a
consumer entry `CONSUME emailJob → sendEmailWorker`
(`metadata.kind=queue_consumer`, as `queue.process("emailJob", ...)` now
emits). Started a flow-enabled server on :3199
(`NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1`, `NANO_BRAIN_FLOW_ENABLED=true`,
`NANO_BRAIN_DATABASE_URL=…/nanobrain_test`).

## MCP streamable-HTTP handshake + tool call

```
HTTP/1.1 200 OK   initialize                (Mcp-Session-Id issued)
202                notifications/initialized
HTTP/1.1 200 OK   tools/call memory_flow  entry="POST /schedule"   (NO stitch_workspaces arg)
```

Decoded response:

```json
{
  "found": true,
  "nodes": [
    {"id": "POST /schedule", "role": "entry"},
    {"id": "scheduleEmail", "role": "handler"},
    {"id": "CONSUME emailJob", "role": "integration"}
  ],
  "edges": [
    {"from": "POST /schedule", "to": "scheduleEmail", "kind": "http"},
    {"from": "scheduleEmail", "to": "CONSUME emailJob", "kind": "cross_service"}
  ]
}
```

**Result:** `memory_flow` on the route now crosses the Bull queue boundary —
the producer `scheduleEmail` is linked to the `CONSUME emailJob` consumer
entry by job-name, **without** the caller passing `stitch_workspaces`. This
confirms the #578 stitching plumbing (built for Redis pub/sub) generalizes to
Bull with zero changes to `memory_flow`/`flow.Stitch` — only the extractor
needed to emit the same edge shape.

## Isolation / cleanup

Server on :3199 / nanobrain_test only; killed by captured PID (no broad kill).
Seeded workspace/edges deleted, temp binary removed.
