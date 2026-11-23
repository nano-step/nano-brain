# Design: request-scoped trace lookups

The MCP BFS retains its current queue, `seen` and proximity disambiguation.
Introduce a per-invocation lookup cache keyed by bare target name for
`ResolveSymbolByName` results. Cache both non-empty and empty results, but
never cache a SQL error. Preserve full match lists: nearest-symbol selection
depends on each edge's `source_file`, so a previously narrowed list cannot
be reused for another caller.

`documents` has a workspace index but no composite title/symbol-source
index; measure the partial `(workspace_hash, title)` index on isolated
`nanobrain_test` and add an additive migration if it helps. Existing graph
view's computed `split_part` for bare lookups may defeat `graph_nodes`
indexes; assess separately using `EXPLAIN` and preserve qualified JS/TS
matching constraints. Do not assume REST latency proxies MCP latency.

Verify relative/absolute path, max-depth, cycles, missing/builtin targets,
ambiguous same-name matches and `include_external`. The ideal long-term
frontier-batching approach is explicitly deferred until cache+index
measurements justify its complexity.
