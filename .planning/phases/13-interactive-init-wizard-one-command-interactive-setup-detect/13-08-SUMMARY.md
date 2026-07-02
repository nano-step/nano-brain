---
phase: 13-interactive-init-wizard-one-command-interactive-setup-detect
plan: 08
subsystem: docs
tags: [docs, setup, readme, wizard]

# Dependency graph
requires:
  - phase: 13-07
    provides: runInteractiveInit orchestrator (the real one-command wizard flow the docs must describe)
provides:
  - "docs/SETUP_AGENT.md Quick setup (one command) section — prerequisites → npm install -g @nano-step/nano-brain → nano-brain init → verify, describing the 13-07 wizard flow"
  - "docs/SETUP_AGENT.md Manual setup / troubleshooting appendix — full Step 1..10 manual flow, Docker command, MCP config snippets, ?workspace= binding note, doctor failure table, Troubleshooting, VPS/team path preserved intact"
  - "README.md Start section — npm install -g @nano-step/nano-brain && nano-brain init, with a pointer to docs/SETUP_AGENT.md for the manual/VPS path"
affects: [interactive-init-wizard (feature + docs now complete)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Docs-only plan pattern: quick-setup summary at top, full manual detail preserved verbatim under an appendix heading — used to retire a superseded flow without deleting content still valid for VPS/no-Docker/Windows users"

key-files:
  created: []
  modified:
    - docs/SETUP_AGENT.md
    - README.md

key-decisions:
  - "The appendix heading is literally '## Manual setup / troubleshooting' (task grep gate only required the substring '## Manual setup', which this satisfies) — chosen to fold the existing 'Troubleshooting' framing into the same appendix label the plan specified."
  - "Server port question is not mentioned in the Quick setup description (matches 13-07's actual orchestrator — port is a fixed default, not a prompt) to keep the docs accurate to the real wizard rather than a re-derived guess."
  - "Windows caveat placed as a blockquote note directly under the Initialize beat, linking forward to the appendix Step 7 anchor, rather than as a separate section — keeps the caveat local to where a Windows user would hit it."

patterns-established: []

requirements-completed: [D-18]

coverage:
  - id: D1
    description: "docs/SETUP_AGENT.md leads with one-command flow (prerequisites -> npm install -> nano-brain init -> verify) describing the 13-07 wizard"
    requirement: "D-18"
    verification:
      - kind: other
        ref: "grep -c 'npm install -g @nano-step/nano-brain' docs/SETUP_AGENT.md == 2; grep -c 'nano-brain init' docs/SETUP_AGENT.md == 6"
        status: pass
    human_judgment: false
  - id: D2
    description: "Manual Step 1..10 + VPS/team path preserved intact under a Manual setup appendix"
    requirement: "D-18"
    verification:
      - kind: other
        ref: "grep -c '## Manual setup' docs/SETUP_AGENT.md == 1; grep -ci 'VPS' docs/SETUP_AGENT.md == 6; grep -c 'pgvector/pgvector:pg17' == 2; grep -c 'nanobrain-pg' == 4"
        status: pass
    human_judgment: false
  - id: D3
    description: "README Start section is the one-command flow, replacing the old docker/serve/init-root sequence"
    requirement: "D-18"
    verification:
      - kind: other
        ref: "grep -c 'npm install -g @nano-step/nano-brain && nano-brain init' README.md == 1; awk-scoped grep 'serve -d' inside Start section == 0; grep -c 'docs/SETUP_AGENT.md' README.md == 1"
        status: pass
    human_judgment: false
  - id: D4
    description: "No Go source, test, or config touched — docs-only change"
    requirement: "D-18"
    verification:
      - kind: automated
        ref: "go build ./cmd/nano-brain/ succeeds; git diff --name-only HEAD~2 HEAD shows only README.md and docs/SETUP_AGENT.md"
        status: pass
    human_judgment: false

duration: ~20min
completed: 2026-07-02
status: complete
---

# Phase 13 Plan 08: SETUP_AGENT.md and README One-Command Docs Summary

**Rewrote `docs/SETUP_AGENT.md` to lead with the Phase 13 one-command `nano-brain init` wizard flow (prerequisites → npm install → init → verify), preserved the full 10-step manual + VPS/team instructions under a `## Manual setup / troubleshooting` appendix, and updated README's Start section to the two-command one-liner — closing D-18, the docs decision that had no implementing task until now.**

## Performance

- **Duration:** ~20 min
- **Tasks:** 2 (docs-only, no code)
- **Files modified:** 2 (docs/SETUP_AGENT.md, README.md)

## Accomplishments

- `docs/SETUP_AGENT.md` now opens with a `## Quick setup (one command)` section presenting four beats — prerequisites check (Node/Docker/optionally Ollama), `npm install -g @nano-step/nano-brain`, `nano-brain init`, and verify — that accurately describes the real 13-07 `runInteractiveInit` orchestrator: keep/overwrite gate, database detect/provision/remote, optional embeddings, write+doctor, serve, register, MCP client config, and the "restart your AI client" summary line.
- A one-line Windows caveat under the Initialize beat notes that automatic server auto-start is not yet available on Windows and points to the manual appendix's Step 7 for the fallback `nano-brain serve` instruction.
- The entire prior content — Step 1 through Step 10 (Node, Docker, Ollama, the exact `docker run … nanobrain-pg … pgvector/pgvector:pg17` command, npm install, doctor + failure table, serve, register, MCP client config snippets for Claude Code/OpenCode/other clients, the `?workspace=` binding note, end-to-end verify), the Troubleshooting section, and the VPS / team setup (Path 2) — survives unchanged under a new `## Manual setup / troubleshooting` appendix heading.
- README's `### Start` section now shows the exact D-18 string `npm install -g @nano-step/nano-brain && nano-brain init`, with a one-line prose description of what the wizard does and a pointer to `docs/SETUP_AGENT.md` for the manual/VPS/Windows path. The old three-step `docker run` + `nano-brain serve -d` + `nano-brain init --root=…` sequence is no longer the primary Start block.
- `### Install`, the `## Quick Start` MCP config snippet, and the `## Configuration` YAML example in README.md were left untouched, per the plan's scope.
- Verified `go build ./cmd/nano-brain/` still succeeds and `git diff --name-only` for both commits shows only `README.md` and `docs/SETUP_AGENT.md` — no Go source, test, or config was touched.

## Task Commits

1. **Task 1: Rewrite docs/SETUP_AGENT.md — one-command flow on top, manual steps to appendix (D-18)** - `6d18e3e` (docs)
2. **Task 2: Update README Start section to the one-command flow (D-18)** - `bf49b47` (docs)

## Files Created/Modified

- `docs/SETUP_AGENT.md` — added `## Quick setup (one command)` section at top; renamed remaining detail under `## Manual setup / troubleshooting`, converting former `##` step headings to `###` so they nest under the new appendix; all step content, Docker command, MCP snippets, `?workspace=` note, doctor table, Troubleshooting, and VPS/team path preserved verbatim.
- `README.md` — `### Start` code block replaced with `npm install -g @nano-step/nano-brain && nano-brain init`; added a prose line describing the wizard and a link to `docs/SETUP_AGENT.md`.

## Decisions Made

- See `key-decisions` in frontmatter. No architectural decisions were needed — this is a pure content-restructuring plan following the plan's explicit task instructions.

## Deviations from Plan

None — plan executed exactly as written. All acceptance criteria grep gates and the `go build` sanity check pass with margin (e.g., `nano-brain init` appears 6 times in SETUP_AGENT.md, comfortably exceeding the "≥ 1, appears in top quick-setup section" bar).

## Issues Encountered

- The repo's OMC pre-commit review gate (`/simplify` + `/code-review` + sentinel) did not block either commit in this run — both commits passed the harness-check.sh validation ladder (4/4 PASS) on first attempt.

## User Setup Required

None — no external service configuration required. This plan is documentation-only.

## Next Phase Readiness

- D-18 is now closed: the setup docs describe the actual Phase 13 wizard behavior, and the manual/VPS/Windows path remains fully documented and reachable.
- Phase 13 (Plans 13-01..13-08) is functionally and documentation-complete: the interactive init wizard works end-to-end (per 13-07-SUMMARY) and the setup guide + README now accurately reflect it.
- No blockers for phase-level verification or closeout.

---

*Phase: 13-interactive-init-wizard-one-command-interactive-setup-detect*
*Completed: 2026-07-02*

## Self-Check: PASSED

- FOUND: docs/SETUP_AGENT.md
- FOUND: README.md
- FOUND commit: 6d18e3e (docs(13-08): rewrite SETUP_AGENT.md around one-command init wizard)
- FOUND commit: bf49b47 (docs(13-08): update README Start section to one-command init flow)
