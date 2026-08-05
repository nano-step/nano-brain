# Source-scoped re-extraction smoke procedure

The lifecycle proof uses `testutil.SetupTestDB(t)`, which defaults to the
isolated `nanobrain_test` database. The fixture contains `repo-a/consumer.ts`,
`repo-a/lib/api.ts`, and a colliding `repo-b/lib/api.ts`. The watcher registers
the temporary root with `WatchWithFilter`, scans it, and calls
`ReextractEdgesForWorkspace` twice. The test then edits the consumer, drives the
same dirty/contextual-re-extraction path used by watcher events, and renames the
exporter. Each update replaces only the consumer source batch, preserves the
collision fixture, and changes the target to `<unresolved>` when proof is gone.

Command:

```text
go test -tags=integration ./internal/watcher -run TestReextractEdgesForWorkspaceReplacesSourceEdges -count=1
```

Observed output on 2026-08-05: `ok github.com/nano-brain/nano-brain/internal/watcher 8.626s`.

Focused streamable HTTP and REST route controls passed on 2026-08-05:

```text
go test -race -count=1 -tags=integration ./internal/mcp -run TestStreamableHTTP_ConnectionDefaultWorkspace -v  PASS
go test -race -count=1 -tags=integration ./internal/server -run 'Test.*Route|Test.*Health|Test.*Graph' -v  PASS
```

The bounded live `:3199` server procedure remains the coordinator's final
smoke gate. Per repository policy, agents must not start nano-brain inside the
container. A probe on 2026-08-05 found no host-backed listener:
`curl http://127.0.0.1:3199/api/status` failed with connection refused.
Therefore the live REST/MCP portion is environment-blocked; no server was
started and port 3100 was not touched.
