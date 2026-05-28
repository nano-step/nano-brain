## Tasks

- [ ] T1: Update `InitWorkspace` signature in `workspace.go` — add `fw *watcher.Watcher`, `watcherCfg config.WatcherConfig`
- [ ] T2: After commit in `InitWorkspace`, call `fw.WatchWithFilter` for memory/sessions/code collections (nil-guard fw)
- [ ] T3: Update `routes.go` — pass `s.watcher`, `s.currentConfig().Watcher` to `InitWorkspace`
- [ ] T4: Update `workspace_test.go` and `workspace_integration_test.go` — add `nil, config.WatcherConfig{}` params
- [ ] T5: `go build ./...` → exit 0
- [ ] T6: `go test -race -short ./...` → all pass
- [ ] T7: Review gate → tag v2026.5.2712 → push via kokorolx
