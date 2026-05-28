# Tasks: Connect-Error UX

## Phase 1 — Helpers (no behavior change yet)

- [x] **1.1** Add `isTTY() bool` to `cmd/nano-brain/client.go` (stdlib only: check `os.Stdin` AND `os.Stderr` via `Stat().Mode() & os.ModeCharDevice`). Add unit test in `client_test.go` (or `commands_test.go` if consolidating).
- [x] **1.2** Add `isNpxLaunched() bool` — check `npm_execpath` and `npm_package_name` env vars. Unit test with table-driven cases.
- [x] **1.3** Add `suggestStartCommand() string` returning `"npx @nano-step/nano-brain@beta serve -d"` or `"nano-brain serve -d"` based on `isNpxLaunched()`. Unit test.
- [x] **1.4** Add `formatConnectError(host string, port int) string` producing the 3-line error message (header / hint / action). Unit test the exact format.

## Phase 2 — Health-check polling

- [x] **2.1** Add `waitForServerHealthy(timeout time.Duration) error` to `client.go` — polls `getBaseURL()+"/api/status"` every 200ms. Returns nil on first HTTP 200, error on timeout. Unit test with `httptest.NewServer` simulating both success-after-delay and never-ready.

## Phase 3 — Prompt + auto-start

- [x] **3.1** Add `promptStartServer(scanner *bufio.Scanner) bool` — writes prompt to stderr, reads from scanner, returns true on `Y`/`y`/empty, false otherwise. Unit test by injecting a `bytes.Buffer`-backed scanner.
- [x] **3.2** Wire the connect-error recovery path into `doRequest`:
  - Detect `connection refused` (existing branch in `doRequest`).
  - If `NANO_BRAIN_NO_AUTO_START=1` OR not TTY → print formatted error + suggestion, return original error untouched (so callers still exit non-zero).
  - If TTY → prompt. On accept, call `runServeDaemon(<configPath>)`, then `waitForServerHealthy(10*time.Second)`, then retry the request exactly once.
  - On decline / daemon-fork-fail / health-timeout → restore formatted error, return.
- [x] **3.3** Resolve `configPath` reachable from `doRequest`. Two options:
  - (a) thread `configPath` through call sites (touches all CLI commands)
  - (b) read default `config.DefaultConfigPath()` inside the recovery branch.
  - **Pick (b)** — keeps blast radius small; `runServeDaemon` already accepts a configPath argument and uses defaults internally if needed.
- [x] **3.4** Handle `runServeDaemon` exit semantics. Note: existing `runServeDaemon` calls `os.Exit(1)` internally on failure (does not return an error). Two options:
  - (a) Refactor `runServeDaemon` to return `error` and have the caller print + exit. Cleaner but touches `daemon.go` and its existing callers.
  - (b) Wrap the call: before invoking, validate prerequisites we can check (PID file absence, executable resolvable) and only fork if pre-flight passes. If `runServeDaemon` exits, the user already sees its error message, so the connect-error retry never executes — acceptable for v1.
  - **Pick (b)** — minimal blast radius. Document the limitation in design.md so future work can graduate to (a).

## Phase 4 — Tests

- [x] **4.1** End-to-end test with `httptest`: server returns connection refused, env vars simulated, assert no prompt + correct exit. (Already partly covered by 3.x unit tests.)
- [x] **4.2** Mock TTY behavior using `os.Pipe` to feed answers; verify retry happens on "Y".
- [x] **4.3** Verify existing `commands_test.go` tests still pass (no regression in `doRequest` happy path).

## Phase 5 — Validation ladder

- [x] **5.1** `CGO_ENABLED=0 go build ./...` → success
- [x] **5.2** `go test -race -short ./cmd/nano-brain/...` → all pass
- [x] **5.3** Manual smoke (recorded in `docs/evidence/connect-error-ux.md`):
  - `killall nano-brain; npx nano-brain init --root $(pwd)` → expect prompt
  - `NANO_BRAIN_NO_AUTO_START=1 ...` → expect no prompt
  - `echo "" | npx nano-brain ...` → expect no prompt

## Phase 6 — Docs

- [ ] **6.1** README: add a one-line note under troubleshooting linking to this behavior (optional — defer to follow-up if PR is at risk of growing).

## Phase 7 — PR

- [ ] **7.1** Open PR linking issue #141. Reference this OpenSpec change in the PR body.
- [ ] **7.2** Capture screencast/transcript of the new prompt flow in PR description (evidence per HARNESS.md).
