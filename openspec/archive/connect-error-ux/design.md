# Design: Connect-Error UX

## Architecture

All changes live in `cmd/nano-brain/client.go` — the single chokepoint for CLI → server HTTP traffic. Every existing CLI command (init, write, query, search, vsearch, harvest, status, collection, …) already routes through `doRequest()`. By upgrading the error path there, no per-command changes are needed.

New helpers (same package, same file or split into `client_help.go`):

```
isTTY() bool                                  // stdin AND stderr are TTY
isNpxLaunched() bool                          // detect launch via npx
suggestStartCommand() string                  // returns "npx @nano-step/nano-brain@beta serve -d" or "nano-brain serve -d"
promptStartServer(scanner *bufio.Scanner) bool // returns true if user accepted
waitForServerHealthy(timeout time.Duration) error // poll GET /api/status
```

Modified function: `doRequest()` — when the original `http.Client.Do` returns `connection refused`, instead of formatting an opaque error, route into a recovery flow.

## Flow

```
doRequest(method, url, body)
  │
  ├─ httpClient.Do(req)
  │   ├─ SUCCESS → return response (unchanged path)
  │   └─ ERROR "connection refused"
  │       ↓
  │   formatConnectError(host, port)
  │       ├─ build header line: "Error: cannot connect to nano-brain server at <host>:<port>"
  │       ├─ build hint line:   "The server does not appear to be running."
  │       ├─ build action line: "Run this to start it: <suggestStartCommand()>"
  │       │
  │       ├─ NANO_BRAIN_NO_AUTO_START=1?   → return formatted error, exit non-zero
  │       ├─ isTTY() == false?              → return formatted error, exit non-zero (CI/agent)
  │       │
  │       └─ Both TTY → promptStartServer()
  │              ├─ User says NO  → return formatted error, exit non-zero
  │              └─ User says YES → runServeDaemon(configPath)  (existing func from daemon.go)
  │                     ├─ daemon fork fails → print error, exit non-zero
  │                     └─ daemon forked    → waitForServerHealthy(10s)
  │                           ├─ healthy           → retry original request ONCE
  │                           │     └─ success     → return response
  │                           │     └─ failure     → return original-style error (no second retry)
  │                           └─ timeout/unhealthy → "Server started but did not become healthy in 10s.
  │                                                   Check logs: ~/.nano-brain/logs/nano-brain.log"
```

## Key Decisions

### 1. Why `serve -d` not `docker start`

- `serve -d` runs the existing binary as a background daemon, no Docker dependency.
- `docker start` introduces Postgres lifecycle, port 5432 conflicts, Docker daemon requirement — too many failure modes to auto-trigger.
- Users without Docker can still use this flow (they have Postgres installed locally).
- Users with Docker who want `docker start` can ignore the prompt and run it manually — the error suggestion is just a hint.

### 2. Why retry ONCE, not loop

- One retry covers 99% of cases (server starts in <2s typically).
- Looping risks infinite recovery on real bugs.
- If the retry fails, we surface the second error so the user sees what's actually broken.

### 3. TTY detection

- Both `os.Stdin` AND `os.Stderr` must be TTYs.
- Stdin alone is not enough — agent harnesses (OpenCode, Claude Code) may inherit a TTY but the user can't actually answer.
- Use `term.IsTerminal(int(fd))` from `golang.org/x/term` (already in dep tree via existing CLI tooling) **OR** stdlib `(os.Stdin.Stat().Mode() & os.ModeCharDevice) != 0` for zero new deps.
- **Decision: stdlib path** — avoids adding `x/term` if not already imported. Verify in implementation.

### 4. npx vs binary detection

Check `os.Args[0]` and key env vars:

```go
// npx leaves these breadcrumbs:
//  - process arg may end in /nano-brain (a wrapper script)
//  - npm_lifecycle_event, npm_package_name set
//  - npm_execpath set to /path/to/npx-cli.js
func isNpxLaunched() bool {
    if os.Getenv("npm_execpath") != "" { return true }
    if os.Getenv("npm_package_name") != "" { return true }
    return false
}
```

Suggestion mapping:
- npx → `npx @nano-step/nano-brain@beta serve -d` (always pin to @beta channel until 1.0)
- binary → `nano-brain serve -d`

### 5. Why centralize in `doRequest`

- 14+ CLI commands all currently route through `doRequest`. Fixing it once = fixing all.
- Per-command fixes would duplicate logic and inevitably drift.
- Test surface stays small (1 function, all paths exercised by client_test.go).

### 6. Health-check polling

- Poll `GET <baseURL>/api/status` every 200ms.
- 10s total budget (50 attempts).
- Success criterion: HTTP 200 with valid JSON body.
- On timeout, surface a specific error pointing at the log file location.

## Files Changed

- `cmd/nano-brain/client.go` — modify `doRequest`, add helpers (NEW logic in same file)
- `cmd/nano-brain/client_test.go` — NEW tests for TTY detection, error formatting, retry logic
  - Note: existing test file is `cmd/nano-brain/commands_test.go` for client behavior; may consolidate.
- `cmd/nano-brain/daemon.go` — NO changes (reuse `runServeDaemon`)
- `cmd/nano-brain/main.go` — NO changes
- `README.md` — add a sentence under troubleshooting (optional, defer if PR gets large)

## Test Plan

Unit tests (in `client_test.go`):

1. `TestSuggestStartCommand_NpxLaunched` — env vars set → npx string
2. `TestSuggestStartCommand_BinaryLaunched` — env vars unset → binary string
3. `TestIsTTY_Stdin` — table-driven with fake fds
4. `TestFormatConnectError_Structure` — verify all 3 lines present
5. `TestConnectionRefused_NoTTY` — mock httptest unreachable URL + force `os.Stdin` to non-TTY → no prompt, no auto-start
6. `TestConnectionRefused_NoAutoStartEnv` — `NANO_BRAIN_NO_AUTO_START=1` → no prompt even in TTY
7. (Integration, optional, gated by build tag): bring up real server, kill, run command, expect prompt mocked via stdin pipe.

Manual smoke (post-implementation):

```bash
# 1. Server down
killall nano-brain 2>/dev/null
npx @nano-step/nano-brain@beta init --root $(pwd)
# Expect: clear error + suggestion + (if TTY) Y/n prompt
# On Y: server starts, init retries, succeeds

# 2. NANO_BRAIN_NO_AUTO_START
NANO_BRAIN_NO_AUTO_START=1 npx @nano-step/nano-brain@beta init --root $(pwd)
# Expect: error + suggestion, no prompt

# 3. Pipe (non-TTY)
echo "" | npx @nano-step/nano-brain@beta init --root $(pwd)
# Expect: error + suggestion, no prompt
```

## Known Limitations

- `runServeDaemon` currently calls `os.Exit(1)` on failure (does not return an error). The v1 implementation accepts this: if the daemon fails to fork, the user sees its existing error message and the retry path never executes. Refactoring `runServeDaemon` to return an `error` is a clean follow-up (see tasks.md 3.4).

## Out-of-Scope (Future Work)

- Auto-start via `docker start` — separate proposal if requested
- Server-readiness check in `serve -d` itself so daemon.go waits before backgrounding (would let us drop the polling here) — orthogonal improvement
- Refactor `runServeDaemon` to return `error` instead of `os.Exit` — orthogonal improvement
- Localize error messages — proposal stays English-only
