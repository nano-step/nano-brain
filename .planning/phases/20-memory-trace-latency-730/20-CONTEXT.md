# Phase 20: MCP trace lookup latency (#730)

## Goal

Lower `memory_trace` cost for repeated present/missing bare-call targets
without changing the returned graph. Do not touch existing indexed
workspaces or the development database.

## Decisions

- Reuse complete symbol match lists within a single MCP tool invocation,
  including misses; never cache SQL errors or caller-specific narrowing.
- Add a partial index for workspace-and-title symbol lookup after
  demonstrating it on an isolated test database.
- Defer frontier batching and bare-graph SQL rewrite until separately
  measured; keep REST semantics unchanged.
- Verify synthetic MCP behavior and SQL plans on `nanobrain_test`, run
  build/race/integration suites, and independently review before shipping.

The OpenSpec change `fix-730-trace-latency` defines acceptance scenarios.
Private observations remain in ignored local research; public evidence
uses only synthetic workspace identifiers.
