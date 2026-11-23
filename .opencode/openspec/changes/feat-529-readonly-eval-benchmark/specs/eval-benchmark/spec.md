# Read-Only Evaluation Benchmark Suite

## ADDED Requirements

### Requirement: Transport-level read-only enforcement and workspace snapshot guard
The evaluation benchmark runner SHALL wrap all HTTP traffic in `ReadOnlyTransport`
so every non-read-only HTTP method, REST route, or MCP `tools/call` fails client-side
with `ErrForbiddenMutation` before reaching the server, and SHALL capture pre/post
workspace document and chunk counts to report `mutated` status.

#### Scenario: Mutating MCP tools and REST endpoints are blocked before network send
- **WHEN** the benchmark client attempts `tools/call` for `memory_write`, `memory_delete`, `memory_update`, or an unlisted tool over `/mcp`, or sends requests to `/api/v1/write`, `/api/v1/reindex`, `/api/v1/update`, `/api/v1/init`, `/api/v1/reset-workspace`, `/api/v1/config`, `/api/harvest`, `/api/reload-config`, or uses HTTP `PUT`, `PATCH`, or `DELETE`
- **THEN** `ReadOnlyTransport` returns `ErrForbiddenMutation` and zero mutating requests reach the upstream HTTP server

#### Scenario: Snapshot guard detects concurrent workspace count changes
- **WHEN** `CaptureWorkspaceSnapshot` is recorded before and after an evaluation run on target workspace hashes
- **THEN** `DiffSnapshots` reports `mutated: false` when `(doc_count, chunk_count)` are unchanged and `mutated: true` when any target workspace count changed, failing the command when `--strict-immutable` is enabled

### Requirement: Multi-workspace capability, ranking, latency, and token-efficiency evaluation
The benchmark engine SHALL evaluate capability datasets (`go`, `typescript`,
`vue`, `rails`, or custom files) over MCP Streamable HTTP (JSON and SSE framing)
or REST without writing temporary dataset files or calling workspace `init`,
computing `FixedRecall`, `AgentRecall`, `RecallAt5`, `MRR`, per-tool `p50`/`p95`
latency, collection distribution (`code`/`memory`/`sessions`/`other`), and read-only
context-gathering token savings vs naive grep + ground-truth file reads (#529).

#### Scenario: Multi-workspace suite execution with template substitution and token-cost proxy
- **WHEN** `nano-brain bench eval` runs capability suites with runtime `--var PROJECT=app` or `--profiles` against registered workspaces on MCP or REST
- **THEN** placeholders such as `__PROJECT__` are substituted in memory, unregistered workspaces are marked `skipped` without mutating server state, and each evaluated suite reports `FixedRecall`, `AgentRecall`, `RecallAt5`, `MRR`, per-tool latency `p50`/`p95`, collection mix, and `TokenSavingsPct` against naive grep + file reads on local ground-truth files

### Requirement: Privacy-preserving observe JSONL log replay and baseline regression comparison
The benchmark engine SHALL replay read-only `kind == "mcp_tool"` entries from
`nano-brain-observe` JSONL logs (`calls-*.jsonl`), skip mutating tools while
counting `skipped_mutations`, redact private query strings and workspace names
by default, and detect regressions against a saved baseline JSON report.

#### Scenario: Observe JSONL replay skips mutations and redacts private traffic by default
- **WHEN** `bench eval` replays JSONL logs containing `memory_query`, `memory_get`, `memory_write`, handshake `mcp`, and `rest` entries without `--verbose-private`
- **THEN** only read-only `mcp_tool` calls are replayed over `/mcp`, `memory_write` increments `skipped_mutations`, `EmptyRate`, `ErrorRate`, `p50`/`p95` latency, and `Top5Overlap` are reported by tool and anonymized workspace slot (`ws-1`, `ws-2`), and `--compare` exits non-zero when recall, MRR, empty rate, token usage, or latency regresses beyond threshold

