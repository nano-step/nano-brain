# Issue 493 — TypeScript Capability Score Evidence

Benchmark profile: `benchmarks/typescript/capability/`

Runtime workspace identity was provided through `NANO_BRAIN_WORKSPACE` and is intentionally omitted from this evidence file.

## Frozen baseline score

| Metric | Score |
|---|---:|
| Overall | 0.850 |
| multi-tool | 1.000 |
| search-qa | 0.760 |
| symbol-lookup | 1.000 |

## Latest validation run

| Metric | Score |
|---|---:|
| Overall | 0.885 |
| multi-tool | 1.000 |
| search-qa | 0.817 |
| symbol-lookup | 1.000 |

## Task summary

| Task | Fixed | Agent | Final |
|---|---:|---:|---:|
| `ts-cs2-pricing` | 1.000 | 1.000 | 1.000 |
| `ts-inventory-read` | 0.833 | 1.000 | 1.000 |
| `ts-trade-status` | 0.714 | 0.714 | 0.714 |
| `ts-item-models` | 0.750 | 0.750 | 0.750 |
| `ts-steam-id` | 1.000 | 1.000 | 1.000 |
| `ts-trading-workflow` | 0.333 | 0.333 | 0.333 |
| `ts-price-to-inventory-multi-tool` | 0.333 | 1.000 | 1.000 |
| `ts-trade-state-multi-tool` | 0.800 | 1.000 | 1.000 |

## Validation

```text
go test -c -tags=capbench ./benchmarks/typescript/capability
PASS

go test -v -tags=capbench -run TestCapabilityBenchmark ./benchmarks/typescript/capability
PASS — baseline 0.850, latest 0.885

go test -race -short ./...
PASS
```

Privacy check: no real workspace names, workspace hashes, or private absolute paths are committed in `benchmarks/typescript/capability/`.
