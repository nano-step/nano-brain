
## 2026-07-02 — Plan 12-02 Task 1

- **Pre-existing gofmt issue** in `internal/server/handlers/reset_workspace.go` and `internal/server/handlers/workspace_remove.go` — an if/else block is indented one tab short of gofmt's expectation. Confirmed pre-existing via `git stash` diff before any Plan 12-02 edits landed. Out of scope for this annotation-only plan (Rule scope boundary) — not fixed.

## 2026-07-02 — Plan 12-02 Task 3

- `/ui` (routes.go line 145, static HTML dashboard-moved redirect) is deliberately excluded from the OpenAPI spec — not a JSON REST endpoint. No annotation added. Plan 04's route-count reconciliation should treat `/ui` as an intentional exclusion, not a gap.
- Assumption A2 (RESEARCH.md) CONFIRMED PASS: the placeholder-anchor function pattern (`mcpProtocolDoc`/`sseProtocolDoc` in `internal/server/handlers/protocol_doc.go`) is correctly parsed by swag with zero fallback needed — /mcp (GET/POST/DELETE) and /sse (GET/POST) all appear in the generated spec.
