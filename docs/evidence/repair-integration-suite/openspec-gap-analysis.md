# Gap Analysis: Repair Integration Suite

Tracking: #605

## Scope

Restore the normal integration suite without changing production behavior.

## Baseline Failures

- Backfill and stale-raw fixtures used the retired `session-summary` collection
  while production selects `sessions`.
- Cleanup intentionally stopped at migration 10 but used an sqlc insert that
  requires columns added in later migrations.
- The live benchmark ran with `integration` alone and required an undeclared
  server on port 3199.
- The full suite exposed additional stale fixture contracts: OpenCode SQLite
  sessions lacked `parent_id`, summary persistence expected the old collection
  and filename form, and MCP wake-up used the old collection.

## Resolution

- Updated test fixtures and expectations to the active production contracts.
- Kept the cleanup test's migration-10 scenario and inserted only columns
  available in that schema.
- Isolated the live benchmark behind an explicit `benchmark` build tag.

## Acceptance Evidence

- `go build ./...` passed.
- `go test -race -short ./...` passed.
- `go test -race -tags=integration -count=1 ./...` passed against
  `nanobrain_test`.
- `openspec validate repair-integration-suite --strict` passed.
