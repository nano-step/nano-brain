# Design

## Architecture (`internal/evalbench` + `cmd/nano-brain/bench_eval.go`)

1. **Transport-Level Zero-Mutation Guard (`guard.go`):**
   - `ReadOnlyTransport` wraps `http.RoundTripper` and validates every outgoing
     HTTP request prior to network transmission:
     - `GET` allowlist: `/health`, `/api/status`, `/api/v1/workspaces`,
       `/api/v1/stats`, `/api/v1/symbols`, `/api/v1/tags`, `/api/v1/wake-up`,
       `/api/v1/documents`, `/api/v1/documents/*`.
     - `POST /mcp`: parses JSON-RPC body (restoring `req.Body`) and permits only
       `initialize`, `notifications/initialized`, `tools/list`, `ping`, and
       `tools/call` whose `params.name` is in the read-only MCP tool allowlist.
     - `POST` REST allowlist: `/api/v1/query`, `/api/v1/search`, `/api/v1/vsearch`,
       `/api/v1/get`, `/api/v1/multi-get`, `/api/v1/graph`, `/api/v1/graph/query`,
       `/api/v1/graph/impact`, `/api/v1/graph/trace`, `/api/v1/graph/flow`,
       `/api/v1/graph/flowchart`, `/api/v1/workspaces/resolve`, `/api/v1/wake-up`.
   - `CaptureWorkspaceSnapshot` + `DiffSnapshots` compare `(doc_count, chunk_count)`
     before and after evaluation on target workspaces (`--strict-immutable`).

2. **Dual-Transport Capability & Token-Cost Engine (`client.go`, `capability.go`):**
   - Supports `mcp` (JSON + SSE `event: message` framing, `X-Requested-With: nano-brain-ui`)
     and `rest` transports across suites `go`, `typescript`, `vue`, and `rails`.
   - Resolves workspaces read-only via `GET /api/v1/workspaces` /
     `memory_workspaces_resolve`; unregistered suites report `status: "skipped"`.
   - Substitutes template variables (e.g. `__PROJECT__`) in memory from runtime
     gitignored `--profiles` (`.omc/bench-eval.local.json`) or `--var KEY=VAL`.
   - Computes `FixedRecall`, `AgentRecall`, `RecallAt5`, `MRR`, per-tool `p50`/`p95`
     latency, collection distribution, and #529 `TokenSavingsPct` against cached
     read-only ground-truth file reads + expected-symbol grep snippet lines.

3. **Observe Log Replay & Regression Comparison (`replay.go`, `report.go`, `compare.go`):**
   - Replays only read-only `kind == "mcp_tool"` entries from `calls-*.jsonl`,
     counting `skipped_mutations`, computing live vs recorded empty/error/latency
     metrics and top-5 Jaccard overlap, and redacting private query/path strings
     into anonymized workspace slots (`ws-1`, `ws-2`) unless `--verbose-private`.
