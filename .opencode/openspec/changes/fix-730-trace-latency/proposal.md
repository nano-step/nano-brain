# Reduce memory_trace latency (#730)

## Why

Deep MCP traces perform one edge query per frontier node plus one symbol
lookup per bare call edge. Repeated unresolved names trigger repeated
workspace-wide document scans. An isolated Go trace at depth 4 took 12.6s;
in-depth evidence and limitations are in the local #730 brief.

## What

- Reuse symbol resolution results within one MCP trace request, including
  misses, without changing cross-workspace or ambiguous-node semantics.
- Evaluate and apply an index-friendly symbol lookup, and optimize the
  bare graph lookup only if an isolated SQL plan and live-path probe show
  a measurable benefit.
- Add a failing-before-fix query-count test and read-only test-DB evidence.

## Non-goals

No REST behavior change, response-shape change, implicit truncation, data
reset or reindex, private workspace publication, or dev database test.
