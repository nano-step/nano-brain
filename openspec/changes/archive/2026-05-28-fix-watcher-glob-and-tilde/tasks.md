## Tasks

- [ ] T1: `internal/watcher/watcher.go` — replace `scanCollection` body with `filepath.WalkDir`; add `errors`, `io/fs` imports
- [ ] T2: `internal/server/handlers/workspace.go` — `initWorkspace`: expand tilde using `os.UserHomeDir()` for memoryPath/sessionsPath; add `os` import if missing
- [ ] T3: `go build ./...` → exit 0
- [ ] T4: `go test -race -short ./...` → all pass
- [ ] T5: Smoke: restart server, call `init --root <path> --force`, wait 10s, `GET /api/v1/workspaces` shows `document_count > 0`
- [ ] T6: Review gate → tag v2026.5.2713 → push via kokorolx
