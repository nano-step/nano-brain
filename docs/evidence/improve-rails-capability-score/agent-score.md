# Rails Capability Agent-Oriented Score — Issue #489

Date: 2026-06-24

## Command

Runtime command used a private Rails workspace supplied through `NANO_BRAIN_WORKSPACE`; the workspace identifier is intentionally omitted from committed evidence.

```bash
NANO_BRAIN_URL=<runtime-server> NANO_BRAIN_WORKSPACE=<runtime-workspace> \
  go test -v -tags=capbench -run TestCapabilityBenchmark ./benchmarks/rails/capability
```

## Score-only Result

Primary recall is agent-oriented recall after deterministic retrieval augmentation. Fixed recall remains printed in the local run as a diagnostic layer for raw tool behavior.

| Category | Agent-oriented recall |
| --- | ---: |
| flow | 0.933 |
| impact | 0.500 |
| multi-tool | 1.000 |
| search-qa | 1.000 |
| state-transition | 0.667 |
| support-root-cause | 0.813 |
| symbol-lookup | 0.750 |
| trace | 0.667 |

Overall recall: **0.795**

## Interpretation

- The fixed diagnostic layer still shows raw graph gaps.
- The agent-oriented layer better reflects how coding agents use nano-brain: broad query first, then symbol discovery from identifiers.
- Runtime output files remain uncommitted.

## Privacy Notes

- No real workspace name, hash, or filesystem path is recorded here.
- Raw `results_current.json` is runtime output and must remain uncommitted.
- The benchmark dataset uses the generic workspace placeholder `rails-app`.
