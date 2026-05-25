# Harness Init — Question Flow

This file defines the EXACT questions an orchestrating agent should ask, in
order. Each question has:

- **id**: stable identifier used in `/tmp/harness-init-answers.yaml`
- **prompt**: question text shown to user
- **type**: single-choice | text | multi-choice | yes-no
- **default-from-detect**: which detection JSON field provides the default
- **options**: enumerated choices (single/multi-choice only)
- **skip-if**: condition under which question is auto-skipped
- **emit**: YAML key/value to write into answers file

The agent MUST ask required questions in order. Optional questions may be
skipped if a sensible default is detected.

---

## Required questions

### Q1 — Project name
- **id**: `project_name`
- **type**: text
- **prompt**: "What's the project name? (used in docs + GitHub issue titles)"
- **default-from-detect**: `project_name`
- **emit**: `project_name: "<value>"`

### Q2 — Output language (locked to English for v1)
- **id**: `output_language`
- **type**: single-choice
- **prompt**: "Output language for generated docs?"
- **options**:
  - English (recommended, only supported in v1)
- **emit**: `output_language: "en"`

### Q3 — Issue tracker
- **id**: `issue_tracker`
- **type**: single-choice
- **prompt**: "Use an issue tracker for harness milestones?"
- **default-from-detect**: first item in `mcp_issue_trackers[]` if non-empty
- **options**:
  - GitHub Issues (recommended — `gh` CLI integration)
  - Linear (requires Linear MCP)
  - Jira (requires Jira MCP)
  - None — track harness work silently
- **emit**: `issue_tracker: "github" | "linear" | "jira" | "none"`

### Q4 — GitHub repo (only if Q3 = github)
- **id**: `github_repo`
- **type**: text
- **prompt**: "GitHub repo for issues? (format: owner/repo)"
- **default-from-detect**: `git.owner_repo`
- **skip-if**: `issue_tracker != "github"`
- **emit**: `github_repo: "owner/repo"`

### Q5 — GitHub account (only if Q3 = github and multiple accounts authed)
- **id**: `github_account`
- **type**: single-choice
- **prompt**: "Which GitHub account to use for issue creation?"
- **default-from-detect**: `gh.active_account`
- **options**: enumerate `gh.accounts[]`
- **skip-if**: `issue_tracker != "github" || len(gh.accounts) <= 1`
- **emit**: `github_account: "<username>"`

### Q6 — Linter / quick validate command
- **id**: `validate_quick_cmd`
- **type**: text
- **prompt**: "Command for `validate:quick` (linter + type-check + unit test)? Multiple commands separated by ` && `."
- **default-from-detect**: assembled from `linters[]` + `test_runners[]`
  - Python ruff+mypy+pytest → `ruff check . && mypy . && pytest -q -k "not db_session"`
  - TypeScript eslint+tsc+vitest → `eslint . && tsc --noEmit && vitest run`
  - Empty → leave as placeholder `# TODO: set validate:quick command`
- **emit**: `validate_quick_cmd: "<command>"`

### Q7 — Integration test command
- **id**: `validate_integration_cmd`
- **type**: text
- **prompt**: "Command for `test:integration` (real DB / external services)? Leave blank if N/A."
- **default-from-detect**:
  - pytest detected → `pytest tests/integration/ -v`
  - vitest detected → `vitest run --config vitest.integration.config.ts`
  - Empty default
- **emit**: `validate_integration_cmd: "<command or empty>"`

### Q8 — E2E test command
- **id**: `validate_e2e_cmd`
- **type**: text
- **prompt**: "Command for `test:e2e` (user-flow / browser)? Leave blank if N/A."
- **default-from-detect**:
  - playwright detected → `npx playwright test`
  - cypress detected → `npx cypress run`
  - Empty default
- **emit**: `validate_e2e_cmd: "<command or empty>"`

### Q9 — Smoke test command
- **id**: `validate_smoke_cmd`
- **type**: text
- **prompt**: "Command for smoke test (live system reachable)? E.g. `curl -fsS localhost:8000/health`. Optional."
- **default-from-detect**: none
- **emit**: `validate_smoke_cmd: "<command or empty>"`

### Q10 — OpenSpec integration
- **id**: `use_openspec`
- **type**: yes-no
- **prompt**: "Use OpenSpec for change proposals + specs? (recommended)"
- **default-from-detect**: `openspec.installed && openspec.initialized` → "yes"; else ask
- **options**:
  - Yes — required for full harness flow
  - No — use markdown-only proposals in `docs/proposals/`
- **emit**: `use_openspec: true | false`

### Q11 — Hard gates (high-risk auto-classify)
- **id**: `hard_gates`
- **type**: multi-choice (with custom add)
- **prompt**: "Which concerns auto-classify a change as high-risk? Pick all that apply, or add custom."
- **default-from-detect**: enable all six defaults
- **options** (defaults all checked):
  - auth (login, session, JWT)
  - authorization (roles, RBAC, tenant scope)
  - data model (schema, migrations, retention)
  - audit/security (logs, sensitive data)
  - external providers (payment, email, SMS, queues)
  - public API contracts (response shape, breaking changes)
  - (add custom): _free-form list_
- **emit**: `hard_gates: ["auth", "data-model", ...]`

### Q12 — Change types
- **id**: `change_types`
- **type**: multi-choice (with custom add)
- **prompt**: "Which change types should the harness recognize?"
- **default-from-detect**: enable all six defaults
- **options** (defaults all checked):
  - user-feature
  - bug-fix
  - infrastructure
  - refactor
  - docs
  - dependency-bump
  - (add custom): _e.g. ml-model-update, content-update_
- **emit**: `change_types: ["user-feature", "bug-fix", ...]`

### Q13 — File conflict policy
- **id**: `conflict_policy`
- **type**: single-choice
- **prompt**: "Default action when a target file already exists?"
- **default-from-detect**:
  - Any of `existing_files.*` is true → recommend "ask-each"
  - All false → "fresh-install"
- **options**:
  - Ask for each conflict (recommended)
  - Overwrite with `.bak.<timestamp>` backup
  - Skip — keep existing files
  - Fresh install — no existing files expected
- **emit**: `conflict_policy: "ask-each" | "backup-overwrite" | "skip" | "fresh"`

---

## Optional questions (skip with sensible defaults)

### Q14 — Lane names
- **id**: `lanes`
- **type**: text (CSV)
- **prompt**: "Lane names from lowest to highest risk? (default: tiny,normal,high-risk)"
- **default**: `tiny,normal,high-risk`
- **emit**: `lanes: ["tiny", "normal", "high-risk"]`

### Q15 — PR bot review tool
- **id**: `pr_bot`
- **type**: single-choice
- **prompt**: "Which automated PR reviewer is configured on this repo?"
- **options**:
  - None / I don't know
  - GitHub Claude PR Reviewer
  - CodeRabbit
  - Greptile
  - Custom
- **emit**: `pr_bot: "none" | "github-claude" | "coderabbit" | "greptile" | "custom"`

### Q16 — Evidence storage location
- **id**: `evidence_dir`
- **type**: text
- **prompt**: "Where to store evidence (screenshots, recordings)?"
- **default**: `docs/evidence/`
- **emit**: `evidence_dir: "<path>"`

### Q17 — Story template path
- **id**: `story_template_path`
- **type**: text
- **prompt**: "Where to put the story template?"
- **default**: `docs/templates/story.md`
- **emit**: `story_template_path: "<path>"`

### Q18 — Create labels now?
- **id**: `create_labels_now`
- **type**: yes-no
- **prompt**: "Create harness labels on GitHub now? (lane:* / change-type:* / status:*)"
- **skip-if**: `issue_tracker != "github" || !gh.authed`
- **default**: yes
- **emit**: `create_labels_now: true | false`

### Q19 — Update AGENTS.md
- **id**: `update_agents_md`
- **type**: yes-no
- **prompt**: "AGENTS.md detected. Append a harness section pointing to docs/HARNESS.md?"
- **skip-if**: `!existing_files.agents_md`
- **default**: yes
- **emit**: `update_agents_md: true | false`

### Q20 — Dogfood first issue
- **id**: `dogfood_issue`
- **type**: yes-no
- **prompt**: "Create the first tracking issue (titled 'harness: bootstrap complete') to verify integration?"
- **skip-if**: `issue_tracker != "github" || !gh.authed`
- **default**: yes
- **emit**: `dogfood_issue: true | false`

---

## Answers file schema (`/tmp/harness-init-answers.yaml`)

```yaml
# Required
project_name: "capy-home"
output_language: "en"
issue_tracker: "github"
github_repo: "kokorolx/capy-home"
github_account: "kokorolx"
validate_quick_cmd: "ruff check . && mypy . && pytest -q -k 'not db_session'"
validate_integration_cmd: "pytest tests/integration/ -v"
validate_e2e_cmd: "cd web && npx playwright test"
validate_smoke_cmd: "curl -fsS localhost:8000/health"
use_openspec: true
hard_gates:
  - auth
  - authorization
  - data-model
  - audit-security
  - external-providers
  - public-api-contracts
change_types:
  - user-feature
  - bug-fix
  - infrastructure
  - refactor
  - docs
  - dependency-bump
conflict_policy: "ask-each"

# Optional
lanes: ["tiny", "normal", "high-risk"]
pr_bot: "github-claude"
evidence_dir: "docs/evidence"
story_template_path: "docs/templates/story.md"
create_labels_now: true
update_agents_md: true
dogfood_issue: true

# Auto-filled from detection
_detected:
  cwd: "/path/to/project"
  in_container: false
  openspec_installed: true
  openspec_initialized: true
```
