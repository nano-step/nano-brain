# Validation evidence — cross-platform daemon auto-start (#615)

Scope merged with the install-UX workstream (#601): the one-line installers
(#533 curl|bash, #603 opencode init, #534 zero-config init) get the binary
onto the machine; this change registers the daemon with the OS supervisor
(launchd/systemd user service) with native autostart, restart, and update.

## 1. Unit tests (all pass, `-race -short`)

```
go test -race -short ./...           → exit 0 (full suite)
node --test npm/run.test.js npm/postinstall.test.js → 30 pass / 0 fail
```

Coverage highlights:

| Area | Tests |
|---|---|
| Contracts | `parseServiceArgs` (valid/invalid subcommands, `--json`, no manager), `statusExitCode` matrix (0/1/2), unsupported platform exit-3 methods |
| Rendering | plist XML validity + escaping + foreground-only; systemd unit ExecStart quoting + `%`→`%%` + no `Environment=`; atomic same-dir write with 0700 parents, no leftover temp files |
| Platform adapters | launchd bootout→bootstrap→kickstart sequence + bootstrap failure propagation; `launchctl print` state matrix (active/inactive/unregistered/unknown); systemd daemon-reload→enable→start, is-active matrix; absent-unit idempotent unregister |
| Launcher resolution | NANO_BRAIN_BIN precedence + validation, global npm wrapper pair, npx rejection, direct-binary fallback, absolute config resolution |
| Lifecycle | installDefinition write + rollback (restore-previous / remove-new on manager failure); uninstall removes definition only after unregister |
| Health | /health ready/degraded/401/unreachable probe; status JSON round-trip |
| Recovery | managed-vs-legacy matrix for `startManagedServiceIfRegistered` and `recoverFromConnectionRefused` (legacy never launched when a managed definition exists and fails) |
| npm wrapper | exit-status propagation, spawn-failure message, SIGTERM forwarding to the child (node-child signal test), launcher metadata env vars |

## 2. Integration — PostgreSQL-resilient startup (task 6.4)

`internal/storage/pool_retry_integration_test.go` delays PG availability behind
a TCP proxy that refuses connections for 3s, then verifies:

- fail-fast `NewPool` errors while PG is unreachable (single attempt);
- `NewPoolWithRetry` reconnects after PG returns without any restart, and the
  pool pings cleanly.

```
NANO_BRAIN_TEST_DATABASE_URL="postgres://nanobrain:nanobrain@localhost:5432/nanobrain_test?sslmode=disable" \
  go test -race -tags=integration -run TestNewPoolWithRetrySurvivesPostgreSQLDelay ./internal/storage/
→ ok (4.7s)
```

Ran against `nanobrain_test` only; dev DB untouched.

## 3. macOS native smoke test (task 8.4)

Real launchd registration with an isolated HOME + config (port 3199, the test
port; the dev server stays untouched on :3100). Full transcript:
`docs/evidence/cross-platform-daemon-auto-start/smoke-macos.md`.

Results (all green):

1. `service install` → plist written at isolated path, `plutil -lint` OK
2. `launchctl print` → job registered, `state = running`, `state = active`
3. `/health` ready in 4s (`{"status":"ok","ready":true,...}`)
4. `service status --json` → `registered:true, supervisor_state:active,
   health_reachable:true, ready:true, endpoint:http://localhost:3199/health`
5. `kill -9` the foreground serve process → launchd `KeepAlive` restarted it;
   health returned (no supervisor restart, no manual action)
6. `service update` → definition rewritten + re-registered (launchd
   bootout→bootstrap race fixed with a bounded retry: bootstrap immediately
   after bootout fails with "Input/output error" while the job is SIGTERMed)
7. `service uninstall` → booted out, definition removed

Bugs found and fixed during the smoke test:

- **guardBeforeStart probed only the env/default port** (:3100), so a managed
  service on a non-default port was blocked by an unrelated server on the
  default port and blind to duplicates on its own port. `startServer` now
  loads config first and guards on the configured port
  (`guardBeforeStartPort`).
- **launchd bootstrap race**: `launchctl bootstrap` immediately after
  `bootout` fails with "Bootstrap failed: 5: Input/output error" while the
  old job is still `SIGTERMed`. `register` retries bootstrap with a 700ms
  bounded backoff (3 attempts, context-aware).

## 4. Windows unsupported-path build (task 8.6)

```
GOOS=windows CGO_ENABLED=0 go build ./cmd/nano-brain
→ fails with 5 pre-existing undefined symbols (runServeDaemon, pidFilePath,
  runServeCmd, runStopCmd, runRestartCmd) in the legacy daemon.go path
```

The identical failure occurs on `origin/master` — the package's Windows build
was already broken before this change and is out of scope. The service
subsystem adds **zero new Windows errors**: every `service*` file is tag-free
with a runtime GOOS split, and the legacy-PID check is isolated behind the
build-tagged `legacyDaemonRunning` helper. On Windows the `service` commands
compile cleanly and return the actionable unsupported error (exit 3) without
writing files.

## 5. Fixture hygiene

All tests use generic placeholders (`com.nano-step.nano-brain`,
`/home/user/...`); no real workspace names, paths, hashes, or credentials in
committed fixtures (task 1.3).
