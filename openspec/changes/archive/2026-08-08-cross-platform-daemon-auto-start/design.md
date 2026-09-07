## Context

The current CLI starts the server in the foreground or uses `daemon.go` to
self-spawn a detached `--daemon-child`, write
`~/.nano-brain/nano-brain.pid`, and implement `stop`/`restart`. That path is
deliberately retained for backward compatibility. It is not an OS registration:
it cannot start after login/boot and a dead process is only discovered when a
client notices a connection failure.

`startServer` currently calls `storage.NewPool`, which creates and pings a
PostgreSQL pool once. A failed ping exits before HTTP health can be served. A
managed service needs the process to remain alive while PostgreSQL becomes
available, but one-shot commands and integration helpers must keep their current
fail-fast `NewPool` behavior.

The npm package exposes `npm/run.js`, which currently synchronously launches the
downloaded Go binary. A persistent service must use a stable global npm wrapper
or a validated direct executable, preserve the configured absolute config path,
and forward termination signals without leaving a child process behind.

The service implementation is a CLI concern. It has no REST/MCP or database
schema surface. Its manager commands are injected behind a platform adapter so
template, state, and failure behavior can be tested without mutating a host
service manager.

## Goals / Non-Goals

**Goals:**

- Install, remove, start, restart, refresh, and inspect one per-user service on
  macOS launchd or Linux systemd user services.
- Keep definitions and launch arguments deterministic, absolute, argv-safe,
  restrictive, and atomically replaced.
- Make `service status` useful to both humans and scripts by separating
  definition registration, supervisor state, HTTP reachability, and readiness.
- Keep the service process in foreground `serve` mode so the native supervisor
  owns lifecycle and restart policy.
- Retry only server-start PostgreSQL availability failures with cancellation-aware
  bounded backoff.
- Prevent the existing interactive/client recovery flows from launching a
  competing PID daemon when a managed service is registered.

**Non-Goals:**

- Windows Task Scheduler/service support, BSD service managers, system-wide
  root services, container-managed services, or Docker Compose orchestration.
- Replacing or changing the existing `serve -d`, `stop`, or `restart` PID-file
  contract when no managed service is registered.
- Snapshotting shell environment variables or credentials into service files.
- Database migrations, API response changes, or a new persistent service record.
- Automatic invocation of `loginctl enable-linger`; the user must opt into that
  operating-system policy.

## Decisions

### One service abstraction with platform adapters

`service` dispatch parses `install`, `uninstall`, `status`, `restart`, and
`update` in `cmd/nano-brain`. A small adapter interface owns platform-specific
paths, definition rendering, manager commands, and state inspection. Darwin
uses label `com.nano-step.nano-brain` and
`~/Library/LaunchAgents/com.nano-step.nano-brain.plist`. Linux uses
`~/.config/systemd/user/nano-brain.service` and `systemctl --user`.

The common layer owns state transitions, config/launcher resolution, atomic
writes, status JSON, exit codes, and recovery routing. Build-tagged adapters or
an equivalent compile-safe platform split return a stable unsupported error for
other GOOS values.

### Native definitions own the foreground process

The generated command always runs `serve` without `-d`. On macOS the plist has
`RunAtLoad=true`, `KeepAlive=true`, `ThrottleInterval=5`, and explicit
`StandardOutPath`/`StandardErrorPath` under `~/.nano-brain/logs/`. On Linux the
unit has `Restart=always`, `RestartSec=2`, and `[Install] WantedBy=default.target`,
with stdout/stderr directed to the journal. The service never writes the legacy
PID file.

Install writes a restrictive temporary file beside the target and renames it
over the target only after the complete definition is rendered. It then applies
the manager transition: launchd bootout (best effort), bootstrap, and kickstart;
systemd daemon-reload, enable, and start. Update follows the same sequence and
restarts the registered service. Uninstall stops/disables or boots out the
service before removing only the fixed definition path. A manager failure is
reported and the previous definition is restored when it was replaced and can
be restored safely.

### Stable launcher provenance

The Go CLI resolves the service command using this precedence:

1. A validated non-empty `NANO_BRAIN_BIN` is installed as a direct executable.
2. The npm wrapper supplies absolute `run.js`, `process.execPath`, and a
   `global` stability marker; this pair is used only when the package is from a
   global npm root.
3. A direct `os.Executable()` path is canonicalized, checked as a regular
   executable, and used for direct binary installations.

The npm wrapper rejects no normal invocation, but `service install` rejects
local/npx/ephemeral wrapper provenance with an actionable instruction to use a
global install or a direct binary. Service definitions persist the absolute
config path and launcher arguments only. They do not copy `DATABASE_URL`, auth
tokens, API keys, or arbitrary shell environment. A custom binary override is
represented by its validated direct path rather than serialized as an env var.

`npm/run.js` uses a waiting child-process wrapper for all invocations that need
it: it forwards SIGTERM/SIGINT to the Go child, waits for child exit, and exits
with the child status or signal. This keeps `node run.js serve` foreground and
prevents an orphan when the manager stops the wrapper. Existing `NANO_BRAIN_BIN`
validation and normal command behavior remain unchanged.

### Config and endpoint resolution

`service install` resolves `--config`/`NANO_BRAIN_CONFIG`/default to an absolute
path and stores that path before rendering. Service startup receives the path
before the `serve` command (`--config <absolute> serve`) so it is independent of
the manager's working directory. Service status loads that same file without
triggering client auto-start recovery and probes the configured host/port
directly at `/health` with a short timeout. It reports HTTP reachability even
when `/health` is unauthorized, but marks readiness unknown/false and never
stores credentials to bypass auth. The default `/health` bypass remains the
recommended local probe configuration.

When `serve -d` or a service is registered, client connection recovery and the
interactive init serve step first inspect the managed-service marker. A
registered service is started/restarted through its adapter and then polled for
readiness. Only when no managed service definition exists does the old recovery
path invoke `serve -d`.

### Status contract

`service status` supports `--json` and returns a single object with:

```json
{
  "platform": "darwin|linux|unsupported",
  "registered": true,
  "supervisor_state": "active|inactive|failed|unknown|unsupported",
  "health_reachable": true,
  "ready": true,
  "endpoint": "http://localhost:3100/health",
  "version": "...",
  "error": ""
}
```

Fields remain present with false/empty values in partial states. Human output
uses the same vocabulary. Exit codes are: `0` registered + active + reachable
and ready; `1` registered but inactive, unreachable, unauthorized, degraded, or
not ready; `2` definition not registered; `3` unsupported platform or unavailable
user manager. Lifecycle command failures use exit code `1` and include the
manager operation and remediation without leaking credentials.

### Startup retry ownership

Add a cancellation-aware `storage.NewPoolWithRetry` (or equivalently named
server-start-only helper) that preserves `NewPool` as a single-attempt API.
Configuration parsing/validation errors fail immediately. A failed connection or
ping closes that attempt's pool, logs a warning, waits 1s, 2s, 4s … capped at
30s, and retries until success or context cancellation. The wait uses a timer
select on `ctx.Done()` and leaves no pool behind on cancellation. Retry logging
is rate-limited to attempts/backoff, not every low-level connection event. The
startup path uses the helper; migrations still run after a successful pool ping.
Permanent migration/configuration failures remain visible to the supervisor and
are subject to its native throttling rather than being hidden as readiness.

## Risks / Trade-offs

- **Supervisor restart storm on invalid config** → Native definitions use
  launchd throttling and systemd `RestartSec`; status exposes failed/starting,
  logs retain the configuration error, and install validates the selected
  config before enabling the service.
- **A legacy PID daemon may already be running** → `service install` probes the
  existing PID file and refuses to register a second process until the user
  stops it; service-aware recovery never starts the legacy path while the fixed
  definition exists. Uninstall does not silently kill an unrelated PID.
- **A configured `/health` bypass may be removed** → Status reports the manager
  process as active and health as unauthorized/not ready, with an explicit
  remediation; it never guesses credentials.
- **Global npm paths vary by package manager** → The wrapper supplies a resolved
  global-root marker and Node path; non-global paths fail with a migration
  message instead of creating a service pinned to an ephemeral cache.
- **Environment-only database settings disappear at boot** → The service pins
  the config file and documentation requires service users to place durable
  non-default settings in that file; no secret-bearing environment snapshot is
  created.
- **Manager APIs differ across OS versions/distributions** → All external
  commands are argv arrays behind an injectable runner; errors include captured
  stderr and the exact remediation. Native smoke tests are conditional on the
  host manager being available.
- **Retry delays postpone visible HTTP readiness** → Status distinguishes an
  active service from a ready service, and retry logs explain that PostgreSQL is
  not yet reachable. Cancellation stops retries promptly.
- **Generated files contain user paths** → Local service definitions are expected
  to contain absolute paths, but tests and committed docs use generic fixtures;
  no real home paths, workspace names, hashes, or credentials are committed.

## Migration Plan

1. Release the CLI and npm wrapper with service support; existing PID-file
   commands and configurations remain valid.
2. Users run `nano-brain service install` once. The command validates the config
   and launcher, writes the per-user definition, registers it, and starts it.
3. On Linux, users who require boot-time startup run
   `loginctl enable-linger "$USER"`; otherwise the user service starts when the
   user manager is created at login.
4. During an npm upgrade, the stable global `run.js` path remains in place;
   `nano-brain service update` rewrites the definition from the current wrapper
   metadata and restarts the service. A plain `service restart` uses the already
   registered definition.
5. Rollback is uninstalling the managed definition and returning to
   `nano-brain serve -d`; the old PID commands are not removed. If an update
   manager transition fails, the previous definition is restored and the
   service is left stopped with an actionable error.

## Open Questions

None blocking. Windows and system-wide service support are intentionally
follow-up work rather than unresolved v1 decisions.
