## 1. Contracts and seams

- [ ] 1.1 Add service command parsing, operation/result types, status states, and exit-code constants in `cmd/nano-brain/`; verify valid/invalid subcommands and `--json` handling without starting a manager.
- [ ] 1.2 Define an injectable service-manager command runner and platform adapter interface; verify unit tests can exercise manager failures without invoking launchd/systemd.
- [ ] 1.3 Add generic fixture paths and fake manager outputs for active, inactive, failed, unregistered, unauthorized, and unavailable states; verify fixtures contain no real workspace names, paths, hashes, or credentials.

## 2. Launcher and configuration resolution

- [ ] 2.1 Extend `npm/run.js` to wait for the Go child, forward SIGTERM/SIGINT, propagate exit status/signals, and preserve existing `NANO_BRAIN_BIN` and lazy-download behavior; verify with Node subprocess tests including crash and termination cleanup.
- [ ] 2.2 Add npm wrapper metadata for absolute `run.js`, Node executable, and global-install provenance; verify global paths are accepted and local/npx/ephemeral paths are rejected for persistent service installation.
- [ ] 2.3 Add service launcher resolution in `cmd/nano-brain/` with precedence for validated `NANO_BRAIN_BIN`, global npm wrapper metadata, and direct `os.Executable()`; verify regular-file/executable checks and actionable errors.
- [ ] 2.4 Add an absolute installed-config resolver that records the selected `--config`/default path and a file-only status load path; verify service startup/status are independent of working directory and transient shell config overrides.
- [ ] 2.5 Preserve the existing `cli-binary-resolution` `NANO_BRAIN_BIN` contract and add focused regression tests proving ordinary npm/direct invocations remain unchanged.

## 3. Native definition rendering and safe writes

- [ ] 3.1 Implement launchd plist rendering with label, foreground `serve`, `RunAtLoad`, `KeepAlive`, `ThrottleInterval`, config arguments, and user log paths; verify XML parsing, exact argv, and path escaping.
- [ ] 3.2 Implement systemd user-unit rendering with foreground `serve`, `Restart=always`, `RestartSec=2`, `WantedBy=default.target`, journal output, and safe `ExecStart` escaping; verify unit content with paths containing spaces and metacharacters.
- [ ] 3.3 Implement user-only parent/file permissions, same-directory temporary files, atomic rename, canonical path validation, and safe removal of only the owned fixed definition; verify interrupted writes preserve the prior definition and no secret value is serialized.

## 4. Platform manager lifecycle

- [ ] 4.1 Implement the macOS adapter using the per-user `gui/<uid>` domain and explicit bootout/bootstrap/kickstart or equivalent reload sequence; verify install, update, restart, status, and uninstall with a fake `launchctl` runner.
- [ ] 4.2 Implement the Linux adapter using `systemctl --user daemon-reload`, enable/start/restart/stop/disable, and active/enabled state inspection; verify install, update, restart, status, and uninstall with a fake `systemctl` runner.
- [ ] 4.3 Implement transactional install/update rollback when definition replacement or manager registration fails; verify the CLI reports the failed operation, restores the prior file when safe, and never claims success.
- [ ] 4.4 Implement idempotent `service install`, `service update`, `service restart`, and `service uninstall` state transitions; verify repeated install, update-when-absent, restart-when-absent, and uninstall-when-absent behavior.
- [ ] 4.5 Detect and reject root, container, unsupported GOOS, missing user manager, and live legacy PID-daemon collision; verify code `3` for unsupported/unavailable environments and code `1` for collision/manager operation failure.

## 5. Status and recovery integration

- [ ] 5.1 Implement direct configured `/health` probing with a bounded timeout and no `doRequest` auto-start side effects; verify healthy, unreachable, delayed-start, malformed response, and HTTP 401 states.
- [ ] 5.2 Implement human and `--json` status output with the specified fields, partial-state preservation, version extraction, and exit codes `0`–`3`; verify the complete status matrix.
- [ ] 5.3 Update `cmd/nano-brain/client.go`, `client_helpers.go`, `init_serve.go`, and platform launch hooks so registered services are started/restarted through the native manager and unregistered flows retain legacy `serve -d` behavior.
- [ ] 5.4 Verify recovery does not create a second process while the managed service is waiting for PostgreSQL, and verify `service install` refuses a live legacy daemon without deleting its PID file or definition.

## 6. PostgreSQL-resilient server startup

- [ ] 6.1 Add a cancellation-aware server-start pool retry helper under `internal/storage/` that leaves `NewPool` fail-fast, closes failed attempts, retries at 1s/2s/4s capped at 30s, and avoids credential logging.
- [ ] 6.2 Integrate the retry helper only into `startServer`; preserve immediate invalid-config failure and run migrations only after a successful pool ping.
- [ ] 6.3 Add unit tests for malformed DSN, connection-refused retry success, backoff cap, cancellation during wait, pool cleanup, and one-shot `NewPool` fail-fast behavior.
- [ ] 6.4 Add a server-start integration test that delays PostgreSQL availability on the isolated test database/port, then verifies `/health` becomes ready without a supervisor restart.

## 7. Documentation and migration guidance

- [ ] 7.1 Document `service install/status/restart/update/uninstall` and the foreground/native-manager model in `README.md` and `docs/SETUP_AGENT.md`.
- [ ] 7.2 Document macOS LaunchAgent and Linux systemd-user setup, the `loginctl enable-linger "$USER"` boot requirement, manager-unavailable remediation, and unsupported Windows/container/root scope.
- [ ] 7.3 Document global npm versus direct binary launcher requirements, npm upgrade/update flow, `NANO_BRAIN_BIN`, config-file persistence, and migration from an existing `serve -d` PID daemon.
- [ ] 7.4 Add user-facing log/status guidance distinguishing native manager logs, configured application logs, and legacy `~/.nano-brain` logs.

## 8. Verification and evidence

- [ ] 8.1 Run `gofmt` on changed Go files, `node --test npm/postinstall.test.js` plus new wrapper tests, and `CGO_ENABLED=0 go build ./...`.
- [ ] 8.2 Run `go test -race -short ./...` and record the real output in `docs/evidence/cross-platform-daemon-auto-start/`.
- [ ] 8.3 Run integration tests against `nanobrain_test` only, including the delayed-PostgreSQL startup case, and record the real output.
- [ ] 8.4 On macOS, run a native smoke test using an isolated config/home fixture: install, inspect plist, status, terminate the foreground service, verify restart, update, and uninstall.
- [ ] 8.5 On Linux with a usable user manager, run the corresponding systemd-user smoke test; otherwise record the explicit manager-unavailable result and retain fake-runner coverage.
- [ ] 8.6 Run `GOOS=windows CGO_ENABLED=0 go build ./...` and verify the unsupported service command compiles and returns its stable error path.
- [ ] 8.7 Run `openspec validate cross-platform-daemon-auto-start --strict`, perform the independent implementation review, and archive only after all required evidence is present.
