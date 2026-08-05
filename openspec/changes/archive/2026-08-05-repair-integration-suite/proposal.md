## Why

Tracking: #605

The normal integration suite fails because two fixtures drifted from their
schema/storage contracts and a live benchmark runs without its required test
server. This prevents the harness validation ladder from reporting the real
state of changes.

## What Changes

- Align backfill summary fixtures with the unified `sessions` collection.
- Preserve cleanup's pre-foreign-key test schema while inserting fixture rows
  through columns available at that migration version.
- Align stale-raw, harvest SQLite, and summary-persistence fixtures with the
  current session schema, collection, and disk filename contracts.
- Align the MCP wake-up integration fixture with the canonical `sessions`
  filter.
- Require an explicit `benchmark` build tag for the live-server benchmark.

## Capabilities

### New Capabilities

- `integration-suite-reliability`: Runs database integration tests without
  stale fixture assumptions or an implicit live benchmark dependency.

### Modified Capabilities

- None.

## Impact

- Test-only Go files under `cmd/nano-brain/`, `internal/bench/`,
  `internal/harvest/`, `internal/mcp/`, `internal/storage/sqlc/`, and
  `internal/summarize/`.
- No production database migration, runtime API, or benchmark implementation
  behavior changes.
