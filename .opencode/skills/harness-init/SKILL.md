---
name: harness-init
description: |
  Bootstrap a Sisyphus-compatible engineering harness in any project. Generates
  HARNESS.md, FEATURE_INTAKE.md, story template, HARNESS_BACKLOG.md, optional
  OpenSpec config, GitHub issue tracking labels, and validation ladder
  customized to the project's stack via guided Q&A.

  Use this skill when:
  - Starting a new project that needs spec-driven, risk-classified workflow
  - Adopting the harness pattern in an existing project
  - Standardizing how AI agents propose, implement, review, and ship changes
  - Setting up issue tracking + change-type labels on GitHub / Linear / Jira

  Triggers on: "init harness", "set up harness", "/harness-init", "add Sisyphus
  workflow", "bootstrap project workflow", "install harness in this repo".

compatibility: OpenCode with bash + git. Optional: gh CLI, openspec CLI, MCP issue trackers.
---

# Harness Init

Generate a production-ready engineering harness in the current project.

## What the harness gives the team

- **Risk classification** (tiny / normal / high-risk lanes) at intake time
- **Spec-driven flow**: proposal → design → deep-design gap review → specs → implement
- **Mandatory validation ladder** with concrete commands per lane
- **User-flow testing** + review gate before archive
- **PR + bot review loop**
- **Issue tracking** integrated at every milestone
- **Evidence trail**: tests, screenshots, decisions, friction backlog

See `docs/PHILOSOPHY.md` (in this skill folder) for the full rationale.

## How to use this skill

**You are the orchestrator.** Walk the user through the install flow below.
Never apply files without confirmation. Always show what will change before
changing it.

### Step 1 — Detect the environment

Run the detection script ONCE at the start. It outputs JSON describing the
project's stack, git state, and tools available.

```bash
bash ~/.config/opencode/skills/harness-init/scripts/detect_environment.sh "$PWD"
```

Parse the JSON. Use detected values as **defaults** in the Q&A. Never
auto-apply without showing the user the detected values and getting confirmation.

The JSON contains:
- `cwd`, `git_repo`, `git_remote`, `default_branch`
- `gh_authed`, `gh_active_account`, `gh_accounts[]`
- `openspec_installed`, `openspec_initialized`
- `mcp_issue_trackers[]` (github / linear / jira / none)
- `stack_signals[]` (python, typescript, go, rust, etc. detected from config files)
- `existing_files{}` (HARNESS.md, AGENTS.md, FEATURE_INTAKE.md presence)
- `test_runners[]` (pytest, vitest, jest, playwright, etc.)

### Step 2 — Walk the Q&A

Use OpenCode's `question` tool for each question below, **one at a time**.
For each question:
1. Show the detected default (if any) as the first option labeled "Use detected: X"
2. Show alternatives
3. Always include "Skip — accept default" as an option

**The complete question list lives in `scripts/questions.md`.** Read it first.
You MUST cover all required questions before invoking install.sh. Optional
questions can be skipped using detected defaults.

### Step 3 — Build the answers config

After each answer, append to `/tmp/harness-init-answers.yaml` in this format:

```yaml
project_name: "<value>"
issue_tracker: "github"  # or linear, jira, none
github_repo: "owner/repo"
# ... etc
```

Show the full config to the user BEFORE applying.

### Step 4 — Show plan, get confirmation

Print:
1. List of files that will be created or modified
2. For each existing file: action [Overwrite / Skip / Show diff first]
3. List of GitHub labels that will be created (if issue tracker = github)
4. Final confirmation: "Apply now? [Y/n]"

If user requests "Show diff first" for a file, run:
```bash
bash ~/.config/opencode/skills/harness-init/scripts/preview_diff.sh \
  --template <name> --target <path> --config /tmp/harness-init-answers.yaml
```

### Step 5 — Apply

```bash
bash ~/.config/opencode/skills/harness-init/scripts/install.sh \
  --config /tmp/harness-init-answers.yaml \
  --target "$PWD"
```

The script:
- Renders templates with answers substituted (`${VAR}` placeholders)
- Per-file conflict resolution honoring user's choices from Step 4
- Creates `docs/`, `docs/templates/`, `docs/evidence/` directories
- Initializes `openspec init` if user chose to include OpenSpec
- Creates GitHub labels (if tracker = github and gh authed)
- Prints summary of changes

### Step 6 — Verify

```bash
bash ~/.config/opencode/skills/harness-init/scripts/verify_install.sh "$PWD"
```

Reports:
- All expected files present? ✓ / ✗
- Labels created? ✓ / ✗ (or 'skipped')
- Cross-references valid? ✓ / ✗ (HARNESS.md references story template, etc.)

If verify fails, offer to re-run install or print specific fix commands.

### Step 7 — Dogfood

Offer the user: "Create the first tracking issue to verify everything works
end-to-end?"

If yes: create a GitHub issue titled "harness: bootstrap complete" with
labels `lane:tiny` + `change-type:docs`, linking to the just-generated
`docs/HARNESS.md`. This validates issue tracker integration.

## Question flow (detail in scripts/questions.md)

Required questions (cannot skip):
1. Project name
2. Issue tracker [github / linear / jira / none]
3. If github → repo (owner/repo), confirm gh account
4. Primary language(s)
5. validate:quick command (linter + type-check + unit test)
6. Test runner for user-flow tests
7. Hard gates list (default: auth, data model, API contract; user may add)
8. Change types (default 6 types; user may add domain-specific)
9. OpenSpec integration? [yes / no — if yes and not installed, print install command]
10. Output language? (default: English)
11. Existing file conflict policy default [ask-each / overwrite / skip]

Optional / advanced:
12. PR bot review tool? [github-claude / coderabbit / greptile / none]
13. Custom lane names? (default: tiny / normal / high-risk)
14. Evidence directory location (default: docs/evidence/)
15. Story template location (default: docs/templates/story.md)

## Important rules

- **NEVER apply files without explicit confirmation.** Always Step 4 before
  Step 5.
- **NEVER overwrite without backup option.** Default to creating
  `.bak.<timestamp>` on every overwrite.
- **NEVER call `gh label create` without first checking `gh auth status`.**
  If unauthed, print the commands for user to run manually.
- **NEVER skip Step 1 (detect).** Detection prevents asking the user things
  the environment already tells us.
- **PREFER auto-detected defaults over asking.** If `git remote` returns a
  single GitHub URL and `gh auth status` shows one active account, use them
  silently and only confirm the consolidated result.
- **If OpenSpec is missing and user wants it**, print:
  ```
  npm install -g @openspec/cli
  # or
  brew install openspec
  ```
  Then offer to retry detection.
- **Idempotent re-runs**: running the skill twice on the same project should
  detect the existing install and offer "Upgrade to vX" or "Reconfigure" or
  "Exit".

## Output

After Step 7, print a final summary:

```
Harness installed at <cwd>.

Files created/updated:
  - docs/HARNESS.md
  - docs/FEATURE_INTAKE.md
  - docs/templates/story.md
  - docs/HARNESS_BACKLOG.md
  - docs/evidence/README.md
  - [if OpenSpec] openspec/config.yaml updated
  - [if backup] *.bak.<timestamp> files

GitHub labels created: 13 (lane:* + change-type:* + status:*)
GitHub issue created: #N — kokorolx/capy-home#N

Next steps:
  1. Read docs/HARNESS.md
  2. Customize docs/HARNESS_BACKLOG.md with project-specific friction
  3. Use /opsx-propose to create first OpenSpec change
  4. First user request → create issue → follow the flow
```
