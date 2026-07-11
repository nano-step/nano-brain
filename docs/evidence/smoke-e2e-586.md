# smoke:e2e — #586 (top-level pub/sub subscribers captured in integration extractors)

Change-type: bug-fix. The change is in the graph integration extractors
(`internal/graph/{js,python,integration}_integration_extractor.go`) — AST
extraction logic with no HTTP surface of its own. The authoritative regression
proof is the three new unit tests (one per language) asserting a top-level
subscriber/publisher now emits its coupling edge attributed to a synthetic
`<file>::<module>` symbol, plus the pre-existing `*_TopLevelCall_NoEdge` tests
confirming top-level HTTP calls stay dropped. This smoke additionally confirms
the graph/flow HTTP path is healthy end-to-end.

## HTTP transport (live dev server, :3100, read-only)

```
GET /api/status
HTTP/1.1 200 OK

GET /api/v1/graph/flow/endpoints?workspace=<hash>
HTTP/1.1 200 OK    → 68 endpoints

POST /api/v1/search  {"query":"queue_consumer","limit":3}
HTTP/1.1 200 OK    → 10 hits (existing queue_consumer edges surface via search)
```

The graph query + search pipeline assembles cleanly over the live MCP/HTTP path,
confirming extractor output flows through storage → search/graph unchanged.

## Note on fix-specific behavior

The dev server on :3100 runs a pre-fix binary, so this live call cannot itself
demonstrate the new top-level-subscriber edges — that is covered deterministically
by the unit tests, all passing under `go test -race -short ./internal/graph/`:

- `TestJSIntegrationExtractor_TopLevelSubscribe_Module` — the issue's exact
  `sub.on('connect', () => { sub.subscribe('channelX', ...) })` bootstrap now
  yields `CONSUME channelX → consumer.ts::<module>`.
- `TestPythonIntegrationExtractor_TopLevelSubscribe_Module`.
- `TestIntegrationExtractor_TopLevelPublish_Module`.

The change has no HTTP surface of its own; the unit tests are the definitive e2e
for the extractors.

## Isolation

Read-only GET / POST-search calls against the running dev server; no writes, no
reindex, no DB mutation, no process management. (Per testing-isolation, any
write/index smoke would target nanobrain_test/:3199; none was needed here.)
