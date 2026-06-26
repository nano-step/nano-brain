# TypeScript Capability Benchmark

Agent-oriented benchmark profile for a TypeScript/JavaScript CS2 item-trading workspace. The committed dataset uses privacy-safe placeholders; local runs provide the real workspace hash through `NANO_BRAIN_WORKSPACE`.

## Running

```bash
NANO_BRAIN_URL=http://host.docker.internal:3100 \
NANO_BRAIN_WORKSPACE=<runtime-workspace-hash> \
go test -v -tags=capbench -run TestCapabilityBenchmark ./benchmarks/typescript/capability/
```

The benchmark writes `results_current.json`, which is ignored by git.

## Scoring

Each task has a fixed tool layer plus deterministic agent-style retrieval:

- `fixed_recall` — direct tools listed on the task.
- `agent_recall` — fixed layer plus query/question/input and identifier-driven symbol lookups.
- `recall` — final score used for category and overall metrics.

The dataset intentionally checks project-domain concepts: CS2 item pricing, inventory reads, trade offer status/recheck flows, item/product models, SteamID conversion, and trading workflow documentation. Do not commit real workspace names, hashes, absolute paths, or private project names.
