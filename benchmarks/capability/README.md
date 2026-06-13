# Capability Benchmark

Static end-to-end benchmark that measures how well the live nano-brain server helps understand the nano-brain codebase itself. Each task is a hand-curated developer question with ground-truth symbols/files that a correct answer must surface.

## Prerequisites

- nano-brain server running and indexed on the nano-brain repo
- Go 1.23+

## Running

```bash
# From the repo root — server must be running
go test -v -tags=capbench -run TestCapabilityBenchmark ./benchmarks/capability/

# Point at a non-default server
NANO_BRAIN_URL=http://localhost:3100 go test -v -tags=capbench -run TestCapabilityBenchmark ./benchmarks/capability/

# Use a specific workspace hash
NANO_BRAIN_WORKSPACE=<your-hash> go test -v -tags=capbench -run TestCapabilityBenchmark ./benchmarks/capability/
```

The test skips automatically if the server is unreachable.

## Freezing a baseline

Run once after you are happy with current recall to lock in scores as the regression floor:

```bash
CAPBENCH_FREEZE=1 go test -v -tags=capbench -run TestCapabilityBenchmark ./benchmarks/capability/
```

This writes `benchmarks/capability/baseline_v1.json`. Commit it. Subsequent runs compare against it and fail if overall recall drops by more than 0.001.

## Output files

| File | Purpose |
|---|---|
| `baseline_v1.json` | Frozen baseline scores. Created by `CAPBENCH_FREEZE=1`. Commit this. |
| `results_current.json` | Scores from the most recent run. Gitignore-safe (overwritten each run). |

## Metrics

- **recall** (per task) — `matched / (expect_symbols + expect_files)`. A value of 1.0 means every expected item was found somewhere in the tool responses.
- **by_category** — mean recall across all tasks in that category.
- **overall** — mean recall across all tasks.

### Categories

| Category | Description |
|---|---|
| `flow` | Call-chain tracing from HTTP entry points |
| `impact` | Blast-radius analysis for changing a function |
| `trace` | Downstream call tracing from a specific function |
| `symbol-lookup` | Locating named symbols/files |
| `search-qa` | Free-text semantic questions |
| `multi-tool` | Tasks that combine multiple tools |

## Scoring rules

For each task, all listed tools are called and their responses unioned into two sets:

- **Names** — symbol names, function names, chain/external names
- **Paths** — source paths, file parts of node ids

An `expect_symbols` entry matches if it is a **case-insensitive substring** of any name in the names set.
An `expect_files` entry matches if it is a **case-insensitive substring** of any path in the paths set.

A failed tool call contributes no strings (the task just scores low). The harness never aborts the run on a per-tool error.

## Regression policy

The test **fails** only when `overall < baseline.overall - 0.001`. Improvements are logged without failing.

## Isolated benchmark environment

The benchmark runs against a **dedicated server on port 3199 backed by the `nanobrain_test` database** — never the dev server (3100 / `nanobrain_dev`) — so benchmark indexing never touches dev data. The harness defaults to `http://localhost:3199`.

Spin it up with `./setup.sh` (from this directory), or manually:

```bash
# 1. Clean isolated DB
psql "postgres://nanobrain:nanobrain@localhost:5432/postgres" -c "DROP DATABASE IF EXISTS nanobrain_test WITH (FORCE);"
psql "postgres://nanobrain:nanobrain@localhost:5432/postgres" -c "CREATE DATABASE nanobrain_test OWNER nanobrain;"
psql "postgres://nanobrain:nanobrain@localhost:5432/nanobrain_test" -c "CREATE EXTENSION IF NOT EXISTS vector;"

# 2. Migrate + start the isolated flow-enabled server on 3199
DATABASE_URL="postgres://nanobrain:nanobrain@localhost:5432/nanobrain_test" ./nano-brain db:migrate
NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1 NANO_BRAIN_SERVER_PORT=3199 NANO_BRAIN_FLOW_ENABLED=true \
  DATABASE_URL="postgres://nanobrain:nanobrain@localhost:5432/nanobrain_test" ./nano-brain serve &

# 3. Index ONLY the nano-brain repo into the isolated DB, wait for the scan to reach routes.go
curl -s -X POST http://localhost:3199/api/v1/init -H 'Content-Type: application/json' \
  -d "{\"root_path\":\"$(git rev-parse --show-toplevel)\"}"
# poll until graph_edges has ~55 http edges (routes.go indexed)
```

`NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1` lets it run alongside the dev server; only the nano-brain workspace is registered, so the watcher indexes just this repo (fast, no noise).

## Baseline v1 — interpretation (captured 2026-06-14, isolated `nanobrain_test`, fully indexed + embedded)

Overall **0.813**. Per-category:

| Category | v1 | Reading |
|---|---|---|
| flow | **1.00** | Flow surfaces the true business functions (`HybridSearch`, `BM25SearchAll`, middleware) for each endpoint. |
| impact | **1.00** | Reverse-dependency lookup finds the correct callers. |
| multi-tool | **1.00** | Combining flow + impact (and symbols + impact) answers compound questions. |
| symbol-lookup | **1.00** | `symbols` locates BM25, chunker, and RRF correctly. |
| search-qa | **0.67** | Free-text search works once embeddings are complete (embedding-worker + harvest hit); `qa-recency-decay` misses — search doesn't rank `ApplyRecencyBoost` (`internal/search/recency.go`) for "recency decay ranking". A search-ranking gap. |
| trace | **0.00** | **Real capability gap.** `memory_trace` can't traverse multi-hop: `calls` edges store bare callee names while sources are `file::func`, so it dead-ends after one hop. The same symbol-reconciliation the flow builder uses would fix it. |

**Improvement targets:** (1) fix `memory_trace` multi-hop via reconciliation → lifts `trace` toward flow's score; (2) improve search ranking so `ApplyRecencyBoost`-style symbol queries surface their defining file. The flow feature is demonstrably useful *in combination* (flow + impact + multi-tool all 1.0) — the question that motivated this benchmark.

> The baseline assumes a **fully embedded** index — `setup.sh` waits for the embed queue to drain before you run. search-qa was 0 only while embeddings were still draining; with them complete it's 0.67.

## Editing the dataset

Edit `dataset.json` to add tasks or fix stale ground truth. Do NOT change expected values just to make a run pass — only update them when the correct answer genuinely changes (e.g. after a refactor renames a symbol). After any dataset change, re-freeze the baseline.
