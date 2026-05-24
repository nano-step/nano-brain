## Design

### Architecture

Daemon mode uses self-re-exec pattern (common in Go CLIs):

```
User runs: nano-brain serve -d
  → Parent process:
    1. Resolve binary path (os.Executable)
    2. Check PID file — if exists + alive, error exit
    3. Open log file (~/.nano-brain/nano-brain.log)
    4. Exec child: os.StartProcess(self, ["nano-brain", "--daemon-child", "--config", path], ...)
       - Stdout/Stderr → log file
       - Stdin → /dev/null
       - SysProcAttr.Setsid = true (detach from terminal)
    5. Write child PID to ~/.nano-brain/nano-brain.pid
    6. Print "nano-brain started (PID: N)"
    7. Exit 0

  → Child process (--daemon-child):
    1. Detected via flag.Bool("daemon-child", ...)
    2. Runs normal server startup (lines 91-241 of current main.go)
    3. On SIGTERM: graceful shutdown (already implemented)
    4. On exit: remove PID file (deferred)
```

### New file: `cmd/nano-brain/daemon.go`

~150 lines. Functions:

1. **runServeCmd(args, configPath)** — parse -d flag; if -d → runServeDaemon; else → run server foreground (call startServer)
2. **runServeDaemon(configPath)** — self-re-exec, PID file, log file redirect
3. **runStopCmd()** — read PID, SIGTERM, wait, cleanup PID file
4. **runRestartCmd(args, configPath)** — stop then serve -d
5. **pidFilePath()** — returns ~/.nano-brain/nano-brain.pid
6. **readPID()** — read + parse + validate PID file
7. **isRunning(pid)** — signal 0 check (Unix)

### Refactor: extract startServer()

Current main() lines 91-241 contain server startup logic inline. Extract to:
```go
func startServer(configPath string)
```
Called by:
- No-args path (existing behavior)
- `serve` command (foreground)
- `--daemon-child` path (background child)

This is a pure extraction — no behavior change.

### PID file management

- Path: `~/.nano-brain/nano-brain.pid`
- Written by parent after `os.StartProcess` succeeds
- Removed by child on clean exit (`defer os.Remove(pidFilePath())`)
- Stale PID detection: `readPID()` + `isRunning()` — if PID file exists but process dead, overwrite

### Log file for daemon mode

- If `cfg.Logging.File` is empty and `-d` flag is set:
  - Auto-set log path to `~/.nano-brain/nano-brain.log`
  - Print info: "Logging to ~/.nano-brain/nano-brain.log"
- If `cfg.Logging.File` is already set → use that path
- The `logs` command already reads from config logging.file path

### Platform considerations

- `SysProcAttr.Setsid` — Unix only (linux, darwin). Not available on Windows.
- `isRunning` uses `syscall.Kill(pid, 0)` — Unix only.
- Build constraint: `//go:build !windows` on daemon.go
- Windows: `serve -d` prints "daemon mode not supported on Windows" and exits 1

### Dependencies
- No new dependencies
- stdlib only: os, os/exec, fmt, strconv, syscall, path/filepath
