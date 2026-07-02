---
phase: 13-interactive-init-wizard-one-command-interactive-setup-detect
reviewed: 2026-07-02T16:08:38Z
status: findings
reviewer: gsd-code-reviewer (independent, R88)
branch: feat/interactive-init-wizard
base: origin/master
files_reviewed:
  - cmd/nano-brain/init.go
  - cmd/nano-brain/commands.go
  - cmd/nano-brain/docker_provision.go
  - cmd/nano-brain/docker_provision_test.go
  - cmd/nano-brain/init_db.go
  - cmd/nano-brain/init_db_test.go
  - cmd/nano-brain/init_embedding.go
  - cmd/nano-brain/init_embedding_test.go
  - cmd/nano-brain/init_register.go
  - cmd/nano-brain/init_register_test.go
  - cmd/nano-brain/init_serve.go
  - cmd/nano-brain/init_serve_test.go
  - cmd/nano-brain/init_serve_unix.go
  - cmd/nano-brain/init_serve_windows.go
  - cmd/nano-brain/init_test.go
  - internal/health/doctor/doctor.go
  - internal/health/doctor/doctor_test.go
  - docs/SETUP_AGENT.md
  - README.md
findings:
  critical: 3
  major: 1
  minor: 2
  nit: 1
---

# Phase 13: Interactive Init Wizard — Independent Code Review

**Reviewed:** 2026-07-02T16:08:38Z
**Branch:** feat/interactive-init-wizard vs origin/master
**Status:** findings (3 CRITICAL, 1 MAJOR, 2 MINOR, 1 NIT)

## Evidence

```
$ go build ./...
(clean, no output)

$ go test -race -short -count=1 ./cmd/nano-brain/... ./internal/health/doctor/...
ok  	github.com/nano-brain/nano-brain/cmd/nano-brain	6.754s
ok  	github.com/nano-brain/nano-brain/internal/health/doctor	1.820s
```

Build and existing test suite are green. All findings below are **logic/integration bugs the test suite does not currently catch** — each finding includes the specific reason the existing tests miss it, since "tests pass" was not treated as evidence of correctness here.

## Summary

The wizard restructuring (D-01 through D-17) is well-decomposed into testable step functions with clean seams, and the doctor `provider==""` skip path (D-13) is correct and well-tested. However, three CRITICAL defects would break the wizard for real interactive users in ways the mocked/stubbed test harness cannot observe:

1. A stdin-splitting bug where `stepServe` silently ignores its injected `scanner` and reads from a second, independent `bufio.Scanner` over the same `os.Stdin` fd.
2. The D-15 "empty-ish 'n' to skip" registration-decline contract does not exist in the code — typing "n" is treated as a literal directory name and registration is attempted against a bogus path.
3. The Docker port-conflict fallback (D-07) maps host port 5433 to the wrong container port, so the retry container's Postgres is never reachable at the URL the wizard writes to config.

Additionally there is dead/misleading test-seam scaffolding (`promptMCPClientConfigFn`) and a test-coverage gap on the D-03 keep-path chaining to serve/register/MCP.

## Critical Issues

### CR-01: `stepServe` ignores its injected `scanner`, reads stdin via a second independent `bufio.Scanner`

**File:** `cmd/nano-brain/init_serve.go:52,76`
**Issue:** `stepServe(scanner *bufio.Scanner, checks []doctor.Check, configPath string)` takes a `scanner` parameter but never calls `scanner.Scan()` or reads from it anywhere in the function body (verified: `grep -n "scanner" init_serve.go` matches only the signature). Instead, line 76 calls `promptStartServer(promptReader, promptWriter)`, which internally constructs its **own** `bufio.NewScanner(reader)` (`client_helpers.go:20`) over the package-level `promptReader` var — which defaults to `os.Stdin` (`client.go:24`).

In production, `runInteractiveInit` creates one `scanner := bufio.NewScanner(os.Stdin)` (`init.go:102`) and uses it for every other prompt in the flow (database, embedding, advanced gate, save confirm, register, MCP client Y/N). But when execution reaches `stepServeFn(scanner, checks, configPath)`, the "Start server now?" prompt is answered through a **second, independent** `bufio.Scanner` wrapping the same underlying `os.Stdin` file descriptor. `bufio.Scanner` performs internal buffered reads (default 4096-byte buffer) ahead of what it returns via `Text()`. Two scanners alternating reads on the same fd will race for buffered bytes: whichever scanner's `Scan()` call executes first can silently consume bytes intended for the *other* scanner's next read (e.g., part of the answer to "Register this directory as a workspace?" or an MCP client Y/N line typed slightly ahead of the prompt appearing, which is common when a user pastes multiple answers or a terminal delivers a full line at once).

This is a genuine "double prompts / steps skipped" class defect the task explicitly asked to check for. It is completely untested: `init_serve_test.go`'s `withServeHooks` overrides `promptReader`/`promptWriter` directly with fresh `bytes.Buffer`s (never touching `os.Stdin`), and `init_test.go`'s orchestrator tests stub `stepServeFn` entirely so the real `stepServe` body is never exercised end-to-end with a shared `os.Stdin`.

**Fix:** `stepServe` should read the "Start server now?" answer through the injected `scanner`, not a second scanner over `promptReader`. Either add a `promptConsequential`/`promptWithDefault`-based call using `scanner` directly, or change `promptStartServer` to accept a `*bufio.Scanner` instead of an `io.Reader` so the same scanner instance is reused across the whole wizard:

```go
// init_serve.go
func stepServe(scanner *bufio.Scanner, checks []doctor.Check, configPath string) serveOutcome {
	...
	answer := promptWithDefault(scanner, "Start server now?", "Y")
	if !isAffirmative(answer) {
		fmt.Printf("  Skipped. Start it later with: %s\n", suggestStartCommand())
		return serveSkipped
	}
	...
}
```
and drop the call to `promptStartServer(promptReader, promptWriter)` from this path (that helper can remain for the unrelated `recoverFromConnectionRefused` use site in `client.go`, which is a single-shot CLI invocation with no competing scanner).

---

### CR-02: D-15 "type n to skip registration" contract does not exist — "n" is treated as a literal path

**File:** `cmd/nano-brain/init.go:190-201`
**Issue:** Context decision D-15 states: `"Register this directory as a workspace? [<cwd>]" (Enter = cwd, or type another path, empty-ish "n" to skip)`. The implementation is:

```go
wsDir, ok := promptConsequential(scanner, "Register this directory as a workspace?", cwd)
if ok && wsDir != "" {
    res, err = registerWorkspaceFn(wsDir, "", false)
    ...
} else {
    fmt.Println("  Skipped registration — MCP client config needs a registered workspace.")
}
```

`promptConsequential` (`mcp_client_config.go:349`) returns `(defaultVal, true)` whenever the user presses Enter (empty input maps to the default, here `cwd`) — it **never returns an empty string** for a normal answer; the only way `wsDir == ""` is if `ok == false` (closed stdin/EOF). This means:

- Pressing Enter → registers `cwd` (correct).
- Typing "n" (the documented skip keyword) → `wsDir = "n"`, `ok = true` → falls into the **registration branch**, calling `registerWorkspaceFn("n", "", false)` — attempting to register a workspace rooted at a literal relative path `"n"` (which will almost always fail to resolve, or worse, silently resolve to an unintended directory named `n` if one exists in cwd).
- There is no way to actually skip registration on an open TTY at all — the only skip path is a closed/EOF stdin, which is not what a user driving the wizard interactively can do.

This is untested: `init_test.go`'s `TestRunInteractiveInit_QuestionBudget` and all other orchestrator tests only ever send `"Y\n"` (or nothing, relying on the default) for this prompt; no test sends `"n\n"` to the register step.

**Fix:** Special-case an "n"/"no"/"skip" answer (or empty-after-default per the `promptConsequential` return) explicitly:

```go
wsDir, ok := promptConsequential(scanner, "Register this directory as a workspace?", cwd)
if ok && wsDir != "" && !strings.EqualFold(wsDir, "n") && !strings.EqualFold(wsDir, "no") && !strings.EqualFold(wsDir, "skip") {
    res, err = registerWorkspaceFn(wsDir, "", false)
    ...
} else {
    fmt.Println("  Skipped registration — MCP client config needs a registered workspace.")
}
```

---

### CR-03: Docker port-conflict retry maps host:5433 to the wrong container port — Postgres unreachable at the URL the wizard writes to config

**File:** `cmd/nano-brain/docker_provision.go:128-137`
**Issue:** When the primary `docker run` on port 5432 fails with "port is already allocated" (D-07's port-conflict path), the retry uses:

```go
runArgs5433 := []string{
    "run", "-d",
    "--name", dockerPGContainerName,
    "--restart", "unless-stopped",
    "-p", "5433:5433",
    ...
    dockerPGImage,
}
```

and returns `url5433 = "postgres://nanobrain:nanobrain@localhost:5433/nanobrain_dev?sslmode=disable"`. Docker's `-p HOST:CONTAINER` mapping here forwards host port 5433 to **container port 5433**. But the `pgvector/pgvector:pg17` image's `postgres` process listens on **port 5432 inside the container** (no `PGPORT`/`-p` config override is set to change that). Nothing is listening on container port 5433, so the host-side forward goes nowhere.

Consequence: `waitForPostgresReady` (`init_db.go:46`) will poll `postgres://...@localhost:5433/...` for up to 30 seconds and then fail with `"postgres did not become ready within 30s"` (`stepDatabase`, `init_db.go:115-118`), even though `docker run` itself reported success — the user is dropped into `promptRemoteURL` with a confusing failure despite the "successful" provisioning. If the docker daemon takes a large host-port range or 5432 stays occupied by something else, this path is the ONLY fallback and it is dead on arrival.

This is untested at the level that would catch it: `docker_provision_test.go`'s `TestProvisionPostgres/port-conflict path` only asserts the retry args contain `"5433:5433"` literally (i.e., the test encodes the bug as the expected behavior) — it never asserts against the image's actual internal listen port, and `init_db_test.go`'s `TestStepDatabase_DockerAvailable_ProvisionsAndPolls` stubs `provisionPostgresFn` directly, bypassing `provisionPostgres`'s real Docker args entirely.

**Fix:** Map host 5433 to container 5432:

```go
runArgs5433 := []string{
    "run", "-d",
    "--name", dockerPGContainerName,
    "--restart", "unless-stopped",
    "-p", "5433:5432",
    "-e", "POSTGRES_USER=nanobrain",
    "-e", "POSTGRES_PASSWORD=nanobrain",
    "-e", "POSTGRES_DB=nanobrain_dev",
    dockerPGImage,
}
```
and update the test's assertion from `"5433:5433"` to `"5433:5432"`.

## Major Issues

### MJ-01: `promptMCPClientConfigFn` seam is dead code — never wired into the real call chain

**File:** `cmd/nano-brain/commands.go:38`, `cmd/nano-brain/init_register.go:66`
**Issue:** `commands.go` declares `promptMCPClientConfigFn = promptMCPClientConfig` as one of the orchestrator seams ("each defaults to the real Wave-1/2 step function... tests can override them"), and `init_test.go`'s `withOrchestratorHooks` overrides it and increments `h.mcpConfigCalls`. But `runInteractiveInit` never calls `promptMCPClientConfigFn` directly — the MCP client step (D-16) is actually invoked from inside `registerWorkspace` (`init_register.go:66`), which calls the **real, unmocked** `promptMCPClientConfig` function directly, not through the seam.

Net effect: (a) the seam is misleading — it looks wired into the D-16 step but isn't; (b) `mcpConfigCalls` in the test harness can never be incremented by anything except the test's own manual override of a function pointer nothing calls, and — confirmed by grep — no test actually asserts on `mcpConfigCalls` at all, so this is silently untested dead instrumentation; (c) any orchestrator test using `withOrchestratorHooks` with a stubbed `registerWorkspaceFn` (which all current tests do) can never exercise the real D-16 MCP prompt through the orchestrator's own tests — that path is only covered indirectly via `init_register_test.go`, which doesn't test the interactive MCP-prompt trigger condition at all (it only covers the empty-name-skip guard).

**Fix:** Either route the call through the seam for consistency (`registerWorkspace` would need the seam injected/parameterized, which changes its signature and ripples into `runInitCmd`'s `--root` path), or remove the unused `promptMCPClientConfigFn` var and `mcpConfigCalls` field/assertions to stop implying coverage that doesn't exist. Minimal fix: delete the dead seam and field, and add a dedicated test (e.g., via `registerWorkspace`'s existing httptest-based tests in `init_register_test.go`) that drives stdin through the "yes, configure Claude Code" path so D-16 has real regression coverage.

## Minor Issues

### MN-01: `TestRunInteractiveInit_KeepExisting` doesn't verify the D-03 "keep still chains to serve/register/MCP" contract

**File:** `cmd/nano-brain/init_test.go:138-194`
**Issue:** D-03 requires: "Keep → skip all config questions and jump directly to the service steps (doctor → serve → register → MCP)." `TestRunInteractiveInit_KeepExisting` asserts `h.stepDatabaseCalls == 0`, `h.stepEmbeddingCalls == 0`, and `h.doctorCalls != 0`, but never asserts `h.stepServeCalls != 0` or `h.registerCalls != 0`. The test's own stdin (`"k\n"` then closed) would make a real `stepServe`/register prompt read fail closed, but since `stepServeFn`/`registerWorkspaceFn` are fully stubbed in this harness, the test can't distinguish "keep path correctly falls through to serve/register" from "keep path silently `return`s after doctor and never reaches serve/register." Manual code review (`init.go:174-216`, unconditionally after the `if !keep {}` block closes) confirms the current code is correct, but the test would not catch a future regression that mistakenly wraps the serve/register block inside the `!keep` conditional.

**Fix:** Add assertions `if h.stepServeCalls != 1 { t.Error(...) }` and `if h.registerCalls != 1 { t.Error(...) }` to `TestRunInteractiveInit_KeepExisting`, with stdin supplying enough answers for the stubbed serve/register calls to proceed (they're stubbed, so no extra input should be strictly required, but the register prompt inside the real `runInteractiveInit` body still reads from `scanner` at `init.go:190` and needs an answer or the test will hang on a truly empty pipe — verify by adding e.g. `"k\nY\n"` to the test's input).

### MN-02: `provisionPostgres`'s stray-container `rm` failure is silently swallowed even on real errors

**File:** `cmd/nano-brain/docker_provision.go:124-126`
**Issue:** `_, _, _, _ = runDocker(ctx, "rm", dockerPGContainerName)` discards all outputs including `err` (execution failure, e.g., docker binary disappearing mid-flow) and `exitCode`. The comment justifies ignoring "no such container," but a genuinely different failure (e.g., a permissions error, or the container being in a state that can't be removed) is masked identically, and the subsequent retry `docker run` on 5433 will then fail with a **second, unrelated** "name already in use" error that's confusing relative to the actual root cause (the failed `rm`).

**Fix:** At minimum log the rm failure for diagnostic purposes when it's not the expected "no such container" case, e.g.:
```go
if _, rmStderr, rmExit, rmErr := runDocker(ctx, "rm", dockerPGContainerName); rmErr != nil || (rmExit != 0 && !strings.Contains(rmStderr, "No such container")) {
    // best-effort cleanup failed for a reason other than "already gone" — the
    // retry below will likely fail too; surface why.
}
```
This is minor because the retry's own error message will still surface *a* failure to the user (just a less accurate one).

## Nit

### NIT-01: Windows doc cross-reference points at a manual step whose heading text changed

**File:** `docs/SETUP_AGENT.md:38`
**Issue:** `> **Windows note:** ... see [Step 7 — Start the server](#step-7--start-the-server) in the manual appendix.` The anchor is generated from the now-nested heading `### Step 7 — Start the server` (previously `## Step 7`), so the auto-generated GitHub anchor slug is unchanged (`#step-7--start-the-server`) and the link resolves correctly — verified no break. Flagging only because this is exactly the kind of doc/anchor drift that silently breaks on a future heading-level rename; no fix needed now.

---

_Reviewed: 2026-07-02T16:08:38Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep (cross-file trace of scanner/stdin ownership, Docker argv construction, and D-13/D-15/D-16 contract verification against locked CONTEXT decisions)_
