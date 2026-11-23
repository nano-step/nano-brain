# Tasks

- [x] Create `internal/evalbench` zero-mutation guard (`guard.go`) with
      `ReadOnlyTransport`, `ErrForbiddenMutation`, `CaptureWorkspaceSnapshot`,
      and `DiffSnapshots`.
- [x] Implement dual-transport caller (`client.go`) supporting MCP Streamable
      HTTP (JSON and SSE framing) and REST with latency, byte/token, ranked item,
      and collection tracking.
- [x] Implement Tier 1 capability runner (`capability.go`) with in-memory
      placeholder substitution, `FixedRecall`, `AgentRecall`, `RecallAt5`, `MRR`,
      per-tool `p50`/`p95` latency, collection mix, and #529 naive grep+read
      token-savings proxy.
- [x] Implement Tier 2 `nano-brain-observe` JSONL replay (`replay.go`) with
      read-only filtering, `SkippedMutations` accounting, `Top5Overlap`, and
      privacy-by-default workspace anonymization.
- [x] Implement Markdown/JSON scorecards (`report.go`), baseline regression
      comparison (`compare.go`), and `nano-brain bench eval` CLI (`bench_eval.go`).
- [x] Add comprehensive `httptest.NewServer` unit tests (`internal/evalbench/*_test.go`,
      `cmd/nano-brain/bench_eval_test.go`), update `CHANGELOG.md`, and run full
      validation ladder.
