# Add Read-Only Multi-Workspace and Replay Evaluation Benchmark Suite (#529)

## Why

Existing capability tests under `benchmarks/**/capability` run per-directory via
`go test -tags=capbench`, speak only REST, report only flat substring recall, do
not measure context-gathering token efficiency vs naive grep + file reads (#529),
cannot replay real agent MCP traffic from `nano-brain-observe` JSONL logs, and
lack a transport-level read-only guarantee when pointed at a live server.

## What

- Add package `internal/evalbench` providing a zero-mutation evaluation engine:
  - `ReadOnlyTransport` (`http.RoundTripper`) blocking every mutating MCP tool
    (`memory_write`, `memory_delete`, `memory_update`, unknown tools) and every
    mutating HTTP method/path (`/api/v1/write`, `/api/v1/reindex`,
    `/api/v1/update`, `/api/v1/init`, `/api/v1/reset-workspace`, `/api/v1/config`,
    `/api/harvest`, `/api/reload-config`, `PUT`, `PATCH`, `DELETE`) with
    `ErrForbiddenMutation` before leaving the client, paired with pre/post
    workspace `(doc_count, chunk_count)` snapshot diffing (`--strict-immutable`).
  - Dual-transport caller (`mcp` Streamable HTTP default with JSON + SSE parsing,
    `rest` optional) measuring per-call latency, response bytes, estimated tokens,
    collection mix (`code`/`memory`/`sessions`), `RecallAt5`, `MRR`, `FixedRecall`,
    `AgentRecall`, in-memory `__PROJECT__` template substitution, and read-only
    grep + ground-truth file read token-savings proxy (#529).
  - Read-only `nano-brain-observe` JSONL log replay (`calls-*.jsonl`) filtering
    strictly to read-only `mcp_tool` calls and anonymizing workspace slots/queries
    by default unless `--verbose-private` is passed.
  - Markdown + JSON scorecards and baseline regression comparison (`--compare`).
- Expose `nano-brain bench eval` in `cmd/nano-brain` honoring `resolveCLIConfigPath`.
