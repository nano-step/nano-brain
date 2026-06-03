## Why

End users of nano-brain experience a degraded UX caused by the default invocation pattern (`npx nano-brain`) and inflexible skill configuration. Each `npx` call carries 600ms–1.5s of cold-start overhead (registry HEAD check + Node.js VM + Go binary init), multiplied by 20–50 invocations per AI agent session — 15–75 seconds of wasted overhead per session. Three different binaries (npm cache, local build, bash shim) can coexist on a single machine with no diagnostic to surface which is running. The shipped skill hardcodes `host.docker.internal:3100` as the MCP URL, which silently fails for bare-metal users. The existing `doctor` command checks installation prerequisites but cannot diagnose runtime health (server reachability, embedding queue backpressure, CLI↔server version skew), and a `postinstall` silent exit-0 means binary download failures are invisible until first use. Documentation pushes the `npx` pattern in every example without surfacing the `npm install -g` fast path.

This overhaul makes the fast invocation path obvious, gives users self-service runtime diagnostics, and makes the skill MCP URL portable across Docker and bare-metal environments — without breaking existing Docker-based deployments.

## What Changes

- **NEW**: Pure-docs Phase 0 — reorder `SKILL.md` to recommend MCP / `npm install -g` over `npx`; audit `@beta` → `@latest` in `client_helpers.go`, README, SKILL.md, tests; document `NANO_BRAIN_SKIP_SHA_VERIFY`, `npm install -g --prefix ~/.local`, macOS Gatekeeper workaround
- **NEW**: Phase 0 zero-code investigation — does OpenCode support `${ENV_VAR}` substitution in `skill.json`? Outcome gates Phase 3 implementation
- **NEW**: `nano-brain doctor` gains a "binary exists at expected path" offline check (catches `postinstall.js` silent exit-0 footgun)
- **NEW**: `nano-brain doctor --online` opt-in flag adding runtime checks: server reachable, embedding queue health (WARN at 80% capacity, FAIL at 95%), CLI↔server version comparison, MCP endpoint reachable
- **NEW**: `nano-brain version --which` flag printing resolved binary path, version, and invocation source (npm-local / npm-global / dev-build / PATH)
- **NEW**: `NANO_BRAIN_BIN` env var to override binary resolution (validated: file exists + executable)
- **NEW**: `NANO_BRAIN_MCP_URL` env var read by nano-brain core; resolution order: env var → `/.dockerenv` exists ? `host.docker.internal:3100` : `localhost:3100`
- **NEW**: `nano-brain mcp-url` subcommand printing resolved MCP URL (skill installers can call this)
- **NEW**: Opt-in postinstall binary copy (Phase 4) — when `NANO_BRAIN_AUTO_LINK=1` env or `~/.nano-brain/auto-link` marker is present, `postinstall.js` copies (not symlinks) the downloaded Go binary to `~/.local/bin/nano-brain` (Linux) or `~/Library/nano-brain/bin/nano-brain` (macOS), detects existing files at target and skips with warning, prints PATH guidance, skips on Windows
- **MODIFIED**: `SKILL.md` shipped via skill-manager — section order, recommended commands, troubleshooting
- **CHANGED (deferred to Phase 3)**: `skill.json` MCP URL strategy depends on Phase 0 investigation outcome (env var templating, skill-manager postinstall templating, or dual skill.json variants). **Not a default change in Phase 0 — protects existing Docker users.**
- **OUT OF SCOPE (parallel issue)**: Embedding queue backpressure investigation, multi-workspace queue fairness, Ollama provider tuning
- **OUT OF SCOPE**: `nano-brain doctor --fix` auto-remediation, `nano-brain setup` rustup-style installer, `curl|sh` install script, per-platform optional npm dependencies, CLI write refusal under backpressure, automatic PATH modification

## Capabilities

### New Capabilities

- `cli-doctor-runtime`: Runtime health diagnostics for nano-brain CLI — adds `--online` mode to existing `doctor` command, new `binary exists` offline check, structured exit codes, JSON output for CI consumption. Covers server reachability, embedding queue depth thresholds (WARN ≥ 80%, FAIL ≥ 95%), CLI↔server version skew detection, MCP endpoint reachability.
- `cli-binary-resolution`: Deterministic binary path discovery and reporting — `nano-brain version --which` surfaces resolved binary, version, and invocation source; `NANO_BRAIN_BIN` env var allows explicit override with validation; resolution precedence is documented and testable.
- `cli-mcp-url-resolution`: Environment-aware MCP URL resolution — `NANO_BRAIN_MCP_URL` env var as primary; `/.dockerenv` container detection as fallback; `localhost:3100` as default. New `nano-brain mcp-url` subcommand for skill installers and diagnostic tooling. Pure Go stdlib, no probing.
- `cli-install-path-optimization`: Opt-in postinstall binary placement — `NANO_BRAIN_AUTO_LINK=1` or `~/.nano-brain/auto-link` marker triggers copy of Go binary to platform-appropriate PATH location. Copy (not symlink) for robustness. Detects existing files, never overwrites. Skips Windows. Prints PATH guidance without auto-modifying shell config.
- `skill-distribution-docs`: Skill documentation contract — `SKILL.md` shipped via skill-manager npm package must lead with MCP (zero overhead) and `npm install -g` (fast invocation), document `npx` as fallback only. Includes troubleshooting for `NANO_BRAIN_SKIP_SHA_VERIFY`, non-root install via `--prefix ~/.local`, macOS Gatekeeper `xattr` workaround.

### Modified Capabilities

(No existing spec requirements are changing. The existing `cli-reindex`, `cli-code-intelligence`, and `mcp-server` capabilities remain untouched at the spec level; this change is additive.)

## Impact

**Affected code:**
- `cmd/nano-brain/doctor.go` — extend with `--online` flag and `binary exists` check
- `internal/health/doctor/doctor.go` — add new check functions (server-running, queue-health, version-skew, mcp-reachable, binary-exists)
- `cmd/nano-brain/version.go` (or wherever `version` is registered — verify) — add `--which` flag
- `cmd/nano-brain/main.go` — register new `mcp-url` subcommand
- `cmd/nano-brain/client_helpers.go` — update `suggestStartCommand()` to drop `@beta`, become invocation-aware
- `cmd/nano-brain/client_helpers_test.go` (and any `commands_test.go` referencing `@beta`) — update expected strings
- `internal/server/handlers/` — verify `/api/status` exposes everything needed by `--online` doctor (queue_pending, queue_capacity, version); if missing, add fields
- `npm/postinstall.js` — add opt-in copy-to-PATH logic (Phase 4 only)
- `npm/run.js` — no changes; left as-is for `npx` fallback
- `README.md` — update Quick Start, remove `@beta` references
- `SKILL.md` (both project-local `.opencode/skills/nano-brain/SKILL.md` and the version shipped via skill-manager npm package) — reorder sections, update commands, add troubleshooting

**Affected APIs:**
- New HTTP fields possibly added to `/api/status` response if not already present (`version`, `queue_pending`, `queue_capacity`) — additive only, no breaking change
- New CLI subcommand `mcp-url` and new flag `version --which`, `doctor --online` — additive only

**Affected dependencies:**
- No new Go dependencies (all logic uses stdlib + existing koanf, zerolog, net/http)
- No new npm dependencies (postinstall.js stays Node stdlib)
- Cross-repo coordination: SKILL.md changes ship via skill-manager npm publish cycle (user-owned per Q5 confirmation)

**Affected systems:**
- Phase 3 skill.json change is deferred and gated on OpenCode templating investigation; no breaking change to existing Docker users
- Phase 4 binary copy is strictly opt-in; default `npm install` behavior unchanged

**Out-of-scope but tracked separately:**
- Embedding queue backpressure investigation (parallel GitHub issue, separate workstream)
- Multi-workspace queue fairness (deferred; tracked as known limitation)
- Skill-manager publish coordination is user-owned per Q5

**Risks tracked (see design.md for mitigations):**
- skill-manager postinstall infrastructure unverified → Phase 0 investigation gates Phase 3 scope
- Cross-platform testing burden (macOS arm64/amd64, Linux glibc/musl, Docker Alpine, behind corp proxy) explicitly budgeted in Phase 4 (2–3 days)
- Existing user shims at `~/.local/bin/nano-brain` → Phase 4 detects and skips, never overwrites
