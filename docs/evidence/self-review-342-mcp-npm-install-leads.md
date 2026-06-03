# Self-Review — docs: lead with MCP / npm install -g, drop @beta (Phase 0)

**Date**: 2026-06-03
**Issue**: #342
**PR**: (to be opened after #344 merges)
**Branch**: `docs/342-mcp-npm-install-leads`
**Lane**: tiny
**Change type**: docs

## Diff scope

Three categories of change:

1. **`@beta` → `@latest` in tracked files** (tasks 1.1, 1.2, 1.4):
   - `cmd/nano-brain/client_helpers.go` — `suggestStartCommand()` returns `@latest`
   - `cmd/nano-brain/commands_test.go` — 3 test assertions updated to expect `@latest`
   - `README.md` — 3 occurrences in Quick Start

2. **README Quick Start reorder** (task 1.5):
   - Option A renamed from "Via npx (no Go required)" to "Install globally via npm (recommended for CLI)" with `npm install -g` as the primary path
   - Option B repurposed for `npx` as documented fallback with a cold-start-overhead caveat
   - Option C now holds the existing "Build from source"

3. **`.opencode/skills/nano-brain/` newly tracked** (tasks 1.3, 1.6, 1.7):
   - Brings the locally-installed skill folder (previously untracked, installed by `@nano-step/skill-manager`) into source control as the canonical copy.
   - SKILL.md ordering: already MCP-first by design (line 13 declares MCP as preferred). No structural reorder needed — task 1.6 satisfied by existing copy.
   - Added Troubleshooting section between "Common errors" and "Connection details" with all 5 spec-required entries:
     - `NANO_BRAIN_SKIP_SHA_VERIFY=1` (corp proxy / air-gapped)
     - `npm install -g --prefix ~/.local` (non-root install)
     - macOS Gatekeeper `xattr -dr com.apple.quarantine`
     - `NANO_BRAIN_MCP_URL` env var (with container + bare-metal examples + OpenCode `{env:VAR}` note)
     - `NANO_BRAIN_BIN` env var (with failure-mode table)
   - Skill folder will be kept in sync with `@nano-step/skill-manager` via the `sync-skill-to-manager` skill in a follow-up.

```
$ git diff --cached --stat
 .opencode/skills/nano-brain/AGENTS_SNIPPET.md               |  47 +++++++++
 .opencode/skills/nano-brain/SKILL.md                        | 295 +++++++++++++++++++++
 .opencode/skills/nano-brain/references/cli-cheatsheet.md    | 134 +++++++++++++++++++++
 .opencode/skills/nano-brain/references/code-intelligence.md |  93 ++++++++++++++
 .opencode/skills/nano-brain/references/config-reference.md  | 182 ++++++++++++++++++++++++++
 .opencode/skills/nano-brain/references/http-api.md          | 298 ++++++++++++++++++++++++++
 .opencode/skills/nano-brain/skill.json                      |  28 ++++++
 README.md                                                   |  31 +++++-
 cmd/nano-brain/client_helpers.go                            |   2 +-
 cmd/nano-brain/commands_test.go                             |   6 +-
 10 files changed, 1107 insertions(+), 9 deletions(-)
```

Of the 1107 insertions, 1075 are the newly-tracked skill folder (baseline + edits combined) and 32 are README + Go-test changes.

## self-review:staged-files — PASS

Audit before staging showed only intended files. No `.opencode/worktrees/` content, no `package.json` drift, no `package-lock.json`, no AI attribution metadata.

## self-review:response-shape — N/A

No API surface change. Phase 0 is docs + npm tag rename + Go test string updates.

## Acceptance criteria (from `specs/skill-distribution-docs/spec.md`)

| AC | Check | Result |
|---|---|---|
| AC1 | `grep -n '@beta' README.md` → no matches | ✅ PASS |
| AC2 | `grep -n '@beta' .opencode/skills/nano-brain/SKILL.md` → no matches | ✅ PASS |
| AC3 | `suggestStartCommand()` returns string containing `@latest`, not `@beta` | ✅ PASS (3 test assertions updated, validate:quick green) |
| AC4 | README Quick Start leads with MCP / `npm install -g`; npx is fallback | ✅ PASS (Option A: npm install -g, Option B: npx as fallback with documented overhead caveat) |
| AC5 | SKILL.md documents MCP transport before any CLI invocation; npm install -g before npx | ✅ PASS (line 13: "Agents talk to it via MCP (preferred) — CLI and HTTP are escape hatches"; new Troubleshooting npm install section precedes any npx mention in install context) |
| AC6 | SKILL.md Troubleshooting includes all 5 entries | ✅ PASS — all 5 keywords grep-matched |

```
$ for kw in NANO_BRAIN_SKIP_SHA_VERIFY 'prefix ~/.local' xattr NANO_BRAIN_MCP_URL NANO_BRAIN_BIN; do
    echo -n "$kw: "; grep -c -F "$kw" .opencode/skills/nano-brain/SKILL.md;
  done
NANO_BRAIN_SKIP_SHA_VERIFY: 1
prefix ~/.local: 1
xattr: 1
NANO_BRAIN_MCP_URL: 6
NANO_BRAIN_BIN: 5
```

## Validation ladder run from worktree

### validate:quick — PASS

```
$ go build ./...
(exit 0)

$ go test -race -short ./...
ok  github.com/nano-brain/nano-brain/cmd/nano-brain          3.875s
ok  github.com/nano-brain/nano-brain/internal/bench          (cached)
ok  github.com/nano-brain/nano-brain/internal/chunk          (cached)
... (all 22 packages green)
```

Build clean. All short-mode tests pass — including `cmd/nano-brain` which contains the 3 updated test assertions in `commands_test.go`.

## R19 — smoke:e2e not required

Change-type is `docs`. Per harness change-type table, smoke:e2e is `❌` for docs. Gate 3.12 expected to SKIP.

## R56 — Review Gate not required

Tiny lane + docs change-type: Review Gate is `❌` per harness change-type table.

## Pre-existing master failures (gates 3.3 + 3.4)

Expected to fail with the same root cause as PR #344 — pre-existing failures in `internal/harvest` + `internal/server/handlers` + lint in `_test.go` files. This PR adds 2 lines to `cmd/nano-brain/client_helpers.go` (string change) and 3 lines to `cmd/nano-brain/commands_test.go` (string change). Cannot have introduced new test/lint failures.

Will produce a companion `docs/evidence/342-pre-merge-override.md` and request user `[HARNESS-OVERRIDE]` comment on PR per R7, matching the precedent set by #331/#332 and #343/#344.

## Cross-repo coordination

The `.opencode/skills/nano-brain/` folder is now source-controlled in this repo. The same skill ships separately via `@nano-step/skill-manager` npm package. After this PR merges, run `/sync-skill-to-manager nano-brain` to push the SKILL.md updates to the skill-manager publish pipeline.

## Dependency on PR #344

This PR's `specs/skill-distribution-docs/spec.md` reference assumes #344 (the proposal PR) has merged. If #344 has not merged when this PR is opened, the reference is to an unmerged proposal — acceptable for tiny lane (no Review Gate enforces spec-link integrity) but flagged here for transparency.
