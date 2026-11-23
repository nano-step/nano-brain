# #730 MCP HTTP smoke (isolated test database)

Run from this branch with `NANO_BRAIN_TEST_DATABASE_URL` pointing only to
`nanobrain_test`:

```sh
go test -race -tags=integration -run '^TestTraceLatency730Evidence$' \
  -count=1 -v ./internal/mcp
```

The test creates a disposable per-test schema, a synthetic 40-branch call
graph and 14,000 unrelated symbol docs. It sends a JSON-RPC `tools/call`
for `memory_trace` via the actual Streamable HTTP MCP handler on an
`httptest` listener. It checks the HTTP status and response count rather
than assuming an in-memory SDK call proves the HTTP transport.

HTTP/1.1 200 OK; content-type=text/event-stream; memory_trace count=42 (synthetic workspace)

`TestTraceLatency730Evidence` passed (2.27s; `-race`). The listener uses
an ephemeral local port; the test checks it is using `nanobrain_test`
before fixture creation and does not access the development server.
