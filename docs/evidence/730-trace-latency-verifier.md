# #730 independent test-DB measurement (2026-11-23)

## Reproduce

From this worktree, with a reachable `nanobrain_test` PostgreSQL database:

```sh
NANO_BRAIN_TEST_DATABASE_URL='<nanobrain_test DSN>' \
  go test -race -tags=integration -run '^TestTraceLatency730Evidence$' -count=1 -v ./internal/mcp
git diff --check
```

The test checks the DSN **before** `SetupTestDB`, confirms `current_database()`, creates an isolated schema, runs migrations including #32, and cleans up. It uses only synthetic names/paths. Fixture: 14,000 unrelated symbol documents, one `present` symbol, 40 distinct branch nodes each pointing to `present` and `missing`, and one qualified entry node pointing to all 40 branches (120 graph edges). It runs the actual `memory_trace` MCP handler via the SDK in-memory client/server transport with `max_depth=2`, `include_external=true`, then checks the count is 42. Five warmups precede 20 measured calls. This includes transport, handler and live database time, but **not** HTTP/network overhead.

## Observed output (one `-race` run, not an SLA)

```
indexed present: Index Scan using idx_documents_workspace_symbol_title; shared hit=4; Execution Time: 0.048 ms
indexed missing: Index Scan using idx_documents_workspace_symbol_title; shared hit=3; Execution Time: 0.038 ms
unindexed present: Seq Scan on documents; Rows Removed by Filter: 14000; shared hit=506; Execution Time: 1.770 ms
unindexed missing: Seq Scan on documents; Rows Removed by Filter: 14001; shared hit=506; Execution Time: 1.515 ms
bare graph fallback: Seq Scan on graph_edges (120 rows), split_part(source_node, '::', 2) filter; shared hit=564; Execution Time: 0.333 ms
modified MCP in-memory transport (with -race), 5 warmup + 20 measured: p50=99.002833ms p95=157.268292ms min=47.67925ms max=211.947958ms
--- PASS: TestTraceLatency730Evidence (4.83s)
PASS
ok  	github.com/nano-brain/nano-brain/internal/mcp	6.707s
```

`EXPLAIN (ANALYZE, BUFFERS)` used the exact `ResolveSymbolByName` SQL on the **same connection and fixture** with the migrated index present, then with that index transactionally dropped; rollback restored it. Planner settings were not changed. These are single execution timings and buffer counts, not distributions or end-to-end speedup estimates. Bare graph fallback was separately explained with the `GetOutgoingEdgesBySymbol` SQL; its `split_part` work is **not** addressed by the documents title index. Its small fixture graph cannot predict behavior with large graph tables.

## Same-fixture MCP comparison (no race detector)

Using `go test -overlay` to replace **only** `internal/mcp/tools.go` with
`origin/master` for the baseline, the test fixture, migration 00032,
database, index and SQL were identical in both versions. Each invocation
created an isolated schema with the same 14,000/120-row fixture shape;
it took five warmups followed by 20 measured calls. Two runs per version:

```sh
git show origin/master:internal/mcp/tools.go > /tmp/nb-730-baseline-tools.go
python3 - <<'PY'
import json, pathlib
path = str(pathlib.Path('internal/mcp/tools.go').resolve())
pathlib.Path('/tmp/nb-730-overlay.json').write_text(json.dumps(
    {'Replace': {path: '/tmp/nb-730-baseline-tools.go'}}))
PY
NANO_BRAIN_TEST_DATABASE_URL='<nanobrain_test DSN>' \
  go test -overlay=/tmp/nb-730-overlay.json -tags=integration \
  -run '^TestTraceLatency730Evidence$' -count=2 -v ./internal/mcp
NANO_BRAIN_TEST_DATABASE_URL='<nanobrain_test DSN>' \
  go test -tags=integration -run '^TestTraceLatency730Evidence$' \
  -count=2 -v ./internal/mcp
```

| Version | MCP p50 (ms) | MCP p95 (ms) |
|---|---:|---:|
| Baseline trace code, index 00032 present | 83.52, 75.67 | 99.62, 92.20 |
| Request-local cache, same index | 31.54, 34.44 | 39.16, 41.25 |

This isolates the effect of the cache (about 2.2–2.6x lower p50 for this
fixture), **not** the combined effect of cache plus index. The SQL index
A/B above independently demonstrates its lookup improvement. The earlier
99ms p50 was a separate post-change `-race` run and is not comparable to
these non-race timings. These results are not a production SLA or proof
about deeper graphs; in-memory MCP excludes HTTP/network. No dev database
or server port 3100 was used.

Deployment caveat: an interrupted `CREATE INDEX CONCURRENTLY` can leave an
INVALID index. On migration failure, check `pg_index.indisvalid` for
`idx_documents_workspace_symbol_title` before retrying; `IF NOT EXISTS`
alone does not prove the index is usable. Recovery requires explicitly
removing the invalid index before re-running the migration.
