# Vue Capability Benchmark

Static end-to-end benchmark that measures how well the live nano-brain server helps understand a **Vue.js** codebase. Each task is a hand-curated developer question with ground-truth symbols/files that a correct answer must surface.

This benchmark focuses on components, composables, pages, stores, and utilities — the kind of questions a Vue developer would actually ask about a Vue app.

## Prerequisites

- nano-brain server running and indexed on a real Vue workspace
- Go 1.23+

## Running

```bash
# From the repo root — server must be running, workspace must be registered
go test -v -tags=capbench -run TestCapabilityBenchmark ./benchmarks/vue/capability/

# Point at a non-default server
NANO_BRAIN_URL=http://localhost:3100 go test -v -tags=capbench -run TestCapabilityBenchmark ./benchmarks/vue/capability/

# Use a specific workspace hash (required — must match a registered Vue workspace)
NANO_BRAIN_WORKSPACE=<your-hash> go test -v -tags=capbench -run TestCapabilityBenchmark ./benchmarks/vue/capability/
```

The test skips automatically if the server is unreachable.

## Freezing a baseline

Run once after you are happy with current recall to lock in scores as the regression floor:

```bash
CAPBENCH_FREEZE=1 go test -v -tags=capbench -run TestCapabilityBenchmark ./benchmarks/vue/capability/
```

This writes `benchmarks/vue/capability/baseline_v1.json`. Commit it. Subsequent runs compare against it and fail if overall recall drops by more than 0.001.

## Output files

| File | Purpose |
|---|---|
| `baseline_v1.json` | Frozen baseline scores. Created by `CAPBENCH_FREEZE=1`. Commit this. |
| `results_current.json` | Scores from the most recent run. Gitignore-safe (overwritten each run). |

## Metrics

- **fixed recall** — score from the task's declared fixed tool calls. This remains the diagnostic layer for raw tool behavior.
- **agent recall / recall** — score after deterministic agent-oriented retrieval augments fixed results with broad question search, input-query search, and symbol lookup from code identifiers. This is the primary score because nano-brain is designed for agents.
- **recall** (per task) — `matched / (expect_symbols + expect_files)`. A value of 1.0 means every expected item was found somewhere in the retrieved context.
- **by_category** — mean recall across all tasks in that category.
- **overall** — mean recall across all tasks.

### Categories

| Category | Description |
|---|---|
| `graph-out` | Outgoing dependencies from a component or file (what it imports) |
| `graph-in` | Incoming dependents (who imports this component or composable) |
| `trace` | Downstream call tracing from a specific function or lifecycle hook |
| `impact` | Blast-radius analysis for changing a component or composable |
| `symbol-lookup` | Locating named components, composables, or stores |
| `search-qa` | Free-text semantic questions about features |
| `multi-tool` | Tasks that combine multiple tools (flow + trace + impact, etc.) |

## Scoring rules

For each task, all listed fixed tools are called first and their responses unioned into two sets:

- **Names** — symbol names, function names, chain/external names
- **Paths** — source paths, file parts of node ids

An `expect_symbols` entry matches if it is a **case-insensitive substring** of any name in the names set.
An `expect_files` entry matches if it is a **case-insensitive substring** of any path in the paths set.

A failed tool call contributes no strings (the task just scores low). The harness never aborts the run on a per-tool error.

When `dataset.agent.enabled` is true, the runner then performs a deterministic agent-oriented retrieval pass using only the task question and input, never the expected answers. The current shared workflow is:

1. `query_question` — hybrid-search the natural-language question.
2. `query_input` — hybrid-search the task's explicit query input, if present.
3. `symbols_identifiers` — extract likely code identifiers from question/input and look them up with symbols.

The final score is computed over the union of fixed and agent-retrieved context; fixed recall is still printed for diagnosis.

## Regression policy

The test **fails** only when `overall < baseline.overall - 0.001`. Improvements are logged without failing.

## Dataset

The dataset (`dataset.json`) contains 18 tasks across 7 categories:

| Category | Tasks | What it tests |
|----------|-------|---------------|
| graph-out | 2 | Outgoing dependencies (imports, contains) from a composable |
| graph-in | 2 | Incoming dependents (who imports this composable) |
| trace | 4 | Downstream call chains from composable functions |
| impact | 2 | Blast-radius analysis for changing a composable |
| symbol-lookup | 3 | Locating named functions/files |
| search-qa | 3 | Semantic questions about features |
| multi-tool | 2 | Compound queries combining multiple tools |

The dataset uses `__PROJECT__` placeholders for file path prefixes. Before running, generate `dataset.json` with your project prefix using the generate script (see below).

## Generating the dataset

The committed `dataset.template.json` uses `__PROJECT__` placeholders instead of real path prefixes. Before running the benchmark, generate `dataset.json` for your specific project:

```bash
./generate-dataset.sh <your-project>
# Example: ./generate-dataset.sh tradeit
```

This replaces all `__PROJECT__` placeholders with your project name, producing a `dataset.json` with real paths that match your workspace.

**Important:** `dataset.json` is gitignored — never commit it. Only `dataset.template.json` and the generate script are committed.

## Privacy

No real workspace names, hashes, or absolute filesystem paths appear in committed files. The dataset uses a generic `vue-app` workspace placeholder and `app/` path prefix; the actual workspace hash is supplied at runtime via `NANO_BRAIN_WORKSPACE`.
