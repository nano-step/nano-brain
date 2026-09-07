## Why

Tracking: #615

This change completes the install-UX workstream (#601): the one-line installers
(curl|bash, #533; opencode init target, #603) and zero-config init (#534) get the
binary onto the machine, but a freshly installed daemon still dies silently and
stops harvesting. The existing detached process commands do not register the
daemon with an operating-system supervisor, and startup exits when PostgreSQL is
temporarily unavailable. Delivering `service install|uninstall|status|restart|update`
closes the install story: after install, the daemon is supervised by launchd or
systemd user services with native autostart, restart, and upgrade paths.

## What Changes

- Add a `nano-brain service install|uninstall|status|restart|update` CLI surface.
- Register a foreground `serve` process with macOS launchd (LaunchAgent) or
  Linux systemd user services, with native automatic start and restart behavior.
- Make service definitions idempotent, atomically written, and safe to remove;
  preserve the existing PID-file daemon commands as a separate legacy path.
- Report service registration, supervisor process state, and `/health` readiness
  together, with a defined JSON schema and exit-code matrix for automation.
- Keep service launch paths stable across npm upgrades: a validated global npm
  invocation records the stable `node` + `npm/run.js` pair, while direct binary
  and validated `NANO_BRAIN_BIN` invocations record an absolute executable.
  Ephemeral npx/local launchers are rejected for persistent registration.
- Make the npm wrapper wait for and forward termination signals to its Go child,
  so a managed foreground service cannot leave an orphaned child process.
- Retry connection-refused, timeout, DNS, and other transient PostgreSQL startup
  failures with a 1-second exponential backoff capped at 30 seconds until the
  startup context is cancelled; malformed configuration fails immediately.
- Make CLI connection recovery and interactive initialization service-aware so a
  registered service is started/restarted through its supervisor rather than
  spawning a competing PID-file daemon.
- Pin an absolute config-file path in the service command. Shell environment
  overrides are not copied into service definitions; credentials remain in the
  config/secret mechanism and are never serialized into plist/unit files.
- Reject service installation on unsupported platforms, in containers, or as
  root, with actionable errors; Windows service integration is out of scope for
  v1.
- Document one-line setup for macOS and Linux, including Linux
  `loginctl enable-linger` for boot-time startup.

## Capabilities

### New Capabilities

- `managed-daemon-service`: Native OS registration and lifecycle commands for
  the long-lived foreground daemon, including status and upgrade refresh.
- `service-resilient-startup`: Supervisor-compatible foreground startup with
  PostgreSQL retry/backoff and stable launcher resolution.

### Modified Capabilities

- None.

The archived `daemon-management` behavior remains the compatibility baseline:
`serve -d`, `stop`, and `restart` continue to own only the legacy PID-file
process. The existing `cli-binary-resolution` requirements and `NANO_BRAIN_BIN`
precedence remain unchanged; service launcher metadata is an additive npm-wrapper
handoff. The new service-aware recovery path is specified by the new capability
below rather than changing those archived requirements.

## Impact

- CLI dispatch, service-manager adapters, status probing, and recovery/setup
  consumers under `cmd/nano-brain/`.
- PostgreSQL startup retry behavior under `internal/storage/` and the existing
  `startServer` path, without changing fail-fast one-shot pool callers.
- npm wrapper launcher metadata and signal lifecycle in `npm/run.js`.
- User documentation in `README.md` and `docs/SETUP_AGENT.md`, plus generated
  platform-specific service definitions.
- No REST/MCP contract, database migration, or persistent data-model change.

## Review Debate Outcome

| Finding | Decision | Resolution |
|---|---|---|
| Missing executable tasks and platform QA | Accepted | `tasks.md` will contain dependency-ordered implementation and macOS/Linux/unsupported-platform verification tasks. |
| Missing status schema and exit codes | Accepted | `managed-daemon-service` will define JSON fields, partial states, and exit codes 0–3. |
| npm wrapper process ownership and update path | Amended | `run.js` will forward signals and expose validated global-launcher metadata; service templates will use an argv-safe stable launcher and manager reload sequence. |
| Retry classification and ownership | Accepted | A start-server-only retry API will leave `NewPool` fail-fast for one-shot callers and define cancellation/leak tests. |
| Legacy daemon collision and recovery consumers | Accepted | Registered-service detection will route recovery through the native manager; legacy commands remain unchanged when no service is registered. |
| Security, config, rollback, and manager transitions | Accepted | Design/specs will require absolute validated paths, restrictive atomic writes, no credential serialization, explicit manager operations, and rollback/error reporting. |
| Add modified `daemon-management`/`cli-binary-resolution` specs | Rejected with rationale | The archived daemon contract and current binary-resolution requirements remain unchanged; additive service behavior is covered by the two new capabilities, with `NANO_BRAIN_BIN` explicitly preserved. |
