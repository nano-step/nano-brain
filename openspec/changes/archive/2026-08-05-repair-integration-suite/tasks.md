## 1. Fixture Alignment

- [x] 1.1 Change the backfill fixture to insert unified `sessions` summaries.
- [x] 1.2 Replace cleanup fixture sqlc upserts with pre-00011-compatible
  inserts while retaining its migration cap.
- [x] 1.3 Add required `session.parent_id` columns and canonical summary
  collection expectations to OpenCode SQLite fixtures.
- [x] 1.4 Align stale-raw and summary-persistence expectations with the active
  session collection and date-first disk naming contracts.
- [x] 1.5 Align the MCP wake-up fixture with the canonical `sessions`
  collection filter.

## 2. Benchmark Isolation

- [x] 2.1 Require the explicit `benchmark` tag for the live benchmark test.
- [x] 2.2 Document the required combined build-tag invocation and port-3199
  setup at the benchmark test boundary.

## 3. Verification

- [x] 3.1 Run targeted red-to-green integration tests for backfill and cleanup.
- [x] 3.2 Run normal integration tests without a live benchmark server.
- [x] 3.3 Run strict OpenSpec validation, self-review, and independent review.
