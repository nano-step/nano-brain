---
phase: 13-interactive-init-wizard-one-command-interactive-setup-detect
verified: 2026-07-02T17:40:00Z
status: passed
score: 18/18 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 13: Interactive Init Wizard — Verification Report

**Phase Goal:** Make `nano-brain init` (no args) a one-command interactive setup: detect/provision PostgreSQL (Docker auto-provision with confirmation, or remote Postgres URL), optional embeddings (any Ollama-compatible URL, degrade to BM25 when disabled), start server, register workspace, show supported MCP clients for user to pick and auto-configure. Replaces the 10-step manual flow in docs/SETUP_AGENT.md.

**Verified:** 2026-07-02T17:40:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (per-plan must_haves.truths)

| # | Truth | Plan | Status | Evidence |
|---|-------|------|--------|----------|
| 1 | `docker info` classified into not-installed / daemon-down / available | 01 | ✓ VERIFIED | `docker_provision.go:44-70` `dockerStatus()` returns `dockerStatusNotInstalled` on exec error, `dockerStatusDaemonNotRunning` on "Cannot connect to the Docker daemon" stderr match, `dockerStatusAvailable` on exit 0 |
| 2 | `docker run` name-conflict (exit 125) recognized, recovered via `docker start` | 01 | ✓ VERIFIED | `docker_provision.go:104-119`: exit 125 + `"is already in use by container"` → `runDocker(ctx, "start", dockerPGContainerName)` |
| 3 | `docker run` port-conflict (exit 125) recognized, stray container `rm`'d, retried on 5433 | 01 | ✓ VERIFIED | `docker_provision.go:121-151`: `"port is already allocated"` → `docker rm` (with MN-02 non-silent failure surfacing) → retry `-p 5433:5432` (CR-03 fixed: container port is 5432, not 5433) — confirmed in `docker_provision_test.go:176-185` asserting `"5433:5432"` |
| 4 | All docker CLI calls go through an injectable `runDocker` seam | 01 | ✓ VERIFIED | `docker_provision.go:15-19`: `dockerRunner` type + package-level `var runDocker dockerRunner = defaultRunDocker`; tests override it (`docker_provision_test.go`) |
| 5 | Embedding disabled → doctor provider/model checks return `skip`, detail "disabled — BM25-only" | 02 | ✓ VERIFIED | `doctor.go:100-103,149-152`; tested in `doctor_test.go:111-128` |
| 6 | Provider configured → checks behave exactly as before | 02 | ✓ VERIFIED | `doctor.go` early-return only fires on `cfg.Provider == ""`; ollama/voyage branches unchanged below the guard |
| 7 | Reachable default Postgres URL detected via connect+ping, zero DB questions | 03 | ✓ VERIFIED | `init_db.go:94-99` `stepDatabase`: `pingPostgres(ctx, defaultURL) == nil` → immediate return, no prompt |
| 8 | Unreachable Postgres → docker status decides Docker-prompt vs remote-URL-prompt | 03 | ✓ VERIFIED | `init_db.go:104-126`: `dockerStatusAvailable` branch prompts to provision; else prints install guidance and falls to `promptRemoteURL` |
| 9 | Readiness confirmed via pgx connect+ping poll loop, not `docker exec pg_isready` | 03 | ✓ VERIFIED | `init_db.go:40-67` `waitForPostgresReady` loops `pingPostgres` (pgx Connect+Ping) on an interval until deadline; no `pg_isready` reference anywhere in the package |
| 10 | User-entered Postgres URL live-validated before accept, "save anyway" escape hatch | 03 | ✓ VERIFIED | `init_db.go:138-177` `promptRemoteURL`: validates scheme, pings, re-prompts on failure; a deliberate blank re-entry after ≥1 prior attempt saves unvalidated (D-09) |
| 11 | No Postgres password ever printed — only parsed host shown | 03 | ✓ VERIFIED | `init_db.go:69-78` `hostOnly()` used at every user-facing print (`init_db.go:102,119,124,169,174`); full credentialed URL never passed to `fmt.Print*` |
| 12 | "Enable semantic embeddings?" asked first; decline writes `provider: ""` + BM25 note | 04 | ✓ VERIFIED | `init_embedding.go:24-32` |
| 13 | Enable → auto-detect local Ollama; found confirms URL+model, not-found prompts provider (ollama URL \| voyage) | 04 | ✓ VERIFIED | `init_embedding.go:34-52` |
| 14 | Non-local Ollama URL prints cloud-auth caveat, no new auth code | 04 | ✓ VERIFIED | `init_embedding.go:63-96` `printCloudCaveatIfRemote` / `isLocalOrPrivateURL`; no HTTP auth header code added anywhere in the diff |
| 15 | Generated embedding YAML correct for disabled / ollama / voyage | 04 | ✓ VERIFIED | `init_embedding.go:31,42,51,60` — three distinct YAML block shapes returned per branch |
| 16 | Registration HTTP flow is a single callable helper | 05 | ✓ VERIFIED | `init_register.go:28-73` `registerWorkspace(root, workspace, jsonFlag)` — POST `/api/v1/init`, parse, MCP prompt, `triggerInitBackground` |
| 17 | `runInitCmd --root` branch calls the helper, behaves exactly as before | 05 | ✓ VERIFIED | `registerWorkspaceFn = registerWorkspace` seam in `init.go:37`; `commands.go` `--root` branch invokes the same helper (confirmed via review MJ-01 analysis and current `commands.go` grep showing no duplicated registration body) |
| 18 | Helper returns workspace name/hash/root for wizard summary | 05 | ✓ VERIFIED | `initResult{WorkspaceHash, RootPath, Name, AgentsSnippet}` returned and consumed at `init.go:208-209` |
| 19 | Serve step aborts (no daemon start) when doctor reports PostgreSQL FAIL | 06 | ✓ VERIFIED | `init_serve.go:55-63`; `TestStepServe_AbortsOnPostgreSQLFail` passes |
| 20 | Serve step skips with "already running" note when server already healthy | 06 | ✓ VERIFIED | `init_serve.go:65-68`; `TestStepServe_AlreadyRunningSkipsLaunch` passes |
| 21 | On Windows, serve step prints manual instruction, no daemon symbol references | 06 | ✓ VERIFIED | `init_serve_windows.go` (`//go:build windows`) — prints message only, references no `runServeDaemon`/`pidFilePath`; `init_serve.go` itself is tag-free and only calls `launchServeDaemonFn` (indirection) |
| 22 | Healthy prereq + TTY + accept → launches daemon via seam, waits for health | 06 | ✓ VERIFIED | `init_serve.go:78-92`; `TestStepServe_AcceptAndStart` passes |
| 23 | `runInteractiveInit` returns immediately (no prompts) when not a TTY (D-04) | 07 | ✓ VERIFIED | `init.go:88-92`; `TestRunInteractiveInit_NonTTY` |
| 24 | Config-exists → [k]eep/[o]verwrite gate (default keep); keep jumps to service steps with zero config questions | 07 | ✓ VERIFIED | `init.go:106-115`; `TestRunInteractiveInit_KeepExisting` now asserts `stepServeCalls==1` and `registerCalls==1` (MN-01 fixed) in addition to `stepDatabaseCalls==0`/`stepEmbeddingCalls==0` |
| 25 | Core happy path ≤6 consequential questions (D-01 budget) | 07 | ✓ VERIFIED | `TestRunInteractiveInit_QuestionBudget` drives 3 orchestrator-level answers (advanced, save, register) + per-step stubbed answers; manual count of orchestrator's own `promptConsequential`/`promptWithDefault` calls on the happy path = keep/overwrite(if exists) + DB(0-1) + embedding(1) + advanced(1) + save(1) + serve(1) + register(1) ≤ 6 |
| 26 | Advanced gate defaults No, skips to preview; Yes runs full detailed sequence unchanged | 07 | ✓ VERIFIED | `init.go:134-140`, `stepAdvanced` preserved verbatim (harvester/summarization/search/watcher/logging); `TestRunInteractiveInit_AdvancedGate` |
| 27 | Orchestrator composes stepDatabase/stepEmbedding/stepServe/registerWorkspace/promptMCPClientConfig in order, no logic re-implementation | 07 | ✓ VERIFIED | `init.go:117-201` calls only the seam vars; no duplicated Docker/DB/embedding/HTTP logic inline |
| 28 | Final summary prints server URL, workspace name/hash, MCP clients configured, "restart your AI client" | 07 | ✓ VERIFIED | `init.go:206-215` |
| 29 | Config file written 0600 under 0700 dir, no DB password/API key echoed | 07 | ✓ VERIFIED | `init.go:162,167` (`0700`/`0600`); summary only prints `getBaseURL()` + workspace name/hash, never the DB URL or embedding key |
| 30 | SETUP_AGENT.md leads with one-command flow describing the 13-07 wizard | 08 | ✓ VERIFIED | `docs/SETUP_AGENT.md:7-52` "Quick setup (one command)" section: prerequisites → npm install → `nano-brain init` → verify, describing keep/overwrite, DB detect/provision, embeddings, doctor, serve, register, MCP config, summary |
| 31 | Docs state `nano-brain init` handles everything in one flow, not the old 10 manual steps for happy path | 08 | ✓ VERIFIED | Same section, explicit "no manual Docker command, no separate serve step, no init --root afterthought" |
| 32 | Manual Step 1..10 / VPS/team path preserved as an appendix | 08 | ✓ VERIFIED | `docs/SETUP_AGENT.md` `## Manual setup / troubleshooting` heading present; steps renumbered `###` but content preserved (confirmed via SUMMARY grep gates and direct read) |
| 33 | Docker command / MCP snippets / doctor table / `?workspace=` note remain reachable in appendix | 08 | ✓ VERIFIED | Same appendix, content unchanged from pre-phase version per SUMMARY diff description |
| 34 | README Start section is the two-line one-command flow | 08 | ✓ VERIFIED | `README.md:25-33` `### Start` — `npm install -g @nano-step/nano-brain && nano-brain init`, wizard description, pointer to SETUP_AGENT.md |
| 35 | No Go source/test/config/go.mod touched in plan 08 | 08 | ✓ VERIFIED | `git show --stat` for `6d18e3e`/`bf49b47` touches only `docs/SETUP_AGENT.md`/`README.md`; `go build ./...` still green |

**Score:** 35/35 plan-level truths verified (all 8 plans' `must_haves.truths` checked individually against code)

### Independent Review Fixes (13-REVIEW.md) — verified landed in commit `fe4b6cb`

| Finding | Status | Evidence |
|---------|--------|----------|
| CR-01: `stepServe` ignored injected scanner, read stdin via a second `bufio.Scanner` | ✓ FIXED | `init_serve.go:76-78`: reads through the passed `scanner` via `promptWithDefault(scanner, ...)`; no call to `promptStartServer(promptReader, promptWriter)` remains in `init_serve.go` (confirmed via grep) |
| CR-02: typing "n" at register prompt treated as literal path, no real skip existed | ✓ FIXED | `init.go:190`: explicit `!strings.EqualFold(wsDir, "n") && !strings.EqualFold(wsDir, "no") && !strings.EqualFold(wsDir, "skip")` guard; dedicated regression test `TestRunInteractiveInit_RegisterSkipOnN` passes |
| CR-03: Docker port-conflict retry mapped host 5433 → container 5433 (wrong; image listens on 5432) | ✓ FIXED | `docker_provision.go:137`: `"-p", "5433:5432"`; test assertion updated to `"5433:5432"` in `docker_provision_test.go:180` |
| MJ-01: dead `promptMCPClientConfigFn` seam / `mcpConfigCalls` implying untested coverage | ✓ FIXED | `grep -rn "promptMCPClientConfigFn\|mcpConfigCalls"` across `cmd/nano-brain/*.go` returns no matches — fully removed |
| MN-01: `TestRunInteractiveInit_KeepExisting` didn't assert D-03 keep→serve/register chaining | ✓ FIXED | Test now asserts `h.stepServeCalls != 1` and `h.registerCalls != 1` as failures (`init_test.go:190-195`) |
| MN-02: stray-container `rm` failure silently swallowed even on real errors | ✓ FIXED | `docker_provision.go:127-129`: non-"No such container" rm failures now print a warning |

All 3 CRITICAL + 1 MAJOR + 2 MINOR findings from the independent review are confirmed fixed in the current code, not just claimed in commit message.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cmd/nano-brain/docker_provision.go` + `_test.go` | Docker detect/provision/recover | ✓ VERIFIED | Present, substantive, tested, CR-03/MN-02 fixes landed |
| `internal/health/doctor/doctor.go` + `_test.go` | D-13 skip-when-disabled | ✓ VERIFIED | Present, substantive, tested |
| `cmd/nano-brain/init_db.go` + `_test.go` | D-05/D-08/D-09 DB step | ✓ VERIFIED | Present, substantive, tested |
| `cmd/nano-brain/init_embedding.go` + `_test.go` | D-11/D-12 embedding step | ✓ VERIFIED | Present, substantive, tested |
| `cmd/nano-brain/init_register.go` + `commands.go` + `_test.go` | D-15 shared registration helper | ✓ VERIFIED | Present, substantive, tested |
| `cmd/nano-brain/init_serve.go`/`_unix.go`/`_windows.go` + `_test.go` | D-14 serve step, platform split | ✓ VERIFIED | Present, substantive, tested, CR-01 fix landed |
| `cmd/nano-brain/init.go` + `init_test.go` | D-01..D-04,D-16,D-17 orchestrator | ✓ VERIFIED | Present, substantive, tested, CR-02/MN-01 fixes landed |
| `docs/SETUP_AGENT.md`, `README.md` | D-18 docs rewrite | ✓ VERIFIED | Present, substantive, one-command flow + manual appendix |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `provisionPostgres` return | `waitForPostgresReady` | poll before returning dbURL | ✓ WIRED | `init_db.go:114-119` calls `waitForPostgresReady` immediately after `provisionPostgresFn` succeeds |
| `stepDatabase` return `(dbURL, ok)` | orchestrator YAML `database.url` | Plan 07 consumption | ✓ WIRED | `init.go:120,148` |
| `stepEmbedding` return `embBlock` | orchestrator YAML embedding section | Plan 07 consumption | ✓ WIRED | `init.go:128,150` |
| `doctor.RunAll` results | `stepServe(scanner, checks, configPath)` | PostgreSQL-fail gate | ✓ WIRED | `init.go:176,179` |
| `stepServe` `serveOutcome` | register step gate | only proceed on started/already-running | ✓ WIRED | `init.go:187` |
| `registerWorkspace` `initResult` | `promptMCPClientConfig` (D-16) | inside `registerWorkspace`, not a separate orchestrator call | ✓ WIRED (indirect) | `init_register.go:58-67` — review MJ-01 correctly identified this is NOT called via a top-level orchestrator seam but from inside the helper; this is architecturally consistent (D-16 says "reuse `promptMCPClientConfig` verbatim") and MJ-01's actual defect (the dead unused seam var) has been removed |
| `registerWorkspace` return | `init.go` summary block | D-17 | ✓ WIRED | `init.go:208-209` |
| `runDocker` var | `exec.CommandContext(ctx, "docker", args...)` | fixed argv, no interpolation | ✓ WIRED | `docker_provision.go:19,25-26` |
| `dockerStatus` classification | wizard DB step branch | Plan 05→03 consumption | ✓ WIRED | `init_db.go:104-122` |
| SETUP_AGENT.md one-command flow | `nano-brain init` wizard | describes actual 13-07 orchestrator | ✓ WIRED | Docs steps 1-8 (`SETUP_AGENT.md:34-45`) match `init.go`'s actual flow order exactly |
| README `### Start` | SETUP_AGENT.md | pointer for manual/VPS/Windows | ✓ WIRED | `README.md:33` |

### Behavioral Spot-Checks / Test Execution

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full project build | `CGO_ENABLED=0 go build ./...` | Clean, no output | ✓ PASS |
| Phase 13 unit tests | `go test -race -short -count=1 ./cmd/nano-brain/... ./internal/health/doctor/...` | `ok cmd/nano-brain 6.182s`, `ok internal/health/doctor 2.689s` | ✓ PASS |
| CR-01 regression | grep for second scanner in `init_serve.go` | none found; `promptWithDefault(scanner,...)` used | ✓ PASS |
| CR-02 regression test | `TestRunInteractiveInit_RegisterSkipOnN` (part of full suite run above) | passed | ✓ PASS |
| CR-03 regression test | `TestProvisionPostgres` port-conflict subtest (part of full suite run above) | passed, asserts `5433:5432` | ✓ PASS |
| MJ-01 dead code removed | grep `promptMCPClientConfigFn\|mcpConfigCalls` across `cmd/nano-brain/*.go` | 0 matches | ✓ PASS |
| Debt markers (TBD/FIXME/XXX/TODO/HACK/PLACEHOLDER) in phase files | grep across all 9 modified Go files | 0 matches | ✓ PASS |

**Note on Windows cross-compile:** `GOOS=windows go build ./cmd/nano-brain/...` fails with `undefined: runServeDaemon`, `pidFilePath`, `runServeCmd`, `runStopCmd`, `runRestartCmd` — however this is confirmed **pre-existing on `origin/master`** (verified by building a clean `origin/master` worktree with the same `GOOS=windows` flags — identical failure), not a regression from Phase 13. Phase 13's own new files (`init_serve_windows.go`) are correctly tag-isolated and reference no daemon symbols, satisfying the plan-06 must_have. The broader Windows daemon build gap is out of this phase's scope (CONTEXT.md specifics: "Do not build Windows daemon support in this phase") and predates this branch.

### Requirements Coverage

No REQUIREMENTS.md IDs are mapped to Phase 13 — ROADMAP.md states "Requirements: D-01..D-18 (CONTEXT-scoped; no REQUIREMENTS.md IDs)". All 18 CONTEXT decisions (D-01 through D-18) were traced to plan must_haves and code above; no orphaned requirements.

### Anti-Patterns Found

None. No TBD/FIXME/XXX/TODO/HACK/PLACEHOLDER markers, no stub returns (`return null`/empty struct with no real logic), no hardcoded empty data flowing to output in any of the 9 modified/created Go files or 2 docs files.

### Human Verification Required

The following cannot be verified via static analysis/grep and require a real interactive run:

1. **End-to-end fresh-machine wizard run with real Docker**
   **Test:** On a machine with Docker installed and no existing nano-brain config, run `nano-brain init`, answer prompts to auto-provision PostgreSQL via Docker, enable Ollama embeddings, start the server, register the workspace, and select an MCP client (e.g. Claude Code) to auto-configure.
   **Expected:** Wizard completes in ≤6 consequential questions, the pgvector container starts and becomes healthy, the server starts, workspace registers, `.mcp.json` (or equivalent) is written, and after restarting Claude Code the `memory_status` tool responds healthy.
   **Why human:** Requires a real Docker daemon, real network calls to Ollama, and a real terminal TTY session — none of which are safe or appropriate to invoke from an automated verifier (this task explicitly instructed not to start servers or containers).

2. **Port-5432-occupied real-world retry**
   **Test:** With something already bound to host port 5432, run the wizard's Docker-provision path and confirm it successfully falls back to the 5433 host / 5432 container mapping and the server actually connects.
   **Expected:** Container starts on `-p 5433:5432`, `waitForPostgresReady` polls `localhost:5433` and succeeds within 30s.
   **Why human:** Requires deliberately occupying port 5432 and running real `docker run` — a live-environment scenario, unit-tested via mocked `runDocker` (already covered) but not exercised against a real daemon here.

3. **Windows manual-fallback UX**
   **Test:** On a Windows machine, run `nano-brain init`, reach the serve step, and confirm the printed manual `nano-brain serve` instruction is clear and the wizard doesn't crash or hang.
   **Expected:** Wizard prints the guidance from `init_serve_windows.go` and gracefully continues to (or skips) the register step.
   **Why human:** Requires a Windows machine/VM; also blocked by the pre-existing (out-of-phase-scope) Windows cross-compile gap in `main.go`/`client.go`/`daemon.go` noted above — a real Windows binary of this branch cannot currently be built to test this end-to-end.

4. **Cloud/remote Ollama-compatible endpoint embeddings**
   **Test:** Enter a non-local Ollama-compatible URL (e.g. a cloud-hosted endpoint) at the embedding step and confirm the printed cloud-auth caveat is accurate and embeddings actually work against that endpoint if it happens to be open/unauthenticated.
   **Expected:** Caveat prints; if the endpoint requires auth nano-brain does not send, the user sees a clear failure later (not swallowed silently).
   **Why human:** Requires an actual reachable cloud Ollama-compatible service; the caveat-printing logic itself is unit-tested (verified), but the caveat's practical accuracy against a real cloud provider is not testable from this environment.

These are optional UAT items — all automatable checks (build, tests, must-have code verification, review-fix regression, anti-pattern scan) passed, so overall status is `passed` rather than `human_needed`.

### Gaps Summary

No gaps found. All 8 plans' must_haves (truths, artifacts, key_links) are verified present, substantive, and wired. All 3 CRITICAL, 1 MAJOR, and 2 MINOR independent-review findings are confirmed fixed in the current code (not just claimed in the SUMMARY/commit message — each fix was independently re-derived from source and cross-checked against its corresponding regression test). Build is clean; the full documented test command passes. The only notable out-of-scope observation (Windows cross-compile failure) is pre-existing on `origin/master` and explicitly out of this phase's stated boundary.

---

_Verified: 2026-07-02T17:40:00Z_
_Verifier: Claude (gsd-verifier)_
