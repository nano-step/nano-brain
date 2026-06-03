# Design — nano-brain CLI UX Overhaul

## Context

### Current state

nano-brain ships three coexisting invocation paths that all funnel into the same Go binary:

1. `npx @nano-step/nano-brain[@tag]` — primary documented path in `README.md` and `SKILL.md`. Downloads (or resolves from npm cache) a Go binary via `npm/postinstall.js`, then `npm/run.js` re-execs it.
2. `npm install -g @nano-step/nano-brain` then `nano-brain ...` — fast path. Mentioned only in passing.
3. `CGO_ENABLED=0 go build -o nano-brain ./cmd/nano-brain` — dev / source build. Documented in `README.md` Option B.

The `npx` path carries 600 ms–1.5 s of cold-start overhead per invocation (npm registry HEAD check + Node.js VM init + Go binary process spawn). An AI agent session that issues 20–50 `nano-brain query` / `nano-brain context` calls pays 15–75 s of cumulative overhead. The fast paths (#2 and #3) eliminate the Node.js layer entirely.

### Existing diagnostics

- `nano-brain doctor` (`cmd/nano-brain/doctor.go`, `internal/health/doctor/doctor.go`) runs five offline-ish prerequisite checks: config, PostgreSQL reachability, pgvector extension, embedding provider (Ollama HTTP probe or Voyage key presence), embedding model. **All checks are local-config-driven.** None of them ask the running server how it's doing.
- `/api/status` (`internal/server/handlers/health.go:115-194`) already exposes `pg_status`, `migration_version`, `embedding_queue_depth`, `active_provider`, `workspace_count`, `queue_depth`, `queue_capacity`, `queue_status`, `queue_pending`, and harvester status. It does **not** expose `version` (the version string is on `/health` but not `/api/status`).
- The npm postinstall (`npm/postinstall.js`) downloads the platform binary, verifies SHA-256, places it under `npm/bin/<platform>/nano-brain`. On failure it WARNs and exits 0 (silent failure footgun) — see issue #320 history.

### Existing hardcoded MCP URL

`.opencode/skills/nano-brain/skill.json:11` hardcodes `"url": "http://host.docker.internal:3100/mcp"`. This is correct for OpenCode running inside a container (which is the dominant deployment), wrong for bare-metal users (where the agent IS the host and needs `localhost:3100/mcp`).

### `@beta` references still live

`cmd/nano-brain/client_helpers.go:69` returns `"npx @nano-step/nano-brain@beta serve -d"` as the suggested start command. `README.md:47-53` shows three `@beta` examples. `SKILL.md` already uses `@latest` (line 204). This is an inconsistency to clean up.

### Constraints

- **Single static Go binary** (`CGO_ENABLED=0`). No new Go dependencies. All new logic must use stdlib + existing koanf, zerolog, net/http, pgx/v5.
- **Custom CLI dispatcher** (`cmd/nano-brain/main.go` switch on `os.Args[1]`). No cobra. New subcommands are added by appending a `case "name":` arm.
- **Cross-platform shipping**: Linux amd64/arm64, macOS amd64/arm64, plus a "not supported" branch for Windows in postinstall.
- **Cross-repo coordination**: `SKILL.md` ships via the `@nano-step/skill-manager` npm package as well as via this repo's `.opencode/skills/nano-brain/SKILL.md`. The two MUST stay in sync; the skill-manager publish cycle is user-owned (per Q5 confirmation in proposal).
- **No breaking change to existing Docker users.** `host.docker.internal:3100` must remain a working URL out of the box.

### Stakeholders

- AI agents running inside OpenCode containers (today's primary user) — Docker MCP URL must keep working.
- Bare-metal users (today's silently-broken case) — `localhost` MCP URL must become available without manual config.
- Skill-manager publisher (the user) — controls the SKILL.md publish cycle.

## Goals / Non-Goals

**Goals:**

1. Make the fast invocation path (`npm install -g` or pre-built binary) obvious in docs — `npx` documented as fallback only.
2. Give users a one-command runtime health diagnostic (`nano-brain doctor --online`) covering: server reachable, embed queue not backpressured, CLI ↔ server version match, MCP endpoint reachable.
3. Make MCP URL portable across Docker and bare-metal without manual config: env var override + container auto-detection.
4. Surface which binary is actually running (`nano-brain version --which`) so users can diagnose "wrong version" footguns.
5. Give power users an opt-in fast PATH placement after `npm install` (no PATH auto-modification, never overwrite existing files).
6. Eliminate `@beta` references from user-facing surfaces. Default to `@latest`.

**Non-Goals:**

- `nano-brain doctor --fix` auto-remediation (out of scope per proposal).
- `curl | sh` install script or rustup-style installer (out of scope).
- Per-platform optional npm dependencies (out of scope — postinstall continues to dynamically download).
- Modifying the user's shell rc files (`.zshrc`, `.bashrc`). PATH guidance is printed, never injected.
- Embedding queue backpressure investigation / fix (parallel workstream, separate GitHub issue).
- Forcing existing Docker users onto `localhost` URL. Container detection must remain backward-compatible.

## Decisions

### D1 — Phased rollout: docs first, code second, packaging third

**Decision:** Phase 0 (docs + investigation) and Phase 1 (CLI additions) ship independently. Phase 3 (skill.json change) and Phase 4 (opt-in postinstall copy) are deferred and gated on the Phase 0 investigation outcome.

**Why:** Phase 0 is reversible (`git revert`), zero-risk, immediately useful (clears `@beta` confusion). Phase 1 adds new flags / subcommands — additive only, no behaviour change to existing commands. Phase 3 changes a file that ships cross-repo (skill-manager), so we want full design review with the templating-feasibility answer in hand. Phase 4 touches install-time behaviour on three platforms — needs explicit testing.

**Alternative considered:** Ship everything in one PR. Rejected — too large, hard to revert, mixes pure-docs (low risk) with cross-platform postinstall (high risk).

### D2 — `nano-brain doctor --online` is opt-in, not default

**Decision:** Existing `nano-brain doctor` keeps current behaviour (offline prerequisite checks). New `--online` flag adds runtime checks against a running server.

**Why:**
- Today's `doctor` is run BEFORE the server is started (its job is to validate prerequisites). Adding online checks by default would make `doctor` fail in the most common "I'm running this because nothing works yet" case.
- `--online` gives ops a single command to ask "is my running server healthy", which is a different question.
- JSON output (`--json`) enables CI consumption.

**Alternative considered:** Split into `nano-brain doctor` (prereq) and `nano-brain healthcheck` (runtime). Rejected — adds a new top-level subcommand for a small surface area. The flag pattern matches `--json` precedent on the same command.

### D3 — Embed queue health thresholds: WARN ≥ 80 %, FAIL ≥ 95 % of `queue_capacity`

**Decision:** `--online` interprets `queue_pending / queue_capacity` as a saturation ratio. ≥ 0.80 → WARN, ≥ 0.95 → FAIL.

**Why:**
- `channelCapacity = 10000` is hardcoded in `internal/embed/queue.go:26`. Worst-case "queue full, chunk dropped" warning fires at `len(q.ch) >= channelCapacity`. We want users to see WARN well before that hard limit.
- Backpressure rejection kicks in at `pending >= 50000` (`rejectionThreshold`, queue.go:34). FAIL at 0.95 of channel capacity is upstream of that — early signal.
- These match precedent in `internal/embed/queue.go:checkCapacity()` which already uses 60 %/90 % WARN/ERROR thresholds for the channel itself. Doctor's thresholds are stricter because doctor is read by humans, not log scrapers.

**Alternative considered:** Use `queue_pending / rejectionThreshold (50000)`. Rejected — that's a workspace-aggregate count, not per-channel; mismatched with the natural "is the buffer about to drop messages" question doctor is answering.

### D4 — Version skew detection compares CLI build version to `/api/status` server version

**Decision:** Require `/api/status` to return a `version` field. Doctor compares it to the CLI's compile-time `Version` constant (currently `cmd/nano-brain/main.go:33` `var Version = "dev"`). Mismatch → WARN with both versions reported.

**Why:** A common failure mode is "CLI in npm cache is stale, server is up-to-date" — query result format may have drifted. WARN (not FAIL) because most version skew is benign.

**Cost:** Adds a `version` field to `statusResponse` in `internal/server/handlers/health.go`. Additive JSON change, no breaking compat.

**Alternative considered:** Add a dedicated `/api/version` endpoint. Rejected — `/api/status` already returns 7+ fields; one more is cheaper than a new route.

### D5 — `NANO_BRAIN_BIN` env var validates file exists AND is executable

**Decision:** When set, `NANO_BRAIN_BIN` overrides binary resolution. Validation: `os.Stat()` succeeds, mode has any `0111` bit set. On failure: print error, exit non-zero, do NOT fall back to PATH.

**Why:** Silent fallback when the user explicitly pointed at a binary is hostile — they want to know if their override is broken. Fail loudly.

**Alternative considered:** Fall back to PATH on validation failure with a WARN. Rejected — masks the user's intent.

### D6 — MCP URL resolution precedence

**Decision:** Resolve in this fixed order:
1. `NANO_BRAIN_MCP_URL` env var (if set and non-empty after trim) → use as-is, no validation
2. If `/.dockerenv` exists → `http://host.docker.internal:3100/mcp`
3. Default → `http://localhost:3100/mcp`

**Why:**
- Env var wins because operators need an explicit override (custom port, remote VPS deployment, etc.).
- `/.dockerenv` detection matches existing precedent in `cmd/nano-brain/guard.go` and `cmd/nano-brain/client.go` (`resolveHostPort` already special-cases container env). Reuse the same probe, don't invent a new one.
- `localhost` default is safer for bare-metal users (the silently-broken case today).

**Alternative considered:** Probe both URLs and use whichever answers. Rejected — adds latency to every CLI invocation, and probes can succeed for the wrong reason (e.g. a different server on `localhost:3100`).

**Alternative considered:** Use `runtime.GOOS == "linux" && hostname == "...docker.internal..."`. Rejected — `/.dockerenv` is the canonical signal and already used elsewhere in the codebase.

### D7 — `nano-brain mcp-url` subcommand prints resolved URL on stdout

**Decision:** New subcommand prints the resolved URL on stdout with no trailing newline beyond `Println`'s default, exit 0. No flags. Designed for skill installers and shell substitution.

**Use case:** `MCP_URL=$(nano-brain mcp-url)` lets a skill-manager postinstall script inject the right URL into a templated `skill.json`.

**Alternative considered:** Add `--mcp-url` to `nano-brain version --which`. Rejected — version is about the binary; URL is about runtime config. Mixing concerns.

### D8 — Phase 3 (`skill.json` templating) chosen: OpenCode `{env:VAR}` substitution

**Decision (resolved by Phase 0 investigation, librarian session `ses_176acfc2dffej4VvNPdnOpj1f0`):** Use OpenCode's native `{env:VAR}` substitution syntax. Change `skill.json:11` from a hardcoded URL to `"url": "{env:NANO_BRAIN_MCP_URL}"`.

**Evidence:**

- OpenCode's `ConfigVariable.substitute` (`packages/opencode/src/config/variable.ts:36`) regex `\{env:([^}]+)\}` substitutes the entire config text **before** schema parsing (`config.ts:420-428`).
- skill-manager merges `skill.json.mcp` directly into the opencode.json config object (`@nano-step/skill-manager/src/config.ts:43-58`), so MCP fields shipped via skill.json pass through the same substitution as native opencode.json fields.
- Confirmed working in OpenCode test `packages/opencode/test/config/config.test.ts:1847-1863` (`it("substitutes {env:} tokens in OPENCODE_CONFIG_CONTENT")`).
- Official docs example: `specs/v2/config.md` shows `"headers": {"Authorization": "Bearer {env:DOCS_TOKEN}"}` in MCP server config.

**Empty-env-var behaviour:** If `NANO_BRAIN_MCP_URL` is unset, `{env:NANO_BRAIN_MCP_URL}` substitutes to `""`. OpenCode then fails at MCP URL validation with a clear error. Acceptable UX — beats silent connection failure to `host.docker.internal` for bare-metal users.

**Alternatives considered and rejected:**

- **3b. Skill-manager postinstall templating** — would have required a custom install-time mustache pass in skill-manager. Rejected — D8 is simpler and supported natively.
- **3c. Dual skill.json variants** (`skill.json.docker`, `skill.json.bare`) — rejected as worst UX, no longer needed.
- **3d. Status quo + docs** — rejected; doesn't fix the silent-failure case D8 was created to solve.

**Phase 3 implementation surface (1 line change + docs):**
- `.opencode/skills/nano-brain/skill.json` line 11: `"url": "{env:NANO_BRAIN_MCP_URL}"`
- `SKILL.md` (both copies): document the env var with two example values (`http://host.docker.internal:3100/mcp` for container, `http://localhost:3100/mcp` for bare-metal).
- skill-manager npm publish cycle (user-owned per Q5).

**Cross-dependency on Phase 1:** Phase 1 ships `NANO_BRAIN_MCP_URL` resolution INSIDE the nano-brain CLI/server (env var → `/.dockerenv` → `localhost`). Phase 3 ships the OpenCode-side consumption (skill.json reads the same env var via `{env:...}`). The env var name is the contract between the two phases.

**Risk:** If a future OpenCode release removes `{env:VAR}` substitution, Phase 3 regresses. Mitigation: pin the contract in nano-brain's docs; if OpenCode breaks compat, fall back to alternative 3b.

### D9 — Phase 4 (opt-in postinstall copy): copy, not symlink, never overwrite

**Decision:** When `NANO_BRAIN_AUTO_LINK=1` env OR `~/.nano-brain/auto-link` marker file exists, `npm/postinstall.js` **copies** (not symlinks) the downloaded binary to:
- Linux: `~/.local/bin/nano-brain`
- macOS: `~/Library/nano-brain/bin/nano-brain`
- Windows: skip entirely

Pre-flight: `fs.existsSync(target) || fs.existsSync(target + '.lnk')` → log WARN, skip. Never overwrite.

After successful copy: print PATH guidance ("Add ~/.local/bin to your PATH:..."). Do NOT modify shell rc files.

**Why copy not symlink:**
- The source binary lives in npm's cache (`node_modules/.bin/...` or version-specific paths). When the user runs `npm update`, the source moves; a symlink would dangle.
- Copy + later `npm update` produces a stale binary — but `version --which` will catch the skew, and the user can re-run install or manually delete.
- Symlink races with npm's internal symlinking can be confusing on macOS (System Integrity Protection edge cases).

**Why per-OS path:**
- `~/.local/bin` is on most Linux distros' default `$PATH` (XDG-compliant). macOS does not include it by default.
- `~/Library/nano-brain/bin/` follows macOS app conventions; users are used to adding `~/Library/<app>/bin` to PATH manually.

**Why skip Windows:** PATH conventions are radically different (PowerShell profile vs CMD vs winget). Not worth the testing budget for a small user base; documented in postinstall WARN.

**Alternative considered:** Default-on auto-link. Rejected — touching user's PATH-visible directories without consent is hostile; opt-in matches Rust / Bun convention.

### D10 — `@beta` → `@latest` migration is atomic per surface

**Decision:** Each user-facing surface (`README.md`, `client_helpers.go`, `SKILL.md`, tests) flips `@beta` to `@latest` in the same PR / commit. Do not leave mixed-state.

**Why:** Users grep docs for invocation patterns. Inconsistency erodes trust ("which one do I copy?").

**Cost:** Updates expected-string asserts in `cmd/nano-brain/commands_test.go` and any `client_helpers_test.go`. Minor.

## Risks / Trade-offs

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| R1 — Phase 0 investigation finds OpenCode does NOT support `${ENV_VAR}` in `skill.json`. Phase 3 falls back to a more invasive option (3b skill-manager templating or 3c dual variants). | Medium | Adds 1-3 days to Phase 3 | Phase 0 ships first; Phase 3 is gated. No commit is made to a 3a-shaped change until Phase 0 says YES. |
| R2 — `NANO_BRAIN_MCP_URL` ergonomics: users set it once in `.zshrc`, forget, then wonder why a remote VPS deployment doesn't work. | Low | Medium | `nano-brain mcp-url` prints the resolved URL. `nano-brain doctor --online` logs which source resolved the URL. Document in SKILL.md troubleshooting. |
| R3 — `/.dockerenv` detection misses some container runtimes (Podman, LXC, GitHub Actions runners). | Medium | Medium | Env var override (`NANO_BRAIN_MCP_URL`) is the escape hatch. Document precedence in SKILL.md. |
| R4 — Phase 4 postinstall copy fails on macOS Gatekeeper (`xattr -dr com.apple.quarantine`). | Medium | Medium | Pre-flight check for quarantine attribute; if present, log INFO with `xattr` workaround. Document in SKILL.md troubleshooting. Skip Windows entirely. |
| R5 — Existing user shim at `~/.local/bin/nano-brain` (e.g. a wrapper script) silently overwritten by Phase 4. | Low | High | Phase 4 detects existing file, prints WARN, skips. No overwrite. Test case `TestPostinstall_AutoLink_SkipsExistingTarget`. |
| R6 — Cross-platform testing burden (Linux glibc/musl, Linux arm64, macOS amd64/arm64, Docker Alpine, behind corp proxy with cert MITM). | High | High | Phase 4 only. Explicit 2-3 day testing budget. Test matrix in tasks.md. Failing platform documented and skipped, not blocked. |
| R7 — `version --which` reports the wrong source when `npm_execpath` is unset (e.g. user `chmod +x` a downloaded binary and ran it directly). | Low | Low | Treat unset `npm_execpath` AND binary not under `node_modules/` as `dev-build` or `path`. Tests cover each branch. |
| R8 — `/api/status` `version` field breaks an external monitor that strict-parses the response. | Very Low | Low | Adding a JSON field is non-breaking in 99 % of clients. No known external consumer; only the CLI parses this response. |
| R9 — User has `NANO_BRAIN_BIN` set to a path that becomes invalid (deleted binary). Loud fail kills their workflow until they unset. | Low | Medium | Error message names the env var and instructs how to unset. Faster recovery than silent fallback. |
| R10 — `mcp-url` subcommand return shape changes break a future skill-manager postinstall script. | Low | Medium | Spec the contract: stdout-only, no trailing whitespace beyond `Println`'s `\n`, exit 0. Test it. |

**Major trade-off: opt-in vs default for Phase 4 auto-link.**
- Opt-in (chosen) → users opt into PATH placement; existing `npm install` UX unchanged. Cost: most users never discover the fast path.
- Default-on (rejected) → discoverable, but installs into user's `~/.local/bin/` without consent. Hostile by current open-source ecosystem norms (rustup, bun, deno all ask first).

**Trade-off accepted by D6: container detection via `/.dockerenv` is not 100 % reliable.**
- Trade-off: simple stdlib check covers Docker (the dominant case), misses some niches.
- Mitigation: env var override is the universal escape hatch. Doctor logs which signal resolved the URL.

## Migration Plan

### Sequence

1. **Phase 0** — pure docs + Phase 0 investigation (no code shipped).
   - Edit `README.md`, `cmd/nano-brain/client_helpers.go`, `SKILL.md` (both copies), tests to replace `@beta` with `@latest`.
   - Phase 0 investigation: confirm OpenCode env-var substitution in `skill.json` (in-tree investigation note, not shipped code).
   - Output: 1 PR ("docs: lead with MCP / npm install -g, drop @beta from user surfaces"). No code paths change.
2. **Phase 1** — CLI additions (additive only).
   - `nano-brain version --which`, `NANO_BRAIN_BIN`, `nano-brain mcp-url`, `NANO_BRAIN_MCP_URL`, `nano-brain doctor --online`, `binary-exists` check.
   - `/api/status` gains `version` field.
   - Output: 1 PR per logical surface (4 PRs total recommended) or 1 monolith PR with clear sections; reviewer's choice.
3. **Phase 3** — `skill.json` change (gated on Phase 0).
   - Only after Phase 0 investigation has a definitive YES on env-var templating.
   - If NO: separate RFC for 3b/3c.
4. **Phase 4** — opt-in postinstall copy.
   - `npm/postinstall.js` gains opt-in branch behind `NANO_BRAIN_AUTO_LINK=1` env or `~/.nano-brain/auto-link` marker.
   - Cross-platform test matrix (see tasks.md).

### Rollback strategy

- Phase 0: `git revert` the docs PR. Zero state change.
- Phase 1: New flags / subcommands are additive. To roll back, `git revert` the PR; existing CLI surface is untouched. `/api/status` `version` field is harmless if rolled back (clients ignore unknown fields by spec).
- Phase 3: The skill.json change ships via skill-manager publish cycle. Rollback = publish the previous skill-manager version. The repo-local `.opencode/skills/nano-brain/skill.json` is reverted via `git revert`.
- Phase 4: `npm/postinstall.js` opt-in. If a platform breaks, change the gate to `NANO_BRAIN_AUTO_LINK=1 && process.platform !== "<broken-platform>"`. Already-copied binaries on user machines stay where they are — they are owned by the user.

### Backward compatibility commitments

- Existing `npx @nano-step/nano-brain ...` invocations continue to work unchanged. The `npm/run.js` wrapper is not modified.
- Existing `nano-brain doctor` (no flags) continues to behave identically. Online checks only activate with `--online`.
- Existing `skill.json` URL keeps `host.docker.internal:3100` until Phase 3 ships.
- `/api/status` response gains one field. No fields are removed or renamed.
- No new required env vars. All new env vars are optional overrides.

## Open Questions

1. **OQ1 (Phase 0 gate) — RESOLVED** — Does OpenCode expand env vars inside `skill.json`? **YES.** Syntax: `{env:VAR_NAME}` (curly-brace with `env:` prefix, NOT `${VAR}`). Applies to all string fields in the config including MCP server URLs merged from skill.json. Evidence: `sst/opencode/packages/opencode/src/config/variable.ts:36` regex + `config.ts:420-428` substitution before parse + `@nano-step/skill-manager/src/config.ts:43-58` merge behaviour + OpenCode test suite confirming the pattern. Phase 3 unblocked; D8 alternative 3a chosen.
2. **OQ2** — Should `nano-brain doctor --online` count `pg_status != "healthy"` as FAIL or WARN? Today's `/api/status` returns `pg_status: "unreachable"` only when `pool.Ping()` fails; the server still answers HTTP requests in that state, but no query will succeed. **Tentative answer: FAIL.** Confirm in PR review.
3. **OQ3** — Phase 4 macOS path: `~/Library/nano-brain/bin/` is unconventional (most CLI tools use `/usr/local/bin` which requires `sudo`). Alternative: `~/.local/bin/` on macOS too (XDG-style, no admin needed, but not in default PATH). **Tentative: keep `~/Library/nano-brain/bin/` per macOS convention.** Decide during Phase 4 cross-platform test.
4. **OQ4** — Should `version --which` also report the running server's version when reachable? Today the CLI doesn't probe the server unless a subcommand requires it. Adding a probe to `version` adds latency. **Tentative: NO** — keep `version --which` offline. Use `doctor --online` for server-side version.
5. **OQ5** — `mcp-url` subcommand output format: bare URL vs key=value (`MCP_URL=http://...`)? Bare URL is easier to consume via `MCP_URL=$(nano-brain mcp-url)`. Key=value matches `--export` precedent in `nano-brain workspaces current --export`. **Tentative: bare URL.** Skill installer can wrap as needed.
6. **OQ6** — Is the existing `/health` endpoint sufficient for the MCP-reachable check in `doctor --online`, or do we need to probe `/mcp` directly? `/health` doesn't tell us the MCP transport is wired; `/mcp` is the actual transport (streamable HTTP, MCP 2025-03-26). **Tentative: probe `/mcp` with a `GET` — it returns a session-initialization response.** Confirm in PR after testing.
