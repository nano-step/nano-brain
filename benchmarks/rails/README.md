# Rails Extraction Benchmarks

End-to-end benchmarks measuring nano-brain's Ruby/Rails extraction quality
and performance: route parsing, control-flow graphs (CFGs), and flow builder
end-to-end.

## Benchmarks

| Benchmark | What it measures | Score |
|---|---|---|
| `bench_route_extraction.sh` | Accuracy of Rails route → controller handler mapping | found/expected |
| `bench_cfg_completeness.sh` | Fraction of controller actions with a valid CFG | cgfs_found/actions_expected |
| `bench_flow_e2e.sh` | Whether a full flow resolves for a known Rails entry | pass/fail |

## Prerequisites

- Go 1.23+
- PostgreSQL 17 with pgvector running on `host.docker.internal:5432`
- `nanobrain_test` database accessible (created automatically)

## Quick Start

```bash
# Run all benchmarks in sequence
cd benchmarks/rails
./setup.sh                        # starts server, registers fixtures
./bench_route_extraction.sh       # 1. route accuracy
./bench_cfg_completeness.sh       # 2. CFG completeness
./bench_flow_e2e.sh               # 3. flow E2E
./teardown.sh                     # cleanup
```

## Running Individual Benchmarks

Each benchmark calls `setup.sh` automatically if the server is not running.
To run a single benchmark against an already-running server:

```bash
NANO_BRAIN_URL=http://localhost:3199 ./bench_route_extraction.sh
```

## What Gets Indexed

The benchmark uses a synthetic Rails project under `fixtures/` that includes:

- **config/routes.rb** — `resources`, `namespace`, `scope`, `root`, `devise_for`,
  direct routes, collection/member blocks
- **app/controllers/** — 5 controllers with 15+ actions
- **app/models/** — 4 ActiveRecord models
- **app/services/** — 3 service objects

See `fixtures/` for the complete project structure.

## Expected Output

Each benchmark prints a JSON scorecard to stdout and writes detailed results
to `results/<benchmark_name>_<timestamp>.json`.

## Test Isolation

All benchmarks run against a **dedicated server on port 3199** backed by the
`nanobrain_test` database — never the dev server on port 3100 / `nanobrain_dev`.
The `setup.sh` script creates a fresh database, runs migrations, and starts an
isolated server.

## Adding a Benchmark

1. Create `benchmarks/rails/bench_<name>.sh`
2. Source `helpers.sh` for shared API functions
3. Use `setup_server` and `ensure_fixtures_registered` from `setup.sh`
4. Print results with `print_scorecard` from `helpers.sh`
