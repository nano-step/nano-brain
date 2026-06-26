# Rails Capability Benchmark

Static end-to-end benchmark that measures how well the live nano-brain server helps understand a **Rails photo-print pipeline** codebase. Each task is a hand-curated developer/support question with ground-truth symbols/files that a correct answer must surface.

This benchmark focuses on print/upload/status/order pipeline questions — the kind a developer or support engineer would actually ask about a Rails app.

## Prerequisites

- nano-brain server running and indexed on a real Rails workspace
- Go 1.23+

## Running

```bash
# From the repo root — server must be running, workspace must be registered
go test -v -tags=capbench -run TestCapabilityBenchmark ./benchmarks/rails/capability/

# Point at a non-default server
NANO_BRAIN_URL=http://localhost:3100 go test -v -tags=capbench -run TestCapabilityBenchmark ./benchmarks/rails/capability/

# Use a specific workspace hash (required — must match a registered Rails workspace)
NANO_BRAIN_WORKSPACE=<your-hash> go test -v -tags=capbench -run TestCapabilityBenchmark ./benchmarks/rails/capability/
```

The test skips automatically if the server is unreachable.

## Freezing a baseline

Run once after you are happy with current recall to lock in scores as the regression floor:

```bash
CAPBENCH_FREEZE=1 go test -v -tags=capbench -run TestCapabilityBenchmark ./benchmarks/rails/capability/
```

This writes `benchmarks/rails/capability/baseline_v1.json`. Commit it. Subsequent runs compare against it and fail if overall recall drops by more than 0.001.

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
| `flow` | Call-chain tracing from HTTP entry points / job classes |
| `impact` | Blast-radius analysis for changing a method or class |
| `trace` | Downstream call tracing from a specific function |
| `symbol-lookup` | Locating named symbols/files |
| `search-qa` | Free-text semantic questions |
| `support-root-cause` | Multi-tool support debugging workflows |
| `state-transition` | Status machine lifecycle questions |
| `multi-tool` | Tasks that combine multiple tools |

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

The dataset (`dataset.json`) contains 15 tasks across 8 categories covering the print pipeline domain:

- **Flow tasks** — story sync, Dropbox uploads, print order submission
- **Support root-cause tasks** — diagnosing why photos didn't print, stories stuck in status
- **Impact tasks** — blast radius of changing `Story#create_print_orders`, `DropboxUploadManager`
- **Trace tasks** — downstream calls from `BillingWorker`, `StoryPrintPerformer`
- **Symbol-lookup tasks** — locating `PrintOrder`, `OrderEventNotifier`
- **Search-QA tasks** — status transition queries
- **State-transition tasks** — full print order lifecycle
- **Multi-tool tasks** — compound queries combining flow+trace, symbols+impact

This benchmark is intended for a **real Rails workspace** with indexed controllers, models, workers, services, and engine code. Synthetic fixtures may not produce meaningful scores.

## Privacy

No real workspace names, hashes, or filesystem paths appear in committed files. The dataset uses a generic `rails-app` placeholder; the actual workspace hash is supplied at runtime via `NANO_BRAIN_WORKSPACE`.
