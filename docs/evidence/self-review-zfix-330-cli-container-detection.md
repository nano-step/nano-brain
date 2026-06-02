# Self-review — Issue #330 / PR (TBD)

**Date**: 2026-06-02
**Story**: 330 (CLI defaults to localhost:3100, broken for container agents)
**Lane**: tiny | **Change-type**: bug-fix
**Branch**: `fix/330-cli-container-detection`
**Implementing agent**: Sisyphus orchestrator (direct edit — 3 LOC production change + test scaffolding, no Sisyphus-Junior delegation needed at this scope)

## Scope of changes

| File | Change |
|---|---|
| `cmd/nano-brain/client.go` | +6 lines: new `isContainerFn` test hook + container detection branch in `resolveHostPort()` |
| `cmd/nano-brain/commands_test.go` | +28 lines: 3 new test cases + override `isContainerFn` in `TestGetBaseURL_Defaults` |
| `cmd/nano-brain/ops_test.go` | +3 lines: override `isContainerFn` in `TestResolveHostPort_Defaults` to prevent flakiness when run inside a container |
| `docs/evidence/330-pre-work-gate.md` | new evidence file |
| `docs/evidence/smoke-e2e-330.md` | new evidence file |
| `docs/evidence/self-review-zfix-330-cli-container-detection.md` | this file |

**Production code delta**: 3 logical lines added to `resolveHostPort()`. No new files, no schema changes, no API surface changes.

## Test-injection pattern justification

I introduced `isContainerFn = isContainer` (a package-level var pointing to the real function) to make container detection mockable in tests without touching the filesystem. This matches **3 pre-existing test hooks** in `client.go`:

- `runServeDaemonFn = runServeDaemon` (line 19)
- `promptReader io.Reader = os.Stdin`, `promptWriter io.Writer = os.Stderr` (lines 24-25)
- `isTTYFn = isTTY` (line 29)

Same pattern, identical idiom. Reviewers will recognize it.

## self-review:response-shape

**N/A** — bug-fix to default behavior of an internal function. No HTTP request, no response struct, no JSON marshaling touched.

## self-review:staged-files

```
$ git status (after staging)
On branch fix/330-cli-container-detection
Changes to be committed:
	new file:   docs/evidence/330-pre-work-gate.md
	new file:   docs/evidence/smoke-e2e-330.md
	new file:   docs/evidence/self-review-zfix-330-cli-container-detection.md
	modified:   cmd/nano-brain/client.go
	modified:   cmd/nano-brain/commands_test.go
	modified:   cmd/nano-brain/ops_test.go
```

- ✅ No `.opencode/` files
- ✅ No `package-lock.json`
- ✅ No accidental binary/build artifacts (smoke binary at `/tmp/nano-brain-330` is outside the repo)
- ✅ Every changed file traces to issue #330 (production fix + test coverage + evidence)

## Validation ladder

| Layer | Required | Result |
|---|---|---|
| validate:quick (build + race -short tests) | yes | ✅ ALL PACKAGES PASS |
| self-review:response-shape | N/A | N/A |
| self-review:staged-files | yes | ✅ PASS |
| test:integration | normal+high-risk only — SKIP for tiny | N/A |
| smoke:e2e | yes (bug-fix change-type) | ✅ docs/evidence/smoke-e2e-330.md |
| test:release | before deploy only | deferred |

## Validate:quick evidence

```
$ go build ./...
(no output — success)

$ go test -race -short ./... 2>&1 | grep -E "FAIL|^ok" | tail -25
ok  	github.com/nano-brain/nano-brain/cmd/nano-brain	3.862s
ok  	github.com/nano-brain/nano-brain/internal/bench	(cached)
ok  	github.com/nano-brain/nano-brain/internal/chunk	(cached)
ok  	github.com/nano-brain/nano-brain/internal/config	(cached)
ok  	github.com/nano-brain/nano-brain/internal/embed	(cached)
... (22 packages, all ok)
```

No FAIL lines anywhere in the output. cmd/nano-brain package is the most-modified and runs fresh (3.862s — uncached).

## New unit test evidence

```
$ go test -race -short -run "TestGetBaseURL|TestIsContainer|TestResolveHostPort" ./cmd/nano-brain/... -v 2>&1 | tail -20
=== RUN   TestGetBaseURL_Defaults
--- PASS: TestGetBaseURL_Defaults (0.00s)
=== RUN   TestGetBaseURL_ContainerAutoDetect
--- PASS: TestGetBaseURL_ContainerAutoDetect (0.00s)
=== RUN   TestGetBaseURL_ExplicitHostBeatsContainerAutoDetect
--- PASS: TestGetBaseURL_ExplicitHostBeatsContainerAutoDetect (0.00s)
=== RUN   TestIsContainer_KubernetesEnv
--- PASS: TestIsContainer_KubernetesEnv (0.00s)
=== RUN   TestGetBaseURL_EnvOverride
--- PASS: TestGetBaseURL_EnvOverride (0.00s)
=== RUN   TestGetBaseURL_PartialOverride
--- PASS: TestGetBaseURL_PartialOverride (0.00s)
=== RUN   TestGetBaseURL_EnvVarsFromOS
--- PASS: TestGetBaseURL_EnvVarsFromOS (0.00s)
=== RUN   TestResolveHostPort_Defaults
--- PASS: TestResolveHostPort_Defaults (0.00s)
... (all pass)
```

3 new tests added, all PASS. Pre-existing tests unaffected.

## Backward compat analysis

**Who is affected by this change?**

| User type | Container signal | NANO_BRAIN_HOST | Behavior before fix | Behavior after fix | Impact |
|---|---|---|---|---|---|
| Host user, no env vars | none | unset | `localhost:3100` | `localhost:3100` | ✅ unchanged |
| Host user, explicit | none | `myhost` | `myhost:3100` | `myhost:3100` | ✅ unchanged |
| Container agent, no env vars | `/.dockerenv` | unset | `localhost:3100` (BROKEN) | `host.docker.internal:3100` (WORKS) | ✅ FIX |
| Container agent, explicit | `/.dockerenv` | `localhost` | `localhost:3100` | `localhost:3100` (override wins) | ✅ unchanged |
| Container agent, workaround | `/.dockerenv` | `host.docker.internal` | `host.docker.internal:3100` | `host.docker.internal:3100` (explicit==auto) | ✅ unchanged |
| K8s pod, no env vars | `KUBERNETES_SERVICE_HOST` | unset | `localhost:3100` (BROKEN) | `host.docker.internal:3100` (WORKS) | ✅ FIX |

**Edge case: container agent running its own server at `127.0.0.1:3100`**

If a user (1) starts the nano-brain server INSIDE a container (which AGENTS.md explicitly forbids: "NEVER start nano-brain server inside the container") AND (2) does NOT set `NANO_BRAIN_HOST`, then the CLI inside the same container will redirect to `host.docker.internal:3100` and miss the local server. Mitigation: explicit `NANO_BRAIN_HOST=localhost` (env var always wins). This is an anti-pattern config; AGENTS.md is the canonical reference.

## R29 commit-count

Target: ≤ 3 commits on the branch. Will produce 2-3 commits maximum (initial code + evidence + minor amends if needed).

## R1 issue-closure

PR will explicitly close #330 via `Closes #330` in body.

## Conclusion

The fix is surgical, well-tested, backward-compatible, and aligned with Oracle + Metis verdicts. The test-injection hook (`isContainerFn`) follows the existing codebase pattern. All applicable validation passes.

Ready for review-gate.
