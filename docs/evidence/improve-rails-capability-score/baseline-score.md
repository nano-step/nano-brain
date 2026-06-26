# Rails Capability Baseline — Issue #489

Date: 2026-06-24

## Command

Runtime command used a private Rails workspace supplied through `NANO_BRAIN_WORKSPACE`; the workspace identifier is intentionally omitted from committed evidence.

```bash
NANO_BRAIN_URL=<runtime-server> NANO_BRAIN_WORKSPACE=<runtime-workspace> \
  go test -v -tags=capbench ./benchmarks/rails/capability
```

## Score-only Result

| Category | Recall |
| --- | ---: |
| flow | 0.200 |
| impact | 0.000 |
| multi-tool | 0.167 |
| search-qa | 0.143 |
| state-transition | 0.111 |
| support-root-cause | 0.134 |
| symbol-lookup | 0.350 |
| trace | 0.000 |

Overall recall: **0.144**

## Privacy Notes

- No real workspace name, hash, or filesystem path is recorded here.
- Raw `results_current.json` is runtime output and must remain uncommitted.
- The benchmark dataset uses the generic workspace placeholder `rails-app`.
