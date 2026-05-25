# Connect-Error UX

## Problem

When the nano-brain server is not running, every CLI command that talks to it (`init --root`, `write`, `query`, `search`, `vsearch`, `harvest`, …) fails with a single dead-end line:

```
Error: cannot connect to nano-brain server at localhost:3100
```

This is the **#1 first-run friction point** today. The user just followed the interactive `nano-brain init` flow, which literally prints `To register this workspace, start the server and run: nano-brain init --root <path>`. They run that line, hit this error, and have no idea which command starts the server. The error gives them nowhere to go.

## Solution

Centralize the fix in the HTTP client (`cmd/nano-brain/client.go`) so every CLI command benefits. When connection is refused:

1. **Print a clear, structured error** explaining the server is down, with the exact resolved host:port that was tried.
2. **Suggest the exact start command**, auto-detected from launch context:
   - launched via npx → `npx @nano-step/nano-brain@beta serve -d`
   - launched as binary → `nano-brain serve -d`
3. **If stdin + stderr are both TTYs** (interactive user): prompt `Start server now? [Y/n]`.
   - On `Y`/empty → fork `serve -d` via existing `runServeDaemon`, poll `/api/status` until healthy (max 10s), then **retry the original request once**.
   - On `n` or invalid → exit 1 with the suggestion still visible.
4. **If non-TTY** (CI, OmO agent, piped script): skip the prompt entirely. Print suggestion + exit 1. **Never auto-start in non-interactive mode** — too surprising.

`NANO_BRAIN_NO_AUTO_START=1` env var bypasses the prompt even in TTY mode (escape hatch for scripted interactive sessions).

## Scope

- **In scope**: `cmd/nano-brain/client.go` (error formatter + TTY detection + auto-start retry), `cmd/nano-brain/main.go` (no changes — guard already exists), tests in `cmd/nano-brain/client_test.go`.
- **Out of scope**:
  - Auto-start via `docker start` (different decision tree — Postgres lifecycle, port conflicts). This proposal uses `serve -d` only.
  - Logging — covered by separate proposal (issue #144).
  - Windows daemon support — `daemon.go` is Unix-only by build tag; same constraint applies.

## Risk Classification

- Multi-file change → 1 flag
- Behavior change to ALL CLI commands talking to server → 1 flag
- Stdin TTY prompt — new interactive flow → 1 flag

**Total: 3 flags → lane:normal (proposal required, as written).**

No DB schema change, no API contract change, no external provider, no security-sensitive code.

## References

- Issue #141
- Related: #131 (enhanced-init originally promised "offer init --root if server running")
- Reuses: `runServeDaemon` from `cmd/nano-brain/daemon.go` (lines 60+)
- Reuses: `getBaseURL` / `resolveHostPort` from `cmd/nano-brain/client.go`
