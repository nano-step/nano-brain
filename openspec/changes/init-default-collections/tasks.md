## 1. Implementation

- [ ] 1.1 In `internal/server/handlers/workspace.go`, add a fourth `UpsertCollection` call inside `initWorkspace` for the `code` collection: `name="code"`, `path=absPath`, `glob_pattern="**/*"`, `update_mode="auto"`. Place it after the `sessions` upsert and inside the same transaction block.

## 2. Tests

- [ ] 2.1 In `internal/server/handlers/workspace_test.go`, update the mock expectation in `TestInitWorkspace` (or equivalent) to assert a fourth `UpsertCollection` call with `name="code"` and `path=<expected_abs_path>`.
- [ ] 2.2 Add a test case for idempotency: calling `initWorkspace` twice with the same path produces exactly one `code` collection row (verify via `UpsertCollection` semantics — already ON CONFLICT DO UPDATE).
- [ ] 2.3 Add a test case for transaction rollback: if the `code` collection upsert returns an error, the handler returns 500 and no rows are committed.

## 3. Validation

- [ ] 3.1 `CGO_ENABLED=0 go build ./...` — compiles cleanly.
- [ ] 3.2 `go vet ./...` — no vet errors.
- [ ] 3.3 `go test -race -short ./...` — all tests pass.
- [ ] 3.4 Manual smoke: start server, run `curl -X POST localhost:3100/api/v1/init -d '{"root_path":"/tmp/test-workspace"}'`, then `curl "localhost:3100/api/v1/collections?workspace=<hash>"` — assert `code` collection with `path=/tmp/test-workspace` appears.
