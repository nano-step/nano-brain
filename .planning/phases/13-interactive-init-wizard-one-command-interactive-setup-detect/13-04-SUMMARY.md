---
phase: 13-interactive-init-wizard-one-command-interactive-setup-detect
plan: 04
subsystem: cli
tags: [cli-wizard, embedding, ollama, voyage, koanf, tdd]

# Dependency graph
requires:
  - phase: 13-01
    provides: shared wizard scaffolding conventions (promptWithDefault, isAffirmative, detect* seams)
provides:
  - "stepEmbedding(scanner, notes, defaultURL, defaultModel) (embBlock string) — testable embedding wizard step"
  - "detectOllamaFn test seam (var detectOllamaFn = detectOllama)"
affects: [13-07 (orchestrator concatenates embBlock into final config YAML)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Package-level *Fn seam (detectOllamaFn) overridden via t.Cleanup for test isolation from network calls"
    - "Step functions take an io.Writer for user-facing notes so tests can assert printed hints without stdout capture"

key-files:
  created:
    - cmd/nano-brain/init_embedding.go
    - cmd/nano-brain/init_embedding_test.go
  modified: []

key-decisions:
  - "stepEmbedding signature includes an io.Writer notes parameter (not just scanner+defaults) so the BM25-only note and cloud-auth caveat are assertable independently of the returned YAML block"
  - "RED (test) and GREEN (impl) commits landed together in one feat commit, not split test/feat commits — repo's harness-check.sh pre-commit hook blocks any commit while the build/tests are red, mirroring the precedent set in Phase 10-01/999.1-01"
  - "Cloud-auth caveat (Pitfall 6) triggers on any non-localhost/non-loopback/non-RFC1918-private hostname via net/url.Parse + net.ParseIP; unparseable URLs are treated as non-local (safer default: show the hint)"

patterns-established:
  - "Embedding-step extraction pattern: return the exact YAML block string init.go's embBlock format already produced, so downstream config parsing (orchestrator concatenation in Plan 07) is unchanged"

requirements-completed: [D-11, D-12]

coverage:
  - id: D1
    description: "Enable gate (D-11): declining returns provider: \"\" YAML block and prints a BM25-only note, no further prompts read"
    requirement: "D-11"
    verification:
      - kind: unit
        ref: "cmd/nano-brain/init_embedding_test.go#TestStepEmbedding_Disabled"
        status: pass
    human_judgment: false
  - id: D2
    description: "D-12 ollama-detected branch: injected detectOllamaFn true confirms default URL+model and returns an ollama block"
    requirement: "D-12"
    verification:
      - kind: unit
        ref: "cmd/nano-brain/init_embedding_test.go#TestStepEmbedding_OllamaDetected"
        status: pass
    human_judgment: false
  - id: D3
    description: "D-12 ollama-manual branch: detectOllamaFn false, user picks ollama and enters a URL, returns an ollama block with that URL"
    requirement: "D-12"
    verification:
      - kind: unit
        ref: "cmd/nano-brain/init_embedding_test.go#TestStepEmbedding_OllamaManual"
        status: pass
    human_judgment: false
  - id: D4
    description: "D-12 voyage branch returns a voyage block (provider: voyage, model)"
    requirement: "D-12"
    verification:
      - kind: unit
        ref: "cmd/nano-brain/init_embedding_test.go#TestStepEmbedding_Voyage"
        status: pass
    human_judgment: false
  - id: D5
    description: "Cloud-auth caveat (Pitfall 6): entering a non-local Ollama URL prints an API-key hint, no new auth code"
    verification:
      - kind: unit
        ref: "cmd/nano-brain/init_embedding_test.go#TestStepEmbedding_CloudCaveat"
        status: pass
    human_judgment: false

duration: 22min
completed: 2026-07-02
status: complete
---

# Phase 13 Plan 04: Interactive Init Wizard — Embedding Step Summary

**Extracted the embedding wizard step into `stepEmbedding`, gating on an "enable embeddings?" question (D-11: decline degrades to `provider: ""` for BM25-only) and auto-detecting/prompting an Ollama-compatible URL or Voyage provider on accept (D-12), with a printed cloud-auth caveat for non-local Ollama URLs — zero new provider or auth code.**

## Performance

- **Duration:** 22 min
- **Started:** 2026-07-02T14:36:00Z
- **Completed:** 2026-07-02T14:58:36Z
- **Tasks:** 2 (RED test + GREEN implementation)
- **Files modified:** 2 (both new)

## Accomplishments
- `stepEmbedding` implements the D-11 enable gate: declining returns `embedding:\n  provider: ""\n` (double-quoted empty, no invented `none` sentinel — matches the VERIFIED koanf override behavior from 13-RESEARCH.md) and prints a BM25-only note via the injected `notes io.Writer`, without reading any further prompts.
- D-12 auto-detect/prompt branches: accepting embeddings calls `detectOllamaFn` (test seam over the existing `detectOllama`); found → confirms default URL + `nomic-embed-text` model; not found → prompts provider choice (`ollama`/`voyage`), each producing the exact YAML block shape `init.go`'s original inline code produced (provider/url/model/concurrency for ollama, provider/model/concurrency for voyage).
- Cloud-auth caveat (Pitfall 6): after a user enters an Ollama URL, `isLocalOrPrivateURL` (parses with `net/url` + `net.ParseIP`, checking loopback/RFC1918-private/`"localhost"`) gates a printed hint that hosted/cloud Ollama-compatible endpoints often require an API key nano-brain doesn't currently send — a hint only, no `internal/embed` changes.
- `init_embedding_test.go` covers all 4 resolution paths (disabled / ollama-detected / ollama-manual / voyage) plus the cloud-caveat case, all via the injected `detectOllamaFn` seam — no real network calls (verified via grep for `http.Get` in the test file).

## Task Commits

Both tasks (RED test + GREEN implementation) landed in a single commit because the repo's `.git/hooks/pre-commit` runs `scripts/harness-check.sh in-progress`, which fails the commit if the build/test suite is red — the same constraint documented in STATE.md for Phase 10-01 and 999.1-01. Splitting RED into its own commit was attempted first (`git commit` for the test-only file) and was blocked by the hook with "Build or tests failed", confirming this repo requires RED+GREEN together per task.

1. **Task 1+2: init_embedding.go + init_embedding_test.go — stepEmbedding with D-11/D-12 gates** - `cf673b2` (feat)

**Plan metadata:** (this commit, docs: complete plan)

_Note: TDD RED-only commit was attempted and rejected by the harness pre-commit hook (build/tests must pass); RED and GREEN were committed together in one `feat` commit as a result — see TDD Gate Compliance below._

## Files Created/Modified
- `cmd/nano-brain/init_embedding.go` - `stepEmbedding` step function + `detectOllamaFn` seam + `isLocalOrPrivateURL`/`printCloudCaveatIfRemote` helpers
- `cmd/nano-brain/init_embedding_test.go` - `TestStepEmbedding_*` covering disabled/ollama-detected/ollama-manual/voyage/cloud-caveat

## Decisions Made
- `stepEmbedding(scanner *bufio.Scanner, notes io.Writer, defaultURL, defaultModel string) (embBlock string)` — the plan allowed the signature to include an `io.Writer` for notes "if Task 1's tests need it"; tests needed to assert on the BM25-only note and cloud-auth caveat independently of the returned YAML block, so the `notes io.Writer` parameter was added rather than capturing `os.Stdout`.
- RED-only commit is not achievable in this repo due to the pre-commit harness gate; RED+GREEN were committed together (matches prior-phase precedent already logged in STATE.md).
- Cloud-auth caveat triggers on any non-local hostname (not just a hardcoded `ollama.com` check), using `net/url.Parse` + `net.ParseIP` with loopback/private-range detection, so it generalizes to any hosted/cloud Ollama-compatible endpoint per D-12's "any Ollama-compatible URL" scope.

## Deviations from Plan

None - plan executed exactly as written. Only variance is doc-level: RED and GREEN were committed together instead of as two separate commits, per the repo's pre-commit harness gate (see TDD Gate Compliance below) — this is a process adaptation, not a scope change.

## TDD Gate Compliance

The plan's RED/GREEN task split could not produce two separate commits: the repo's `.git/hooks/pre-commit` invokes `scripts/harness-check.sh in-progress`, which runs `go build`/`go test` and fails the commit (`Build or tests failed`) whenever the working tree is in a red state. This was confirmed empirically — a `git commit` containing only the RED test file was rejected by the hook. RED and GREEN were therefore authored in sequence (test file written and confirmed to fail via `go vet`/`go build` outside of git, then the implementation written and verified to turn all 5 `TestStepEmbedding_*` cases green) and committed together in a single `feat` commit (`cf673b2`). This mirrors the documented precedent in STATE.md for Phase 10-01 ("RED test and GREEN implementation committed together per task ... because repo pre-commit harness-check.sh blocks commits while tests are red") and Phase 999.1-01. No RED-only commit exists in git history for this plan; the fail-fast RED evidence is captured in this SUMMARY instead (see Issues Encountered).

## Issues Encountered
- Attempted `git commit` of the RED-only test file (`init_embedding_test.go` alone, referencing not-yet-existing `stepEmbedding`/`detectOllamaFn`) and confirmed it fails to build via `go vet ./cmd/nano-brain/` (`undefined: detectOllamaFn`) before implementation existed. The commit itself was rejected by the pre-commit hook's harness check, so the RED state was verified out-of-band (via `go vet`) rather than captured as its own commit. Resolved by writing the GREEN implementation immediately after and committing both together.

## Next Phase Readiness
- `stepEmbedding` is ready for Plan 07's orchestrator to call and concatenate its returned `embBlock` into the assembled config YAML, replacing the inline embedding block currently in `runInteractiveInit` (init.go:66-94). Plan 07 is not yet responsible for calling it — no other file was touched in this plan (isolated per plan scope).
- No blockers. `go test -race -short ./...` passes with no regressions across the full repo.

---
*Phase: 13-interactive-init-wizard-one-command-interactive-setup-detect*
*Completed: 2026-07-02*

## Self-Check: PASSED

- FOUND: cmd/nano-brain/init_embedding.go
- FOUND: cmd/nano-brain/init_embedding_test.go
- FOUND: .planning/phases/13-interactive-init-wizard-one-command-interactive-setup-detect/13-04-SUMMARY.md
- FOUND: cf673b2 (feat commit)
- FOUND: 851b6b7 (docs commit, prior revision of this SUMMARY)
