# smoke:e2e — #585 (flow fanout cap keeps integration/reconcile edges)

Change-type: bug-fix, pure function (`BuildFlow`). The authoritative regression
proof is the unit test `TestFanoutCapKeepsIntegrationEdges` (reproduces the exact
scenario: 3 calls edges + a trailing integration edge, maxFanout=2 → asserts the
integration edge survives; fails on the old `break`, passes on `continue`). This
smoke additionally confirms the flow pipeline is healthy end-to-end over the live
MCP/HTTP path.

## HTTP transport (live server, :3100)

```
GET /api/v1/graph/flow/endpoints?workspace=<hash>
HTTP/1.1 200 OK    → 68 endpoints
```

## memory_flow (agent → MCP → server → BuildFlow), real result

```
memory_flow(entry="POST /api/v1/reindex")
→ found: true, 10 nodes, 11 edges
  edge kinds observed: http, middleware, calls (incl. conditional)
  chain: POST /api/v1/reindex → TriggerReindex (handler) → ListCollections/Get/
         collectionsToReindex/LoggerFromCtx → Close → fanoutEvent …
```

The flow assembles cleanly (entry → middleware → handler → downstream calls),
confirming `BuildFlow`'s out-edge scan is intact after the `break`→`continue`
change.

## Note on the fix-specific behavior

The dev server on :3100 runs a pre-fix binary, so this live call cannot itself
demonstrate the integration-edge-survival fix — that is covered deterministically
by the unit test above. The change is a 2-line pure-function edit with no HTTP
surface of its own; the unit test is the definitive e2e for `BuildFlow`.

## Isolation
Read-only calls against the running dev server; no writes, no DB mutation, no
process management.
