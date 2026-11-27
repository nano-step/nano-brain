# Codex Review — PR #749 / Issue #745

Reviewer: OpenAI Codex CLI (`codex review --base origin/master`), non-interactive read-only review pass.
Runtime evidence: codex-cli 0.172.0; session `01a21df7-bee1-7d93-ba7a-bf81df1b9c25`; raw session output captured locally at `/tmp/codex-review-745.raw` during the run.
Review Verdict: FAIL before fix; PASS expected after the valid P2 finding is fixed and re-reviewed.

## Findings and triage

| Finding | Severity | Disposition | Evidence |
|---|---:|---|---|
| Default CCS root created after daemon startup is omitted from startup patterns, so dynamic rediscovery cannot see it until restart | P2 | VALID — fixed in this branch | `cmd/nano-brain/detect.go` skipped a non-existent default root; runtime startup patterns are fixed in `cmd/nano-brain/claudecode_init.go` |

## Why the finding is valid

Startup auto-detection previously copied only evidence-based existing roots into the effective harvester config. The dynamic harvester correctly re-expands configured glob patterns every cycle, but it cannot expand a default `~/.ccs` pattern that was never captured. That made first-install-after-startup require a daemon restart even though the feature's goal is dynamic discovery.

## Fix summary

Runtime auto-detection now keeps evidence-based detection order and appends stable standard future install patterns (platform Claude project roots plus default `~/.ccs/instances/*/projects`) after deduplication. Init-time detection remains evidence-based, so machines with no Claude/CCS storage still render Claude harvesting as disabled rather than falsely treating absent storage as detectable.

Regression coverage:

- `TestApplyClaudeCodeStorageAutoDetection_GatesAndPopulates` asserts the future default CCS glob is retained.
- `TestBuildRenderedConfig_NoClaudeStorageLeavesHarvestDisabled` continues to prove that absent storage does not enable Claude harvesting during init.
- Existing dynamic first-session integration coverage proves that a configured glob with no initial matches is re-expanded on later cycles.
