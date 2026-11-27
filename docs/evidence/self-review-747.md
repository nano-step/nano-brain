# Self-review — Issue #747

## Actions Taken
- Added the `created_*`/`updated_*` `narg` filters to `BM25SearchOR` and `BM25SearchAllOR`, then ran `sqlc generate`.
- In both hybrid fallback blocks, passed `timeRange.ToSqlNullTimes()` to the OR queries, and skipped the fallback when tags are set.
- In `memory_search`, the OR relaxation now passes the time range. Before, it was disabled whenever a time filter was set.
- Added one unit test and one DB integration test. Ran smoke:e2e comparing the master binary against the fixed binary.

## Files Changed
- internal/storage/queries/search.sql
- internal/storage/sqlc/search.sql.go (generated)
- internal/search/service.go
- internal/mcp/tools.go
- internal/search/service_test.go
- internal/server/handlers/timefilter_integration_test.go

## Findings Summary
- No critical or major findings. The independent review (review-747.md) gave a PASS with 2 medium and 2 low findings.
- `go test -race -short ./...`: pass. `go test -race -tags=integration ./...`: pass.

## Resolution Status
- The medium SQL-coverage finding is resolved by the new integration test.
- The medium config-leg behaviour change is documented in the PR.
- The low findings are accepted and out of scope.
