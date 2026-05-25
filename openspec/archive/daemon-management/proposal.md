Tracking: #135

## Why

Users have no way to start nano-brain server in the background. Current options:
- `nano-brain` (no args) — foreground, blocks terminal
- Manual `nohup ... &` — no PID management, no clean stop

This is poor UX for a persistent memory server that should run continuously.

## What Changes

### New: `nano-brain serve`
- Start server foreground (identical to current no-args behavior)
- Accepts `-d` flag for daemon (background) mode

### New: `nano-brain serve -d`
- Re-exec binary as detached child process with `--daemon-child` flag
- Child redirects stdout/stderr to `~/.nano-brain/nano-brain.log`
- Write PID to `~/.nano-brain/nano-brain.pid`
- Parent exits immediately after child is running
- If already running (PID file exists + process alive) → error with PID info
- If `logging.file` not configured → auto-set to `~/.nano-brain/nano-brain.log`

### New: `nano-brain stop`
- Read `~/.nano-brain/nano-brain.pid`
- Send SIGTERM → graceful shutdown (existing signal handler)
- Wait up to 5s for process to exit
- Remove PID file
- If not running → clear "not running" message

### New: `nano-brain restart`
- `stop` + `serve -d` in sequence

### Modified: `cmd/nano-brain/main.go`
- Add `case "serve"` to dispatch switch → routes to serve handler
- Add `case "stop"` → routes to stop handler
- Add `case "restart"` → routes to restart handler
- Add hidden `--daemon-child` flag handling (before command dispatch)
- No-args behavior unchanged (foreground server)

### New file: `cmd/nano-brain/daemon.go`
- `runServeCmd(args, configPath)` — parse `-d` flag, dispatch
- `runServeDaemon(configPath)` — fork child, write PID, exit
- `runStopCmd()` — read PID, kill, cleanup
- `runRestartCmd(args, configPath)` — stop + serve -d
- `readPID() (int, error)` — read and validate PID file
- `isRunning(pid int) bool` — check if process alive
- `pidFilePath() string` — return `~/.nano-brain/nano-brain.pid`

### Modified: `cmd/nano-brain/ops.go`
- Update `printUsage()` to include serve/stop/restart commands
