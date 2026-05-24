## Spec: Daemon Management

### Commands

| Command | Behavior |
|---------|----------|
| `nano-brain serve` | Start server foreground |
| `nano-brain serve -d` | Start server background, write PID file, exit |
| `nano-brain stop` | Read PID file, SIGTERM, wait, cleanup |
| `nano-brain restart` | stop + serve -d |
| `nano-brain` (no args) | Unchanged — foreground server |

### serve -d

**Preconditions:**
- Config file exists and is valid
- PID file does not exist OR referenced process is dead

**Flow:**
1. Check PID file — if alive, print "already running (PID: N)" exit 1
2. Resolve self binary path via `os.Executable()`
3. Open/create log file (`~/.nano-brain/nano-brain.log` or config value)
4. `os.StartProcess(self, args, &os.ProcAttr{...})`
5. Write child PID to `~/.nano-brain/nano-brain.pid`
6. Print "nano-brain started (PID: N)" to stdout
7. Exit 0

**Error cases:**
- Already running → "nano-brain is already running (PID: N)"
- Config invalid → error from config.Load
- Binary not found → error from os.Executable
- Log file not writable → error with path info

### stop

**Flow:**
1. Read PID from `~/.nano-brain/nano-brain.pid`
2. Check process alive via `kill -0`
3. Send SIGTERM
4. Poll every 100ms up to 5s for process to exit
5. Remove PID file
6. Print "nano-brain stopped"

**Error cases:**
- PID file not found → "nano-brain is not running"
- Process not alive → remove stale PID file, print "nano-brain is not running (cleaned stale PID file)"
- Timeout after 5s → "nano-brain did not stop gracefully, sending SIGKILL" → kill -9

### restart

1. Run stop logic (ignore "not running" error)
2. Run serve -d logic

### --daemon-child (internal)

Hidden flag. When set:
1. Skip command dispatch
2. Remove PID file on exit (defer)
3. Run startServer(configPath) — normal server startup

### Files

| File | Change |
|------|--------|
| `cmd/nano-brain/daemon.go` | NEW — all daemon logic (~150 lines) |
| `cmd/nano-brain/main.go` | Add serve/stop/restart cases, extract startServer(), add --daemon-child handling |
| `cmd/nano-brain/ops.go` | Update printUsage() with new commands |
