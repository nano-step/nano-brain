Tracking: #489

## Why

The Rails capability benchmark currently scores **0.144 overall**, with `trace` and `impact` at **0.000**. That score is useful because it exposes real product gaps: Rails support can resolve some HTTP route flows and basic symbols, but it cannot reliably answer support/debug questions like "what calls this worker?", "what breaks if I change this method?", or "where are these status transitions defined?"

The main failure mode is not that the dataset should be loosened. The benchmark uses realistic bare Rails names (`BillingWorker#perform`, `Story#create_print_orders`, `DropboxUploadManager`) while graph storage usually contains file-qualified nodes (`app/...rb::Class#method`) or bare callee names. `memory_flow` has its own reconciliation index, but `memory_trace`, `memory_impact`, and related HTTP handlers do not consistently use the same reconciliation model.

## What Changes

- Add shared Rails/Ruby node reconciliation for trace, impact, and graph traversal so bare `Class`, `Class#method`, and method names can resolve to file-qualified graph nodes.
- Extend impact BFS to use symbol-aware target matching instead of exact-only `target_node` matching.
- Extend trace BFS to use symbol-aware source matching and continue from bare callee names into matching file-qualified method definitions.
- Allow flow-style traversal to start from non-HTTP Rails entries such as jobs/workers/services when no HTTP entry exists.
- Extract Ruby constants such as `STATUS_ORDER_SUBMITTED` as symbols and ensure concern files can surface in symbol/search results.
- Keep the benchmark privacy-safe: committed files use placeholders only; real workspace hashes, paths, names, and live result artifacts remain runtime-only.

## Impact

- Expected Rails capability score target for this change: **overall >= 0.35**, with non-zero `trace` and `impact` categories.
- Improves real support/debug usage, not just benchmark output.
- Preserves existing Go/TypeScript behavior by sharing reconciliation behind graph traversal helpers and adding regression tests.
- HTTP and MCP graph tools must remain behaviorally consistent.

## Non-Goals

- Do not weaken `dataset.json` expectations just to raise the score.
- Do not commit real benchmark result JSON from private Rails workspaces.
- Do not implement full Ruby dynamic dispatch, `method_missing`, `send`, or complete Rails autoload/Zeitwerk semantics in this change.
- Do not introduce a new database schema unless symbol-aware SQL/query helpers are insufficient.
