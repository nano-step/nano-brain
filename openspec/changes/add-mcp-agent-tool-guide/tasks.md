## 1. MCP-visible guidance

- [x] 1.1 Update search tool descriptions with explicit selection guidance: default to `memory_query`, exact text uses `memory_search`, fuzzy concepts use `memory_vsearch`.
- [x] 1.2 Update retrieval/persistence descriptions: use `memory_get` after search, `memory_write` for decisions, `memory_wake_up` at session start, `memory_status` for indexing/queue health.
- [x] 1.3 Update code-intelligence descriptions: `memory_symbols` for known names, `memory_graph` for one-hop callers/callees, `memory_impact` before changes, `memory_trace` for downstream calls, `memory_flow` for HTTP entries, `memory_flowchart` for function control flow.
- [x] 1.4 Update parameter descriptions where they affect tool choice, especially `query`, `node`, `entry`, `direction`, `edge_type`, and `paths`.

## 2. Tests

- [x] 2.1 Add or update MCP tool-list tests to assert the guidance is exposed through tool descriptions.
- [x] 2.2 Run targeted MCP tests.

## 3. Validation

- [x] 3.1 Run `go test -race -short ./...`.
- [x] 3.2 Run privacy grep before finalizing.
