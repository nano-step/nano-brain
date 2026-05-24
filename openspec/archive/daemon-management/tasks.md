## Tasks

- [ ] Extract `startServer(configPath string)` from main() lines 91-241 — pure refactor, no behavior change
- [ ] Add `--daemon-child` flag to main(), route to startServer when set
- [ ] Create `cmd/nano-brain/daemon.go` with serve/stop/restart/PID management
- [ ] Add `case "serve"`, `case "stop"`, `case "restart"` to main.go switch
- [ ] Update `printUsage()` in ops.go with new commands
- [ ] Build + unit test (go build, go test -race -short)
- [ ] E2E: `serve -d` starts background, PID file written
- [ ] E2E: `stop` terminates server, PID file removed
- [ ] E2E: `restart` cycles server
- [ ] E2E: `serve -d` when already running → error
- [ ] E2E: `stop` when not running → clear message
- [ ] E2E: `logs -f` reads daemon log file
