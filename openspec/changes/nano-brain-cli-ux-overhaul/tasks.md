# Tasks — nano-brain CLI UX Overhaul

Phased rollout per `design.md` D1. Each phase is independently shippable.

## 0. Pre-implementation

- [ ] 0.1 Run `deep-design` review of `proposal.md` + `design.md` + all five specs. Address findings before any Phase 1 code.
- [ ] 0.2 Verify Phase 0 librarian investigation finding (OpenCode `{env:VAR}` substitution) by reading `sst/opencode/packages/opencode/src/config/variable.ts:36` and `config.ts:420-428` directly. Paste the actual regex + substitution call in design.md if any drift is found.
- [ ] 0.3 Confirm with the user (or skill-manager publish workflow owner per Q5) the cadence for the skill-manager publish that will ship the updated SKILL.md + skill.json in Phase 3.

## 1. Phase 0 — Pure docs (zero code)

- [ ] 1.1 Replace `@beta` with `@latest` in `cmd/nano-brain/client_helpers.go:69` (`suggestStartCommand`).
- [ ] 1.2 Replace all `@beta` occurrences in `README.md` (3 hits at lines 47, 50, 53 per current grep).
- [ ] 1.3 Verify `.opencode/skills/nano-brain/SKILL.md` already uses `@latest` (line 204) — no change expected; document in PR description if a stray `@beta` is found.
- [ ] 1.4 Update any test that asserts on the suggested start command (`cmd/nano-brain/client_helpers_test.go`, `cmd/nano-brain/commands_test.go`) to expect `@latest`.
- [ ] 1.5 Reorder `README.md` Quick Start so MCP / `npm install -g` is the first/recommended example and `npx` is documented as fallback.
- [ ] 1.6 Reorder `.opencode/skills/nano-brain/SKILL.md` so MCP transport is documented first, then `npm install -g`, then `npx`.
- [ ] 1.7 Add the Troubleshooting section entries to `.opencode/skills/nano-brain/SKILL.md`: `NANO_BRAIN_SKIP_SHA_VERIFY`, `npm install -g --prefix ~/.local`, macOS Gatekeeper `xattr -dr com.apple.quarantine`, `NANO_BRAIN_MCP_URL` (with both example values), `NANO_BRAIN_BIN`.
- [ ] 1.8 Run `validate:quick` (`go build ./... && go test -race -short ./...`).
- [ ] 1.9 Grep verify: `grep -rn '@beta' README.md cmd/nano-brain/ .opencode/skills/nano-brain/` returns no matches.
- [ ] 1.10 Open PR labeled `docs`, `lane:tiny` (or `lane:normal` if change-type policy demands — verify against `docs/FEATURE_INTAKE.md`). No code paths changed.

## 2. Phase 1 — CLI additions

### 2.1 `/api/status` exposes `version` field

- [ ] 2.1.1 Add `Version string \`json:"version,omitempty"\`` field to `statusResponse` struct in `internal/server/handlers/health.go`.
- [ ] 2.1.2 Populate `resp.Version = h.version` in the `Status(c echo.Context)` handler (the same `version` already stored in `Health` struct at construction).
- [ ] 2.1.3 Update `internal/server/handlers/health_test.go` — add `TestStatusReturnsVersion` asserting the `version` field appears with the constructor's version string.
- [ ] 2.1.4 No client breakage check needed: only the CLI consumes `/api/status` (verified via grep).

### 2.2 `nano-brain version --which`

- [ ] 2.2.1 In `cmd/nano-brain/ops.go:runVersionCmd`, parse `--which` flag.
- [ ] 2.2.2 Implement `resolveBinarySource()` helper returning `(path string, source string, err error)` with branch logic per spec scenarios: `env-override`, `npm-local`, `npm-global`, `dev-build`, `path`.
- [ ] 2.2.3 Print three lines on `--which`: `path: <abs>`, `version: <Version>`, `source: <source>`.
- [ ] 2.2.4 `--which --json` emits `{"path":"...","version":"...","source":"..."}` on stdout.
- [ ] 2.2.5 Tests in `cmd/nano-brain/ops_test.go`: `TestVersionWhich_NpmLocal`, `TestVersionWhich_DevBuild`, `TestVersionWhich_EnvOverride`, `TestVersionWhich_Json`. Use `t.Setenv` for env-var-controlled branches.

### 2.3 `NANO_BRAIN_BIN` env override

- [ ] 2.3.1 In `npm/run.js`, after computing the default binary path, check `process.env.NANO_BRAIN_BIN`. If set and non-empty, validate (file exists, mode has executable bit). On validation failure: print error to stderr naming the env var, exit 1.
- [ ] 2.3.2 If validation passes, use the override path as the exec target.
- [ ] 2.3.3 Tests in `npm/postinstall.test.js` (or a new `npm/run.test.js`): `TestRun_NanoBrainBin_ValidPath`, `TestRun_NanoBrainBin_MissingFile`, `TestRun_NanoBrainBin_NotExecutable`, `TestRun_NanoBrainBin_Empty_FallsThrough`.

### 2.4 `nano-brain mcp-url` subcommand

- [ ] 2.4.1 Add `case "mcp-url": runMCPURLCmd(args[1:]); return` in `cmd/nano-brain/main.go` switch.
- [ ] 2.4.2 Implement `runMCPURLCmd` in a new file `cmd/nano-brain/cmd_mcp_url.go`. Reject any flag/positional argument with a `Usage:` line and exit 1. On success, print resolved URL + `\n` and exit 0.
- [ ] 2.4.3 Extract MCP URL resolution into a pure function `resolveMCPURL() string` (likely in a new file `cmd/nano-brain/mcp_url.go` or in `internal/config/`) with precedence: env var (trimmed) → `/.dockerenv` → `localhost`.
- [ ] 2.4.4 Tests: `TestResolveMCPURL_EnvVarWins`, `TestResolveMCPURL_EnvVarWhitespaceTrimmed`, `TestResolveMCPURL_EnvVarEmptyFallsThrough`, `TestResolveMCPURL_DockerEnvDetected`, `TestResolveMCPURL_Default`. Use `t.Setenv` and write a temp `/.dockerenv`-style path injection.
- [ ] 2.4.5 Update `printUsage()` in `cmd/nano-brain/ops.go` to include `mcp-url` in the subcommand list.

### 2.5 `nano-brain doctor --online` + binary-exists offline check

- [ ] 2.5.1 In `internal/health/doctor/doctor.go`, add `CheckBinaryExists(path string) Check` — uses `os.Stat`, checks executable bit. Hooked into `RunAll` as an additional offline check.
- [ ] 2.5.2 Add `CheckServerRunning(baseURL string) Check` — GET `/api/status` with 3 s timeout. Returns the parsed status response on success for downstream checks.
- [ ] 2.5.3 Add `CheckQueueHealth(status statusResponse) Check` — WARN at `queue_pending / queue_capacity >= 0.80`, FAIL at `>= 0.95`. Detail format: `N/M`.
- [ ] 2.5.4 Add `CheckVersionSkew(cliVersion, serverVersion string) Check` — WARN on mismatch with both versions in detail.
- [ ] 2.5.5 Add `CheckMCPReachable(baseURL string) Check` — GET `/mcp` with 3 s timeout, expect HTTP 200.
- [ ] 2.5.6 In `cmd/nano-brain/doctor.go:runDoctorCmd`, parse `--online` flag. When set, call the four runtime checks after the existing offline `RunAll`.
- [ ] 2.5.7 JSON output mode: ensure online checks append to the `checks` array; `all_passed` accounts for all checks.
- [ ] 2.5.8 Tests in `internal/health/doctor/doctor_test.go`: one test per scenario in `specs/cli-doctor-runtime/spec.md` — `TestCheckBinaryExists_Present`, `TestCheckBinaryExists_Missing`, `TestCheckBinaryExists_NotExecutable`, `TestCheckQueueHealth_Nominal`, `TestCheckQueueHealth_Warn80`, `TestCheckQueueHealth_Fail95`, `TestCheckVersionSkew_Match`, `TestCheckVersionSkew_Mismatch`, `TestCheckServerRunning_Unreachable`, `TestCheckMCPReachable_404`. Use `httptest.NewServer` for HTTP mocks.
- [ ] 2.5.9 Integration test in `cmd/nano-brain/doctor_test.go` exercising `runDoctorCmd` with `--online` end-to-end against a `httptest.NewServer`.

### 2.6 Phase 1 validation ladder

- [ ] 2.6.1 `validate:quick` passes: `go build ./... && go test -race -short ./...`.
- [ ] 2.6.2 `self-review:response-shape` — read `statusResponse` struct + `Status(c)` mapping; verify `Version` field is populated.
- [ ] 2.6.3 `self-review:staged-files` — `git status` shows only intended files.
- [ ] 2.6.4 `test:integration` — `go test -race -tags=integration ./...`.
- [ ] 2.6.5 Smoke test: build binary; start server; run each new command (`version --which`, `mcp-url`, `doctor --online`); paste outputs into `docs/evidence/cli-ux-overhaul-phase1.md`.
- [ ] 2.6.6 Open PR labeled `lane:normal`, `component:cli`. Include the evidence file in the PR description.

## 3. Phase 3 — skill.json env-var substitution

(Depends on Phase 1 having shipped `NANO_BRAIN_MCP_URL` handling — the env var contract.)

- [ ] 3.1 Update `.opencode/skills/nano-brain/skill.json` line 11: `"url": "{env:NANO_BRAIN_MCP_URL}"`.
- [ ] 3.2 Coordinate with skill-manager publish owner (per task 0.3): update the SKILL.md and skill.json shipped via `@nano-step/skill-manager` to match.
- [ ] 3.3 Manual verification: install the updated skill in OpenCode running in a container; export `NANO_BRAIN_MCP_URL=http://host.docker.internal:3100/mcp`; confirm MCP tool calls succeed. Then unset the env var; confirm OpenCode reports a clear error (not a silent failure).
- [ ] 3.4 Manual verification on bare-metal: install skill, `export NANO_BRAIN_MCP_URL=http://localhost:3100/mcp`, confirm MCP tool calls succeed.
- [ ] 3.5 Add a regression test in `.opencode/skills/nano-brain/` (or wherever skill tests live) asserting `skill.json` does not contain a hardcoded MCP host.
- [ ] 3.6 Open PR labeled `lane:normal`, `component:skill`. Reference Phase 0 librarian investigation evidence in the PR body.

## 4. Phase 4 — Opt-in postinstall copy

- [ ] 4.1 Update `npm/postinstall.js`: after the existing successful-install path, check opt-in (`process.env.NANO_BRAIN_AUTO_LINK === '1'` OR `fs.existsSync(path.join(os.homedir(), '.nano-brain', 'auto-link'))`).
- [ ] 4.2 Branch on `process.platform`: `linux` → target `~/.local/bin/nano-brain`; `darwin` → target `~/Library/nano-brain/bin/nano-brain`; `win32` → print INFO skip line, return.
- [ ] 4.3 Pre-flight: if target exists, WARN + skip. Never overwrite.
- [ ] 4.4 Create target parent dir with `fs.mkdirSync(..., {recursive: true, mode: 0o755})`.
- [ ] 4.5 Copy source binary to target with `fs.copyFileSync` then `fs.chmodSync(target, 0o755)`.
- [ ] 4.6 Print PATH guidance. Do NOT modify any shell rc file.
- [ ] 4.7 Wrap the copy block in try/catch; on failure print WARN with error message + exit 0 (do not break npm install).
- [ ] 4.8 Tests in `npm/postinstall.test.js`:
  - [ ] `TestAutoLink_OptIn_Linux_HappyPath` — env var set, target absent → file copied with mode 0755.
  - [ ] `TestAutoLink_OptIn_MarkerFile_Linux` — marker present, env unset → file copied.
  - [ ] `TestAutoLink_ExistingTarget_Skipped` — env set, target exists → WARN line printed, target file unchanged (compare bytes).
  - [ ] `TestAutoLink_Windows_Skipped` — env set, `process.platform === "win32"` mocked → INFO line, no copy attempted.
  - [ ] `TestAutoLink_OptOut_Default` — env unset, marker absent → no PATH guidance, no copy.
  - [ ] `TestAutoLink_CopyFailure_NonFatal` — mock `fs.copyFileSync` to throw → WARN line, postinstall exits 0.
  - [ ] `TestAutoLink_MacOS_TargetPath` — env set, `process.platform === "darwin"` → target is `~/Library/nano-brain/bin/nano-brain`.
- [ ] 4.9 Cross-platform manual test matrix (budget: 2-3 days per design.md R6):
  - [ ] macOS arm64 (M-series)
  - [ ] macOS amd64 (Intel)
  - [ ] Linux amd64 (Ubuntu glibc)
  - [ ] Linux arm64 (Ubuntu glibc)
  - [ ] Docker Alpine (musl) — verify Linux branch runs; verify behaviour
  - [ ] Behind corp proxy with cert MITM — verify no extra failures introduced
  - [ ] Document any platform that fails in `docs/evidence/cli-ux-overhaul-phase4-platform-matrix.md` and mark as known-not-supported (do not block on it)
- [ ] 4.10 Open PR labeled `lane:high-risk`, `component:npm-postinstall`. Include the platform matrix evidence.

## 5. Post-merge

- [ ] 5.1 Update CHANGELOG.md with Phase-by-Phase entries under `[Unreleased]`.
- [ ] 5.2 After each PR merge, archive the relevant slice of this change via `openspec archive nano-brain-cli-ux-overhaul` — OR keep the change open across phases and archive at end (decide with reviewer; archive once is simpler).
- [ ] 5.3 Write a `nano-brain write --tags=summary,decision` memory record summarising: phases shipped, Phase 0 investigation outcome, env var contract for skill.json.
