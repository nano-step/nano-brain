# Tasks (#730)

- [x] Test repeated existing and missing bare targets; prove lookup count
      regression before code edits.
- [x] Add request-local memoization without changing trace output.
- [x] Measure symbol/graph SQL plans on isolated `nanobrain_test`; add only
      a proven index/query fix and regenerate sqlc when required.
- [x] Run focused unit/integration, quick race suite and isolated MCP
      before/after timings with the index held constant; separately
      compare SQL plans with and without the index.
- [x] Get two independent reviewer reports; adjudicate and fix blockers.

Shipping remains gated on the final diff review and CI.
