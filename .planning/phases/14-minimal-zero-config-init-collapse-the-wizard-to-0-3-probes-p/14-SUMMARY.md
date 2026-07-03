---
phase: 14-minimal-zero-config-init-collapse-the-wizard-to-0-3-probes-p
plan: 01
subsystem: infra
tags: [cli, config, koanf, yaml, init-wizard, postgres, ollama]

# Dependency graph
requires:
  - phase: 13-one-command-init-wizard
    provides: stepDatabase (Postgres probe/Docker auto-provision), detectOllama, detectOpenCodeStorageDir/detectClaudeCodeStorageDir, stepServe/registerWorkspace chain
provides:
  - internal/config.RenderConfig / fullConfigTemplate — the hand-authored, koanf-key-correct, fully commented default config template
  - Zero-question interactive init when Postgres + Ollama are reachable
  - nano-brain init --yes non-interactive path
affects: [future config-schema changes must update fullConfigTemplate in lockstep with Config struct koanf tags, or the round-trip test fails]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Config templates are hand-authored Go string constants with a round-trip test (render -> Load -> compare against Load(\"\")) as the correctness gate against koanf/yaml key drift, rather than yaml.Marshal(struct)."
    - "Non-interactive CLI paths reuse the same probe/build helpers as interactive paths by passing a scanner over an already-closed reader, so every internal prompt takes its EOF-means-decline branch automatically."

key-files:
  created:
    - internal/config/template.go
    - internal/config/template_test.go
  modified:
    - cmd/nano-brain/init.go
    - cmd/nano-brain/init_embedding.go
    - cmd/nano-brain/init_embedding_test.go
    - cmd/nano-brain/init_test.go
    - cmd/nano-brain/commands.go
    - cmd/nano-brain/ops.go

key-decisions:
  - "Config template is a hand-authored Go string constant (internal/config/template.go), not yaml.Marshal(getDefaults()) — the Config structs carry koanf/json tags but no yaml tags, so yaml.v3 lowercases field names and the marshaled output does not round-trip through config.Load (D-04)."
  - "Round-trip test compares the rendered-then-loaded config against Load(\"\") (defaults loaded through the same koanf path), not raw getDefaults() — koanf's structs.Provider normalizes nil slices/maps to non-nil-empty as an unrelated pre-existing Load() characteristic; comparing against getDefaults() directly produced false positives."
  - "stepEmbedding collapsed to at most one prompt: Ollama reachable -> zero questions; Ollama unreachable -> one prompt for a URL (blank = BM25-only). No provider-choice (voyage) or concurrency prompt remains — that tuning moves to manual config-file editing per D-07."
  - "--yes (runNonInteractiveInit) does not auto-provision Postgres via Docker even when Docker is available, because that offer is itself an interactive prompt and EOF means decline — it aborts with a clear message pointing at NANO_BRAIN_DATABASE_URL/DATABASE_URL instead of silently starting a container without consent."
  - "--yes does not chain into serve/register/MCP client config — those remain interactive-only gates; D-08 scope is limited to collapsing config-building to zero prompts."

requirements-completed: []

coverage:
  - id: D1
    description: "Hand-authored commented full-config template (RenderConfig) with every Config section at getDefaults() values, correct snake_case koanf keys, advanced sections enabled:false, context_hints shown commented"
    verification:
      - kind: unit
        ref: "internal/config/template_test.go#TestRenderConfig_RoundTrip"
        status: pass
      - kind: unit
        ref: "internal/config/template_test.go#TestRenderConfig_Substitutions"
        status: pass
    human_judgment: false
  - id: D2
    description: "No API key ever written to the rendered config file"
    verification:
      - kind: unit
        ref: "internal/config/template_test.go#TestRenderConfig_NoSecretsWritten"
        status: pass
    human_judgment: false
  - id: D3
    description: "runInteractiveInit collapsed to 0 config questions when Postgres+Ollama reachable; stepAdvanced and config-detail prompts removed; keep/overwrite + doctor->serve->register->MCP retained"
    verification:
      - kind: unit
        ref: "cmd/nano-brain/init_test.go#TestRunInteractiveInit_ZeroQuestions"
        status: pass
      - kind: unit
        ref: "cmd/nano-brain/init_test.go#TestRunInteractiveInit_WritesCommentedTemplate"
        status: pass
      - kind: unit
        ref: "cmd/nano-brain/init_test.go#TestRunInteractiveInit_KeepExisting"
        status: pass
      - kind: e2e
        ref: "manual: nano-brain init --yes on this machine (Postgres+Ollama up) — 0 prompts, doctor OK"
        status: pass
    human_judgment: false
  - id: D4
    description: "stepEmbedding simplified: Ollama up -> silent default; Ollama down -> one prompt or BM25; no provider/concurrency prompts"
    verification:
      - kind: unit
        ref: "cmd/nano-brain/init_embedding_test.go#TestStepEmbedding_OllamaUp"
        status: pass
      - kind: unit
        ref: "cmd/nano-brain/init_embedding_test.go#TestStepEmbedding_OllamaDownBlankAnswer"
        status: pass
      - kind: unit
        ref: "cmd/nano-brain/init_embedding_test.go#TestStepEmbedding_OllamaDownProvidedURL"
        status: pass
    human_judgment: false
  - id: D5
    description: "--yes non-interactive path writes a valid default config with zero prompts"
    verification:
      - kind: unit
        ref: "cmd/nano-brain/init_test.go#TestRunNonInteractiveInit_ZeroPrompts"
        status: pass
      - kind: e2e
        ref: "manual: nano-brain init --yes writes config, doctor passes, no stdin interaction"
        status: pass
    human_judgment: false

# Metrics
duration: 45min
completed: 2026-07-03
status: complete
---

# Phase 14: Minimal zero-config init Summary

**Collapsed `nano-brain init` from a dozen consequential questions to at most one (embedding-only, when Ollama is unreachable), by replacing ad-hoc config assembly with a hand-authored, koanf-key-correct, fully commented default config template and adding a `--yes` non-interactive path.**

## Performance

- **Duration:** ~45 min
- **Started:** 2026-07-03T07:31:22Z
- **Completed:** 2026-07-03T07:52:31Z
- **Tasks:** 2 (config template + round-trip test; wizard collapse + `--yes` path)
- **Files modified:** 8 (2 created, 6 modified)

## Accomplishments

- Hand-authored `internal/config/template.go` (`fullConfigTemplate` + `RenderConfig`) replacing the old ad-hoc `fmt.Sprintf` YAML assembly — every `Config` section present with correct snake_case koanf keys, matching `getDefaults()` exactly, advanced sections commented/disabled, `context_hints` shown commented with a "written later per workspace" note, no API keys ever written
- Round-trip test (`TestRenderConfig_RoundTrip`) renders the template, loads it via `config.Load`, and compares against `Load("")` — this is the guard against the koanf/yaml key-drift bug that ruled out `yaml.Marshal(getDefaults())`
- `runInteractiveInit` collapsed: removed `stepAdvanced` (the harvester/summarization/search/watcher/logging detail-prompt sequence, ~120 lines) and `defaultAdvancedBlocks`; config building now goes through a single shared `buildRenderedConfig` helper
- `stepEmbedding` simplified to D-02.2's contract: Ollama reachable → 0 questions (silent default matching `getDefaults()`); Ollama unreachable → exactly 1 prompt (URL or blank for BM25-only); no provider-choice or concurrency prompt
- Added `nano-brain init --yes`: writes the rendered default config with zero prompts by driving the same probe helpers through a closed-stdin scanner (every internal prompt hits its EOF-means-decline branch)
- Verified live on this machine: `nano-brain init --yes` with Postgres + Ollama reachable asked 0 questions, wrote a full commented config, and `doctor` passed against it

## Task Commits

Each task was committed atomically:

1. **Task 1: Hand-authored commented full-config template + round-trip test** - `0c3f132` (feat)
2. **Task 2: Collapse init wizard to 0-3 probes + `--yes` path** - `765ddac` (feat)

**Plan metadata:** (this commit) - `docs(14): complete minimal-zero-config-init plan`

## Files Created/Modified

- `internal/config/template.go` - `fullConfigTemplate` (commented full-default YAML) + `RenderConfig(opts)` substituting probed DB URL / embedding block
- `internal/config/template_test.go` - round-trip, substitution, and no-secrets tests
- `cmd/nano-brain/init.go` - collapsed `runInteractiveInit`; new `buildRenderedConfig`/`writeConfigFile` shared helpers; new `runNonInteractiveInit` (`--yes`); removed `stepAdvanced`, `defaultAdvancedBlocks`, `promptWithDefault`
- `cmd/nano-brain/init_embedding.go` - `stepEmbedding` rewritten to the 0-or-1-question contract
- `cmd/nano-brain/init_embedding_test.go` - rewritten for the new `stepEmbedding` behavior (Ollama up/down, blank/EOF decline, provided URL, cloud caveat)
- `cmd/nano-brain/init_test.go` - updated/added tests: `TestRunInteractiveInit_ZeroQuestions`, `TestRunInteractiveInit_WritesCommentedTemplate`, `TestRunNonInteractiveInit_ZeroPrompts`, `TestBuildRenderedConfig_DatabaseAbort`
- `cmd/nano-brain/commands.go` - `runInitCmd` routes `--yes` to `runNonInteractiveInit` before the `--root` branch
- `cmd/nano-brain/ops.go` - usage line updated to mention `--yes`

## Decisions Made

- Template is a hand-authored Go string constant, not `yaml.Marshal(getDefaults())` (D-04) — proven necessary by the round-trip test, which would otherwise silently accept key-drifted output
- Round-trip baseline is `Load("")`, not raw `getDefaults()`, to avoid false-positive failures from koanf's own nil-vs-empty-slice/map normalization (a pre-existing `Load()` characteristic unrelated to the template)
- `stepEmbedding`'s down-branch offers exactly one prompt (URL or blank), no separate provider/concurrency questions — voyage/cloud setup deferred to manual config editing (D-07)
- `--yes` deliberately does not auto-provision Postgres via Docker (that offer is itself a prompt subject to EOF-decline) and does not chain into serve/register/MCP (interactive-only by design)

## Deviations from Plan

None — plan executed as specified in `14-CONTEXT.md` (D-01 through D-10). Two additional cleanup items were folded into Task 2's commit during self-review before pushing (not scope changes, just consolidation): shared `defaultProbeDBURL`/`defaultProbeEmbURL`/`defaultProbeModel` constants and a `writeConfigFile` helper were factored out to eliminate literal duplication between `runInteractiveInit` and `runNonInteractiveInit` that would otherwise let the two paths drift on defaults or file-write semantics.

## Issues Encountered

- Initial `fullConfigTemplate` draft had two default-value mismatches against `getDefaults()` (HyDE.MaxLatencyMs and Reranking.TopK/MaxLatencyMs copied from a sibling section instead of their own zero-value defaults) — caught immediately by `TestRenderConfig_RoundTrip` before any code depending on the template was written, exactly as designed.
- The round-trip test's first version compared against raw `getDefaults()` and failed on `Server.Auth.Users`/`Watcher.Workspaces` (nil vs koanf-normalized empty) and `Summarization.OutputDir` (tilde-expanded by `Load()` vs literal in `getDefaults()`) — neither was a template bug; fixed by comparing against `Load("")` instead, which isolates exactly what the template contributes.

## User Setup Required

None - no external service configuration required. `nano-brain init` (interactive or `--yes`) continues to probe Postgres/Ollama on the user's machine as before; no new env vars or manual steps introduced.

## Next Phase Readiness

- `internal/config.RenderConfig` is the single source of truth for what a freshly-initialized config looks like; any future `Config` struct field additions must be added to `fullConfigTemplate` in lockstep, or `TestRenderConfig_RoundTrip` will fail.
- The plan's `14-CONTEXT.md` note flags a phase-numbering collision with PR #533 (the curl|bash installer, also claiming "Phase 14") — renumber to 15 at ship time if #533 is still open when this phase is merged.
- Deferred per `14-CONTEXT.md`: OpenAI-compatible embedding provider (REQ-INFRA-01), auto-writing `context_hints` from repo analysis, Windows daemon serve improvements.

---
*Phase: 14-minimal-zero-config-init-collapse-the-wizard-to-0-3-probes-p*
*Completed: 2026-07-03*
