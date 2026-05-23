---
epic: 7
title: "Benchmarking Suite"
status: ready
depends_on: ["Epic 4: Hybrid Search"]
created: 2026-05-23
---

# Epic 7: Benchmarking Suite — User Stories

**Epic summary:** Generate labeled benchmark datasets, measure search quality (P@5, R@10, MRR)
and latency, compare results across runs to detect regressions, and stress-test concurrent writes
for zero data loss. The suite is a first-class shipped tool — one of the v2.0 release gates
("search quality >= v1") cannot be verified without it.

**FRs covered:** FR-37, FR-38, FR-39, FR-40, FR-41

**ARs applied:** AR-1 (Go static binary), AR-8 (sqlc for storage queries), AR-16 (bench/ in
internal/ package tree), AR-18 (go test -race as CI gate)

**NFRs enforced:**
- NFR-3: Benchmarking suite is the quality validation instrument for P@5>=0.835, R@10>=0.970, MRR>=1.000
- NFR-1: Concurrency stress test validates goroutine-safe PostgreSQL writes under load

**Dependency note:** All stories assume Epic 4 is complete. The search pipeline (hybrid search,
BM25, vector) must be functional before any benchmark can produce meaningful quality metrics.
The `internal/bench/` package imports `internal/search/` and `internal/storage/`.

---

## Stories

#### Story 7.1: Dataset Generator (`bench generate`)

**Description:** Implement `nano-brain bench generate --scale=N` to produce a labeled benchmark
dataset from the current workspace. The command samples existing documents, derives ground-truth
query-answer pairs, and writes a JSON dataset file to the workspace data directory. Valid scales
are 100, 500, and 1000. The dataset is the prerequisite for every other bench subcommand.

**Covers:** FR-37

**Applies:** AR-1, AR-8, AR-16

**Complexity:** M

**Acceptance Criteria:**

- Given a workspace with at least N indexed documents, when `nano-brain bench generate --scale=N`
  is run, then it exits 0 and writes a valid JSON dataset file to the workspace data directory
  containing exactly N query-answer pairs.
- Given the dataset file is written, when it is inspected, then each entry contains a `query`
  string and at least one `relevant_doc_id` ground-truth identifier drawn from documents in that
  workspace.
- Given `--scale=500` is specified, when the workspace has fewer than 500 documents, then the
  command exits non-zero with a descriptive error message (e.g., "workspace has only 120 documents,
  --scale=500 requires at least 500").
- Given the dataset file is generated, when `bench generate` is re-run with the same scale and
  workspace, then it overwrites the existing file and exits 0 (idempotent).
- Given `--scale` is omitted, when `bench generate` is run, then it exits non-zero with a usage
  error that lists the valid scale values.
- Given `--json` flag is passed, when the command exits, then all output (progress and errors) is
  emitted as JSON to stdout, not plain text.

**Test expectations:**
- Unit: dataset builder produces valid JSON structure; sampling logic selects from workspace docs only.
- Integration (`//go:build bench`): insert 200 documents into a test workspace, run generate
  --scale=100, assert output file has 100 entries, each `relevant_doc_id` maps to a real document
  in that workspace.

---

#### Story 7.2: Quality and Latency Measurement (`bench run`)

**Description:** Implement `nano-brain bench run --scale=N [--save <path>]` to execute the full
hybrid search pipeline against the benchmark dataset and compute P@5, R@10, MRR, and latency
percentiles (insert p50/p95, query p50/p95). Results are printed to stdout and optionally saved
to a JSON file for use as a future baseline. This is the primary measurement command and the
direct instrument for release gate G3 (search quality >= v1).

**Covers:** FR-38, FR-40

**Applies:** AR-1, AR-8, AR-16

**Complexity:** L

**Acceptance Criteria:**

- Given a generated dataset file for scale N, when `nano-brain bench run --scale=N` is run, then
  it exits 0 and prints a results summary containing non-null values for P@5, R@10, MRR, insert
  p50, insert p95, query p50, and query p95.
- Given the command runs, when results are printed, then P@5 is computed as precision at rank 5
  (fraction of top-5 results that are relevant), R@10 as recall at rank 10 (fraction of all
  relevant docs found in top 10), and MRR as the mean reciprocal rank of the first relevant result.
- Given `--save <path>` is provided, when the command exits 0, then the specified path contains a
  valid JSON file with all computed metrics plus metadata (scale, timestamp, workspace hash,
  nano-brain version).
- Given the search pipeline meets v1 quality targets, when `bench run --scale=100` is executed on
  a workspace matching the v1 test conditions (Ollama local, comparable corpus), then P@5>=0.835,
  R@10>=0.970, MRR>=1.000.
- Given `--scale=N` references a dataset that does not exist, when `bench run` is called, then it
  exits non-zero with an error message directing the user to run `bench generate --scale=N` first.
- Given `--json` flag is passed, when the command completes, then all output is JSON to stdout,
  suitable for piping to `jq` or other tools.

**Test expectations:**
- Unit: metric calculation (P@5, R@10, MRR) verified against hand-computed fixtures with known
  relevant/non-relevant sets.
- Integration (`//go:build bench`): insert known documents, generate scale-10 dataset with
  deterministic relevance, run bench run, assert P@5=1.000 for a trivially correct retrieval set.
- Performance: bench run on scale-100 completes in under 60 seconds on standard dev hardware.

---

#### Story 7.3: Regression Detection (`bench compare`)

**Description:** Implement `nano-brain bench compare <new.json> <baseline.json>` to diff two saved
benchmark result files and flag regressions against defined thresholds. Exits non-zero when any
threshold is breached, making it usable as a CI gate. This is the command that validates release
gate G3 before shipping v2.0.

**Covers:** FR-39

**Applies:** AR-1, AR-16

**Complexity:** S

**Acceptance Criteria:**

- Given two result files where new metrics are within threshold of baseline, when `bench compare
  new.json baseline.json` is run, then it exits 0 and prints a pass summary showing deltas for
  P@5, R@10, MRR, and latency.
- Given a new result file where P@5 has dropped by more than 0.10 from baseline, when `bench
  compare` is run, then it exits non-zero and clearly reports "REGRESSION: P@5 dropped 0.12
  (threshold: 0.10)".
- Given a new result file where R@10 has dropped by more than 0.10 from baseline, when `bench
  compare` is run, then it exits non-zero with the R@10 regression message.
- Given a new result file where MRR has dropped by more than 0.05 from baseline, when `bench
  compare` is run, then it exits non-zero with the MRR regression message.
- Given a new result file where query p95 latency is more than 2x the baseline query p95, when
  `bench compare` is run, then it exits non-zero with the latency regression message.
- Given improvements (metrics better than baseline), when `bench compare` is run, then it exits 0
  and reports the improvements alongside the passing status.
- Given multiple thresholds are breached simultaneously, when `bench compare` is run, then all
  regressions are reported (not short-circuited after the first one).
- Given `--json` flag is passed, when the command exits, then output is a machine-readable JSON
  object with `passed: bool`, `regressions: [...]`, and `deltas: {...}` fields.

**Test expectations:**
- Unit: compare logic tested against fixture JSON pairs; threshold math verified for boundary
  values (exactly at threshold passes, one unit below fails).
- Integration: generate two result files where one is artificially degraded; assert compare exits
  non-zero and names all degraded metrics.

---

#### Story 7.4: Concurrency Stress Test (`bench run --concurrency`)

**Description:** Implement the concurrency stress test within `bench run`: N goroutines write to
the same workspace simultaneously, and after all writes complete the suite verifies zero data loss
and no PostgreSQL constraint violations. This tests release gate G1 ("zero corruption under
concurrent access") and enforces NFR-1. The test is triggered via a dedicated flag rather than a
separate subcommand so it runs in the same bench infrastructure as quality measurement.

**Covers:** FR-41

**Applies:** AR-1, AR-2, AR-8, AR-16, AR-18

**Complexity:** M

**Acceptance Criteria:**

- Given N=10 concurrent writer goroutines, when `nano-brain bench run --concurrency=10` is
  executed against a running nano-brain instance, then it exits 0, all goroutines complete, and
  the result summary reports "0 documents lost, 0 constraint violations".
- Given N concurrent writes all targeting the same workspace, when the test completes, then
  `SELECT COUNT(*) FROM documents WHERE workspace_hash = $1` equals the expected total (N ×
  documents_per_writer), with no missing rows.
- Given a PostgreSQL constraint violation (e.g., unique key conflict causing an unexpected error),
  when the concurrency test detects one, then it exits non-zero and reports which constraint was
  violated and at what approximate write count.
- Given `--concurrency=N` where N > 1, when the test runs, then all N goroutines start
  approximately simultaneously (not sequentially) — verified by overlapping write timestamps in
  the result output.
- Given `go test -race` is run against the bench package, when the race detector is active, then
  zero data races are reported (NFR-1 enforcement).
- Given `--json` flag is passed, when the test completes, then output includes
  `{"concurrency": N, "documents_written": M, "documents_verified": M, "violations": 0}`.

**Test expectations:**
- Unit: concurrency harness logic (goroutine fan-out, result aggregation) tested with mock storage.
- Integration (`//go:build bench`): N=10 writers on real PostgreSQL; post-run row count matches
  expected total; `go test -race` passes.
- Edge case: N=1 should pass trivially (no concurrency, sanity check).

---

#### Story 7.5: Bench CLI Plumbing and `--save` Round-trip

**Description:** Wire the `bench` subcommand tree into the nano-brain CLI (alongside `query`,
`write`, `status`, etc.) and validate the full save/load/compare round-trip. A developer should
be able to generate a dataset, run the benchmark, save the results, then run again after a config
change and compare both files. This story covers the CLI integration and ensures FR-40 (`--save`)
works end-to-end across all bench subcommands that produce output.

**Covers:** FR-40 (save round-trip integration)

**Applies:** AR-1, AR-16, AR-17

**Complexity:** S

**Acceptance Criteria:**

- Given the nano-brain binary, when `nano-brain bench --help` is run, then the output lists three
  subcommands: `generate`, `run`, and `compare`, each with their flags documented.
- Given `nano-brain bench run --scale=100 --save /tmp/result-a.json` completes, when the saved
  file is read, then it is valid JSON containing `p_at_5`, `r_at_10`, `mrr`, `latency_p50_ms`,
  `latency_p95_ms`, `scale`, `workspace_hash`, `timestamp`, and `nano_brain_version` fields.
- Given two saved result files from different runs, when `nano-brain bench compare result-a.json
  result-b.json` is run, then it reads both files without error and produces a comparison — the
  full generate → run → save → compare workflow succeeds end-to-end.
- Given `NANO_BRAIN_HOST` and `NANO_BRAIN_PORT` are set, when any bench subcommand is run, then
  it connects to the specified host and port (inherits FR-83 CLI env var support).
- Given `--json` is passed to `bench run`, when the output is piped to `bench compare` via a
  temporary file, then the file is a valid input to `bench compare` with no format translation
  needed (same JSON schema for stdout and --save output).

**Test expectations:**
- Unit: JSON serialization/deserialization of BenchmarkResult struct is lossless across all metric fields.
- Integration: full CLI round-trip from generate to compare using a real nano-brain instance and
  a real workspace; assert non-zero exit from compare when results are manually degraded.
