## Tasks

- [ ] T1: Add `GetCurrentVersion(ctx, pool) (int64, error)` to `internal/storage/migrate.go`
- [ ] T2: In `cmd/nano-brain/main.go`, call `GetCurrentVersion` after `RunMigrations`; pass result to server handler chain
- [ ] T3: In `internal/server/handlers/health.go`, accept `migrationVersion int64` in constructor; replace hardcoded `1` with field value
- [ ] T4: In `cmd/nano-brain/commands.go`, update `triggerInitBackground` to print harvest result (harvested/skipped/errors) to stdout
- [ ] T5: Run `go build ./...` + `go test -race -short ./...`
- [ ] T6: Smoke test: start server, `GET /api/status` → verify `migration_version` ≠ 1 (should be 9), run `init --force`, verify harvest output printed
- [ ] T7: Review gate → tag → push
