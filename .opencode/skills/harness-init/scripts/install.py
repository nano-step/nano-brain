#!/usr/bin/env python3
"""Harness installer — render templates and apply to target project.

Usage:
    install.py --config <answers.yaml> --target <project-dir> [--dry-run]

Exit codes:
    0 — success
    1 — aborted by user or missing precondition
    2 — render or write error
"""
from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import re
import shutil
import subprocess
import sys
from pathlib import Path
from typing import Any

SKILL_DIR = Path(__file__).resolve().parent.parent
TEMPLATES_DIR = SKILL_DIR / "templates"
VERSION = "0.1.0"


def parse_yaml(path: Path) -> dict[str, Any]:
    """Tiny YAML parser — supports scalars, lists, nested dicts. No anchors."""
    try:
        import yaml  # type: ignore
        return yaml.safe_load(path.read_text()) or {}
    except ImportError:
        pass

    text = path.read_text()
    out: dict[str, Any] = {}
    stack: list[tuple[int, Any]] = [(-1, out)]
    for raw in text.splitlines():
        line = raw.rstrip()
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        indent = len(line) - len(line.lstrip())
        while stack and indent <= stack[-1][0] and len(stack) > 1:
            stack.pop()
        parent = stack[-1][1]
        stripped = line.strip()
        if stripped.startswith("- "):
            val = stripped[2:].strip().strip('"').strip("'")
            if isinstance(parent, list):
                parent.append(val)
            else:
                raise ValueError(f"List item outside list: {line!r}")
            continue
        if ":" in stripped:
            k, _, v = stripped.partition(":")
            k = k.strip()
            v = v.strip().strip('"').strip("'")
            if not v:
                new: list[Any] | dict[str, Any] = []
                parent[k] = new
                stack.append((indent, new))
            elif v.lower() == "true":
                parent[k] = True
            elif v.lower() == "false":
                parent[k] = False
            elif v.isdigit():
                parent[k] = int(v)
            else:
                parent[k] = v
    return out


def build_vars(cfg: dict[str, Any]) -> dict[str, str]:
    """Build all ${VAR} substitutions from the answers config."""
    issue_tracker = cfg.get("issue_tracker", "none")
    use_openspec = cfg.get("use_openspec", False)
    github_repo = cfg.get("github_repo", "")
    validate_quick = cfg.get("validate_quick_cmd", "# TODO: set validate:quick command")
    validate_integration = cfg.get("validate_integration_cmd", "# TODO: set integration test command") or "# N/A"
    validate_e2e = cfg.get("validate_e2e_cmd", "# TODO: set e2e command") or "# N/A"
    validate_smoke = cfg.get("validate_smoke_cmd", "# TODO: set smoke command") or "# N/A"
    pr_bot = cfg.get("pr_bot", "none")
    evidence_dir = cfg.get("evidence_dir", "docs/evidence")
    story_template_path = cfg.get("story_template_path", "docs/templates/story.md")
    project_name = cfg.get("project_name", "project")
    hard_gates = cfg.get("hard_gates", []) or []
    change_types = cfg.get("change_types", []) or []

    has_issue_tracker = issue_tracker != "none"

    v: dict[str, str] = {
        "PROJECT_NAME": project_name,
        "VALIDATE_QUICK_CMD": validate_quick,
        "VALIDATE_QUICK_HINT": validate_quick.split(" && ")[0].split()[0] if validate_quick else "validate:quick",
        "VALIDATE_INTEGRATION_CMD": validate_integration,
        "VALIDATE_E2E_CMD": validate_e2e,
        "VALIDATE_SMOKE_CMD": validate_smoke,
        "USER_FLOW_BOT_CMD": cfg.get("user_flow_bot_cmd", validate_integration),
        "USER_FLOW_WEB_CMD": validate_e2e,
        "USER_FLOW_API_CMD": validate_integration,
        "EVIDENCE_DIR": evidence_dir,
        "STORY_TEMPLATE_PATH": story_template_path,
        "HARD_GATES_LIST": ", ".join(hard_gates) if hard_gates else "auth, data model, API contract",
        "HARD_GATES_INLINE": ", ".join(hard_gates) if hard_gates else "auth, data model, API contract",
        "CHANGE_TYPES_INLINE": " | ".join(f"`{t}`" for t in change_types) if change_types else "`user-feature` | `bug-fix` | `infrastructure` | `refactor` | `docs` | `dependency-bump`",
    }

    if issue_tracker == "github":
        repo = github_repo or "owner/repo"
        v["ISSUE_NODE"] = (
            f"┌─────────────────────┐\n"
            f"│  GitHub Issue       │  gh issue create --repo {repo}\n"
            f"│  (skeleton)         │  title from user intent, lane TBD, body = raw request\n"
            f"│                     │  → returns #N (tracker for the whole flow)\n"
            f"└────────┬────────────┘"
        )
        v["ISSUE_INTAKE_UPDATE"] = f"\n│                     │  → update issue: add lane:* + change-type:* labels"
        v["ISSUE_TINY_CLOSE"] = f" + close issue #N (single comment with diff)"
        v["ISSUE_PROPOSE_UPDATE"] = f"\n│                     │  → update issue #N: link proposal location"
        v["ISSUE_SPECS_UPDATE"] = f"\n│                     │  → update issue #N: paste synthesis + acceptance criteria"
        v["ISSUE_IMPL_UPDATE"] = f"\n│                     │  → update issue #N: tick off tasks as completed"
        v["ISSUE_REVIEW_UPDATE"] = f"\n│                     │  → update issue #N: paste Review Verdict + evidence table"
        v["ISSUE_CLOSE_UPDATE"] = f"\n│                     │  → close issue #N with link to merged PR"
        v["PR_LINK_ISSUE"] = f" (gh pr create --body 'Closes #N')"
        v["ISSUE_TRACKER_LABEL"] = "GitHub Issue"
        v["ISSUE_TRACKER_REF"] = repo
        v["ISSUE_FORBIDDEN_LINES"] = (
            "9. **Starting work without a GitHub issue.** Every new user request (except\n"
            "   pure conversational queries) must have a GitHub issue created BEFORE\n"
            "   classification. Working without an issue ID = invisible work.\n"
            "10. **Stale issue.** If implementation progresses but the issue isn't updated\n"
            "    at the milestones in § GitHub Issue Tracking, the change is in violation."
        )
        v["ISSUE_TRACKING_SECTION"] = github_issue_tracking_section(repo)
        v["EVIDENCE_PROVIDER_HINT"] = (
            "GitHub renders relative paths in issue comments when the comment is created\n"
            "via the repo (not when posted from a fork)."
        )
    elif issue_tracker in ("linear", "jira"):
        tracker_name = "Linear" if issue_tracker == "linear" else "Jira"
        v["ISSUE_NODE"] = (
            f"┌─────────────────────┐\n"
            f"│  {tracker_name} Issue      │  create issue via {tracker_name} MCP\n"
            f"│  (skeleton)         │  title from user intent, lane TBD\n"
            f"│                     │  → returns issue ID (tracker for the whole flow)\n"
            f"└────────┬────────────┘"
        )
        for k in ("ISSUE_INTAKE_UPDATE", "ISSUE_PROPOSE_UPDATE", "ISSUE_SPECS_UPDATE",
                  "ISSUE_IMPL_UPDATE", "ISSUE_REVIEW_UPDATE", "ISSUE_CLOSE_UPDATE"):
            v[k] = f"\n│                     │  → update {tracker_name} issue at this milestone"
        v["ISSUE_TINY_CLOSE"] = f" + close {tracker_name} issue"
        v["PR_LINK_ISSUE"] = f" (reference {tracker_name} issue in PR body)"
        v["ISSUE_TRACKER_LABEL"] = f"{tracker_name}"
        v["ISSUE_TRACKER_REF"] = f"via {tracker_name} MCP"
        v["ISSUE_FORBIDDEN_LINES"] = (
            f"9. **Starting work without a {tracker_name} issue.** Every new user request\n"
            f"   (except pure conversational queries) must have a {tracker_name} issue\n"
            f"   created BEFORE classification."
        )
        v["ISSUE_TRACKING_SECTION"] = generic_tracking_section(tracker_name)
        v["EVIDENCE_PROVIDER_HINT"] = f"Link evidence files in {tracker_name} comments using project-relative paths."
    else:
        for k in ("ISSUE_NODE", "ISSUE_INTAKE_UPDATE", "ISSUE_PROPOSE_UPDATE",
                  "ISSUE_SPECS_UPDATE", "ISSUE_IMPL_UPDATE", "ISSUE_REVIEW_UPDATE",
                  "ISSUE_CLOSE_UPDATE", "ISSUE_TINY_CLOSE", "PR_LINK_ISSUE",
                  "ISSUE_FORBIDDEN_LINES", "ISSUE_TRACKING_SECTION"):
            v[k] = ""
        v["ISSUE_NODE"] = "(no issue tracker — work tracked silently)"
        v["ISSUE_TRACKER_LABEL"] = "No issue tracker"
        v["ISSUE_TRACKER_REF"] = "tracking via story packets only"
        v["EVIDENCE_PROVIDER_HINT"] = ""

    if use_openspec:
        v["PROPOSE_TOOL_LINE"] = '  openspec new change "<name>" → proposal.md + design.md + tasks.md'
        v["ARCHIVE_CMD"] = 'openspec archive "<name>"'
        v["OPENSPEC_INTEGRATION_SECTION"] = openspec_integration_section()
        v["SOURCE_HIERARCHY_SPEC_LAYER"] = (
            "        └── OpenSpec change proposal (openspec/changes/<name>/)\n"
            "              ├── proposal.md   — what and why\n"
            "              ├── design.md     — how (architecture, data model, API shape)\n"
            "              ├── specs/        — one spec per behavior slice\n"
            "              └── tasks.md      — implementation checklist"
        )
        v["SPEC_REF"] = "OpenSpec change"
    else:
        v["PROPOSE_TOOL_LINE"] = "  write docs/proposals/<name>/{proposal,design,tasks}.md"
        v["ARCHIVE_CMD"] = "move docs/proposals/<name>/ → docs/proposals/archive/<date>-<name>/"
        v["OPENSPEC_INTEGRATION_SECTION"] = ""
        v["SOURCE_HIERARCHY_SPEC_LAYER"] = (
            "        └── Change proposal (docs/proposals/<name>/)\n"
            "              ├── proposal.md   — what and why\n"
            "              ├── design.md     — how\n"
            "              └── tasks.md      — implementation checklist"
        )
        v["SPEC_REF"] = "change proposal"

    v["DEEP_DESIGN_BLOCK"] = (
        "┌─────────────────────┐\n"
        "│  Deep-Design        │  spawn deep-design agent → find gaps, ambiguities, risks\n"
        "│  Gap Analysis       │  (Metis + Oracle in parallel → cross-critique → synthesis)\n"
        "└────────┬────────────┘\n"
        "         │\n"
        "         ├── gaps found ──► revise proposal/design ──► re-run deep-design\n"
        "         │\n"
        "         ▼  clean pass\n"
    )
    v["DEEP_DESIGN_SECTION"] = deep_design_section()

    if pr_bot == "github-claude":
        v["PR_BOT_DESC"] = "automated review by GitHub Claude PR Reviewer"
        v["PR_BOT_NAME"] = "Claude PR bot"
        v["PR_BOT_LOOP_DESC"] = "The PR triggers automated bot review (Claude PR reviewer)."
    elif pr_bot == "coderabbit":
        v["PR_BOT_DESC"] = "automated review by CodeRabbit"
        v["PR_BOT_NAME"] = "CodeRabbit"
        v["PR_BOT_LOOP_DESC"] = "The PR triggers automated bot review (CodeRabbit)."
    elif pr_bot == "greptile":
        v["PR_BOT_DESC"] = "automated review by Greptile"
        v["PR_BOT_NAME"] = "Greptile"
        v["PR_BOT_LOOP_DESC"] = "The PR triggers automated bot review (Greptile)."
    elif pr_bot == "none":
        v["PR_BOT_DESC"] = "human review (no bot configured)"
        v["PR_BOT_NAME"] = "Reviewer"
        v["PR_BOT_LOOP_DESC"] = "No PR bot configured. Human reviewer follows the same loop manually."
    else:
        v["PR_BOT_DESC"] = "automated PR review"
        v["PR_BOT_NAME"] = "PR bot"
        v["PR_BOT_LOOP_DESC"] = "The PR triggers your configured automated reviewer."

    v.update(build_feature_intake_vars(cfg, has_issue_tracker, use_openspec, github_repo))
    v.update(build_story_vars(cfg, has_issue_tracker, use_openspec))

    return v


def build_feature_intake_vars(
    cfg: dict, has_tracker: bool, use_openspec: bool, github_repo: str
) -> dict[str, str]:
    project_name = cfg.get("project_name", "project")
    validate_quick = cfg.get("validate_quick_cmd", "validate:quick")
    hard_gates = cfg.get("hard_gates", []) or [
        "auth", "authorization", "data model", "audit/security",
        "external providers", "public API contracts"
    ]

    v: dict[str, str] = {}

    if has_tracker and cfg.get("issue_tracker") == "github":
        repo = github_repo or "owner/repo"
        v["INTAKE_ISSUE_STEP"] = f"Create GitHub issue (skeleton)        ← gh issue create --repo {repo}\n    |                                    (skip only for pure questions / read-only)"
        v["INTAKE_LABEL_STEP"] = "    |\n    v\nUpdate issue with lane + change-type labels"
        v["STEP_0_SECTION"] = step_0_section_github(repo)
        v["TINY_ISSUE_CLOSE_STEP"] = "5. Close issue #N with single comment containing the diff + validate output."
        v["NORMAL_ISSUE_PROPOSE_STEP"] = f"   → `gh issue comment <N> --body \"Proposal: <location>\"`"
        v["NORMAL_ISSUE_DEEPDESIGN_STEP"] = f"   → `gh issue comment <N> --body \"Deep-design synthesis: <gaps + resolutions>\"`"
        v["NORMAL_ISSUE_LINK_STEP"] = f"   Set `github_issue: #N` in story frontmatter."
        v["NORMAL_ISSUE_CRITERIA_STEP"] = f"   → `gh issue comment <N> --body \"Acceptance criteria: <paste from spec>\"`"
        v["NORMAL_ISSUE_IMPL_STEP"] = (
            "   → Issue update: tick off tasks in Progress checklist as completed.\n"
            "   → For multi-day work, post status comment every ~3 substantive commits."
        )
        v["NORMAL_ISSUE_USERFLOW_STEP"] = f"   → `gh issue comment <N> --body \"User-flow test PASS: <command + output>\"`"
        v["NORMAL_ISSUE_REVIEW_STEP"] = f"   → `gh issue comment <N> --body \"Review Gate: PASS — <verdict table>\"`"
        v["NORMAL_ISSUE_CLOSE_STEP"] = "   Merge PR → issue auto-closes via `Closes #N`. Verify it closed."
        v["HIGHRISK_ISSUE_PROPOSE_STEP"] = "   → Issue: link proposal + apply `lane:high-risk` label."
        v["HIGHRISK_ISSUE_DEEPDESIGN_STEP"] = "   → Issue: paste full Metis + Oracle synthesis as a comment."
        v["HIGHRISK_ISSUE_HUMAN_STEP"] = "   → Issue: comment \"Human approved: <date> — proceeding to specs\"."
        v["HIGHRISK_ISSUE_REVIEW_STEP"] = "   → Issue: paste full review-work verdict + per-criterion evidence."
        v["HIGHRISK_ISSUE_CLOSE_STEP"] = "   Merge PR → issue auto-closes via `Closes #N`. Verify it closed."
        v["PR_LINK_TEXT"] = " with `Closes #N` in body"
    elif has_tracker:
        tracker = cfg.get("issue_tracker", "")
        v["INTAKE_ISSUE_STEP"] = f"Create {tracker} issue (skeleton via MCP)\n    |"
        v["INTAKE_LABEL_STEP"] = "    |\n    v\nUpdate issue with lane + change-type metadata"
        v["STEP_0_SECTION"] = step_0_section_generic(tracker)
        v["TINY_ISSUE_CLOSE_STEP"] = f"5. Close {tracker} issue with diff + validate output."
        for k in ("NORMAL_ISSUE_PROPOSE_STEP", "NORMAL_ISSUE_DEEPDESIGN_STEP", "NORMAL_ISSUE_LINK_STEP",
                  "NORMAL_ISSUE_CRITERIA_STEP", "NORMAL_ISSUE_IMPL_STEP", "NORMAL_ISSUE_USERFLOW_STEP",
                  "NORMAL_ISSUE_REVIEW_STEP", "NORMAL_ISSUE_CLOSE_STEP",
                  "HIGHRISK_ISSUE_PROPOSE_STEP", "HIGHRISK_ISSUE_DEEPDESIGN_STEP",
                  "HIGHRISK_ISSUE_HUMAN_STEP", "HIGHRISK_ISSUE_REVIEW_STEP", "HIGHRISK_ISSUE_CLOSE_STEP"):
            v[k] = f"   → Update {tracker} issue at this milestone."
        v["PR_LINK_TEXT"] = f" referencing {tracker} issue"
    else:
        v["INTAKE_ISSUE_STEP"] = "(no issue tracker step — proceed to classification)"
        v["INTAKE_LABEL_STEP"] = ""
        v["STEP_0_SECTION"] = ""
        for k in ("TINY_ISSUE_CLOSE_STEP", "NORMAL_ISSUE_PROPOSE_STEP", "NORMAL_ISSUE_DEEPDESIGN_STEP",
                  "NORMAL_ISSUE_LINK_STEP", "NORMAL_ISSUE_CRITERIA_STEP", "NORMAL_ISSUE_IMPL_STEP",
                  "NORMAL_ISSUE_USERFLOW_STEP", "NORMAL_ISSUE_REVIEW_STEP", "NORMAL_ISSUE_CLOSE_STEP",
                  "HIGHRISK_ISSUE_PROPOSE_STEP", "HIGHRISK_ISSUE_DEEPDESIGN_STEP",
                  "HIGHRISK_ISSUE_HUMAN_STEP", "HIGHRISK_ISSUE_REVIEW_STEP", "HIGHRISK_ISSUE_CLOSE_STEP"):
            v[k] = ""
        v["PR_LINK_TEXT"] = ""

    if use_openspec:
        v["PROPOSE_CMD_NORMAL"] = (
            '```bash\n   openspec new change "<kebab-name>"\n   ```\n'
            '   Write `proposal.md` and `design.md`.'
            + (" Include `Tracking: #N` at top of `proposal.md`." if has_tracker else "")
        )
        v["SPECS_CMD_NORMAL"] = (
            '```bash\n   openspec instructions specs --change "<name>"\n'
            '   openspec validate "<name>" --strict\n   ```'
        )
        v["SPECS_CMD_HIGHRISK"] = (
            '```bash\n   openspec instructions specs --change "<name>"\n'
            '   openspec validate "<name>" --strict\n   ```'
        )
        v["IMPLEMENT_CMD"] = "```\n   /opsx-apply\n   ```"
        v["ARCHIVE_CMD_NORMAL"] = '```bash\n   openspec archive "<name>"\n   ```'
        v["ARCHIVE_CMD_HIGHRISK"] = '```bash\n   openspec archive "<name>"\n   ```'
    else:
        v["PROPOSE_CMD_NORMAL"] = (
            "Create `docs/proposals/<name>/` with `proposal.md`, `design.md`, `tasks.md`."
        )
        v["SPECS_CMD_NORMAL"] = "Write acceptance criteria into `docs/proposals/<name>/specs.md`."
        v["SPECS_CMD_HIGHRISK"] = "Write detailed acceptance criteria + edge cases into `docs/proposals/<name>/specs.md`."
        v["IMPLEMENT_CMD"] = "Work through `docs/proposals/<name>/tasks.md` checklist."
        v["ARCHIVE_CMD_NORMAL"] = "Move `docs/proposals/<name>/` → `docs/proposals/archive/<date>-<name>/`."
        v["ARCHIVE_CMD_HIGHRISK"] = "Move `docs/proposals/<name>/` → `docs/proposals/archive/<date>-<name>/`."

    risk_flag_table = []
    for flag, desc in [
        ("Auth", "login, logout, sessions, JWT, password, refresh token"),
        ("Authorization", "roles, permissions, tenant or company scope"),
        ("Data model", "schema, migrations, uniqueness, deletion, retention"),
        ("Audit/security", "audit logs, privacy, sensitive data, access logs"),
        ("External systems", "email, payments, cloud services, provider SDKs, queues, webhooks"),
        ("Public contracts", "API shape, response envelope, client-visible behavior"),
        ("Cross-platform", "desktop/mobile/browser split, native shell behavior, deep links"),
        ("Existing behavior", "already implemented or test-covered behavior changes"),
        ("Weak proof", "unclear or missing tests around the affected area"),
        ("Multi-domain", "more than one product domain changes at once"),
    ]:
        risk_flag_table.append(f"| {flag} | Applies when the work touches {desc} |")
    v["RISK_FLAGS_TABLE"] = "\n".join(risk_flag_table)

    v["INTAKE_OUTPUT_EXAMPLE_USER_FEATURE"] = build_intake_example(
        cfg, has_tracker, "user-feature", "normal",
        "touches API contract and existing behavior (2 flags)."
    )
    v["INTAKE_OUTPUT_EXAMPLE_INFRASTRUCTURE"] = build_intake_example(
        cfg, has_tracker, "infrastructure", "normal",
        "data model touched (1 flag), no user-visible behavior."
    )
    v["INTAKE_OUTPUT_EXAMPLE_TINY"] = build_intake_example(
        cfg, has_tracker, "docs", "tiny",
        "single-file change, 0 risk flags.", tiny=True
    )

    return v


def build_intake_example(cfg, has_tracker, change_type, lane, reason, tiny=False) -> str:
    lines = []
    if has_tracker:
        ref = cfg.get("github_repo", "owner/repo") + "#42"
        lines.append(f"Issue: {ref}")
    lines.append(f"Lane: {lane}")
    lines.append(f"Change type: {change_type}")
    lines.append(f"Reason: {reason}")
    if not tiny:
        lines.append("Proposal: <link>")
        lines.append(f"Validation: validate:quick + test:integration{' + test:e2e' if lane == 'high-risk' else ''}")
        if change_type in ("user-feature", "bug-fix"):
            lines.append("User-flow test: matching changed surface")
            lines.append(f"Review Gate: {'full review-work skill' if lane == 'high-risk' else 'single Oracle review'}")
        else:
            lines.append("User-flow test: not applicable — change type exempt")
            lines.append("Review Gate: self-verify")
    else:
        lines.append(f"Action: patch directly, run validate:quick.")
        lines.append("No proposal, no Review Gate, no user-flow test required.")
    lines.append("PR Bot Review: " + ("required (max 3 push cycles)" if not tiny else "still required if pushing to remote."))
    return "\n".join(lines)


def build_story_vars(cfg: dict, has_tracker: bool, use_openspec: bool) -> dict[str, str]:
    v: dict[str, str] = {}
    if has_tracker and cfg.get("issue_tracker") == "github":
        repo = cfg.get("github_repo", "owner/repo")
        v["ISSUE_FIELD_BLOCK"] = (
            "## GitHub Issue\n\n"
            f"`{repo}#N` — created at Feature Intake step 0. Required for every story\n"
            "(skip only for tiny-lane changes that never touch remote)."
        )
    elif has_tracker:
        tracker = cfg.get("issue_tracker", "")
        v["ISSUE_FIELD_BLOCK"] = (
            f"## {tracker.capitalize()} Issue\n\n"
            f"Issue ID — created at Feature Intake step 0. Required for every story."
        )
    else:
        v["ISSUE_FIELD_BLOCK"] = ""

    if use_openspec:
        v["SPEC_LINK_FIELD"] = "## OpenSpec Change\n\n`openspec/changes/<name>/` — leave blank for tiny lane stories."
    else:
        v["SPEC_LINK_FIELD"] = "## Proposal\n\n`docs/proposals/<name>/` — leave blank for tiny lane stories."
    return v


def github_issue_tracking_section(repo: str) -> str:
    return f"""## GitHub Issue Tracking

Every user request that triggers harness work (not a pure question) gets a
GitHub issue in `{repo}`. **Create early, update at every milestone.**

### When to create

**Create immediately after Intent Gate, BEFORE Feature Intake classification.**

The issue starts as a skeleton with the raw user request. It evolves as the
flow progresses.

**Skip issue creation for:**
- Pure conversational questions ("how does X work?")
- Read-only exploration that doesn't produce a deliverable
- Interactive setup tasks initiated by the user

When unsure: create the issue. Closing is cheap.

### Issue lifecycle

| Phase | Action | Command |
|-------|--------|---------|
| Intent | Create skeleton issue | `gh issue create --repo {repo} --title "<intent>" --body "<raw request + assumptions>"` |
| Intake | Add lane + change-type labels | `gh issue edit <N> --add-label "lane:normal,change-type:user-feature"` |
| Proposal | Comment with location | `gh issue comment <N> --body "Proposal: <location>"` |
| Deep-design | Comment with synthesis | `gh issue comment <N> --body "Deep-design: $verdict"` |
| Specs | Comment with acceptance criteria | `gh issue comment <N> --body "Acceptance: ..."` |
| Implementation | Comment per major task | `gh issue comment <N> --body "Implemented: ..."` |
| User-flow test | Comment with proof | `gh issue comment <N> --body "User-flow PASS: ..."` |
| Review Gate | Comment Review Verdict | `gh issue comment <N> --body "Review: PASS — ..."` |
| PR | Link PR to issue | `gh pr create ... --body "Closes #<N>"` |
| Archive | Close issue | auto-closed by PR merge (via `Closes #N`) |

### Labels

Apply `lane:*` + `change-type:*` (+ optional `status:*`) labels as soon as
classification completes. See `scripts/setup_labels.sh` in this skill or run:

```bash
bash ~/.config/opencode/skills/harness-init/scripts/setup_labels.sh {repo}
```
"""


def generic_tracking_section(tracker: str) -> str:
    return f"""## {tracker} Issue Tracking

Every user request gets a {tracker} issue via MCP. Create at intake, update
at every milestone (proposal, deep-design, specs, implementation, review,
PR, archive)."""


def openspec_integration_section() -> str:
    return """## OpenSpec Integration

OpenSpec is the **proposal and design layer** of this harness. Every normal or
high-risk change must have an OpenSpec change before implementation starts.

### Commands
```bash
openspec new change "<name>"            # scaffold change directory
openspec validate "<name>" --strict     # validate all artifacts
openspec archive "<name>"               # archive after merge
```"""


def deep_design_section() -> str:
    return """## Deep-Design Gap Analysis

After the proposal produces `proposal.md` and `design.md`, run **deep-design**
before locking any spec.

- Spawns Metis (scope/risk) + Oracle (architecture) in parallel
- Cross-critiques their findings
- Produces a confidence-scored synthesis: gaps, ambiguities, hidden risks

### Gate rule

```text
deep-design pass (no blocking gaps)
  → proceed to specs/ + story packet

deep-design finds gaps
  → revise proposal.md or design.md
  → re-run deep-design
  → repeat until clean pass
```

A gap is blocking if it touches: auth, data model, API contract, isolation
boundary, or multi-domain scope. Stylistic gaps are non-blocking."""


def step_0_section_github(repo: str) -> str:
    return f"""## Step 0 — Create GitHub Issue

**Before classifying anything**, create a tracking issue:

```bash
gh issue create \\
  --repo {repo} \\
  --title "<concise restatement of intent>" \\
  --body "$(cat <<'EOF'
## Intent
<verbatim user request, or paraphrased with stated assumptions>

## Lane
TBD

## Change Type
TBD

## Proposal
TBD

## Acceptance Criteria
TBD

## Progress
- [ ] Feature Intake
- [ ] Proposal + design
- [ ] Deep-design (Metis + Oracle)
- [ ] Specs + Story packet
- [ ] Implementation
- [ ] User-flow test
- [ ] Review Gate
- [ ] PR opened + bot review
- [ ] Merged + archived
EOF
)"
```

Record the returned issue number (`#N`). This is the **harness tracking ID**
for the entire flow. Update it at every milestone (see HARNESS.md
§ GitHub Issue Tracking).

**Skip issue creation only for:**
- Pure questions / explanations (no deliverable expected)
- Read-only exploration
- Live setup tasks where user is the orchestrator
- Tasks that revise the harness itself (those go via `HARNESS_BACKLOG.md`)

When unsure: **create the issue**. Closing is cheap.
"""


def step_0_section_generic(tracker: str) -> str:
    return f"""## Step 0 — Create {tracker} Issue

Use the {tracker} MCP to create a tracking issue with the raw user request as
body. Record the issue ID and update it at every harness milestone.
"""


def substitute(template: str, vars: dict[str, str]) -> str:
    def repl(m: re.Match) -> str:
        key = m.group(1)
        return vars.get(key, m.group(0))
    return re.sub(r"\$\{([A-Z_][A-Z0-9_]*)\}", repl, template)


def render_template(name: str, vars: dict[str, str]) -> str:
    src = TEMPLATES_DIR / name
    if not src.exists():
        raise FileNotFoundError(f"Template not found: {src}")
    return substitute(src.read_text(), vars)


def write_with_conflict(
    target: Path, content: str, policy: str, dry_run: bool, log: list[str]
) -> str:
    target.parent.mkdir(parents=True, exist_ok=True)
    if not target.exists():
        if dry_run:
            log.append(f"[dry-run] would create {target}")
            return "created"
        target.write_text(content)
        log.append(f"created {target}")
        return "created"
    if policy == "skip":
        log.append(f"skipped (exists) {target}")
        return "skipped"
    if policy == "backup-overwrite":
        ts = dt.datetime.now().strftime("%Y%m%d-%H%M%S")
        backup = target.with_suffix(target.suffix + f".bak.{ts}")
        if not dry_run:
            shutil.copy2(target, backup)
            target.write_text(content)
        log.append(f"{'[dry-run] would overwrite' if dry_run else 'overwrote'} {target} (backup: {backup.name})")
        return "overwritten"
    log.append(f"CONFLICT {target} — needs ask-each resolution (use --apply-individually flag)")
    return "conflict"


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--config", required=True, type=Path)
    ap.add_argument("--target", required=True, type=Path)
    ap.add_argument("--dry-run", action="store_true")
    args = ap.parse_args()

    if not args.config.exists():
        print(f"ERROR: config not found: {args.config}", file=sys.stderr)
        return 1
    if not args.target.exists():
        print(f"ERROR: target not found: {args.target}", file=sys.stderr)
        return 1

    cfg = parse_yaml(args.config)
    vars = build_vars(cfg)
    policy = cfg.get("conflict_policy", "ask-each")
    log: list[str] = []
    target = args.target.resolve()

    files = [
        ("HARNESS.md.tmpl", target / "docs" / "HARNESS.md"),
        ("FEATURE_INTAKE.md.tmpl", target / "docs" / "FEATURE_INTAKE.md"),
        ("story.md.tmpl", target / cfg.get("story_template_path", "docs/templates/story.md")),
        ("HARNESS_BACKLOG.md.tmpl", target / "docs" / "HARNESS_BACKLOG.md"),
        ("evidence_README.md.tmpl", target / cfg.get("evidence_dir", "docs/evidence") / "README.md"),
    ]

    print(f"=== Harness Init v{VERSION} ===")
    print(f"Target: {target}")
    print(f"Policy: {policy}")
    print(f"Dry-run: {args.dry_run}\n")

    for tmpl, dest in files:
        try:
            content = render_template(tmpl, vars)
        except FileNotFoundError as e:
            print(f"ERROR: {e}", file=sys.stderr)
            return 2
        write_with_conflict(dest, content, policy, args.dry_run, log)

    for line in log:
        print(line)

    print(f"\n=== Summary ===")
    print(f"Files processed: {len(files)}")
    if any("CONFLICT" in l for l in log):
        print("Conflicts detected — resolve manually or re-run with conflict_policy: backup-overwrite")
        return 1
    print("Install complete.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
