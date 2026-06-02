# PRE-WORK gate — Issue #331

**Date**: 2026-06-02
**Issue**: #331 (skills/nano-brain/SKILL.md v3.1.0 has 3 doc drifts)
**Lane**: tiny | **Change-type**: docs
**Branch**: `docs/331-skill-doc-drifts`
**Worktree**: `.opencode/worktrees/feat-331-skill-doc-drifts/`

## Gate Run Output (from master before worktree creation)

```
─ PRE-WORK checks
[FAIL] 1.1 Open PRs still pending (1)
[PASS] 1.2 No active OpenSpec changes
[PASS] 1.3 Issue #331 exists (state: OPEN)
[PASS] 1.4 master is up-to-date
[PASS] 1.5 Validation ladder passes
[SKIP] 1.6 On master or branch unknown (check after creating feature branch)
Summary: 4 PASS, 1 FAIL, 1 SKIP (total: 6)
```

## [HARNESS-OVERRIDE] Gate 1.1

**Reason**: Open PR #321 (`feat/320-release-sha256` by kokorolx, created 2026-06-02T06:43:58Z) is an unrelated feature from a prior session targeting issue #320 (release SHA-256 verification). It is not blocked, not abandoned, and not related to #331 (docs-only skill fix).

**Risk assessment**:
- #331 touches `skills/nano-brain/SKILL.md` only (1 file, 3 line edits)
- PR #321 touches `.github/workflows/`, `cmd/nano-brain/`, release scripts
- ZERO file overlap → ZERO merge conflict risk
- Both can ship independently

**Decision**: Proceed with #331. PR #321 left in place for its owner to resolve.

## Lane justification (tiny)

- Files changed: 1 (`skills/nano-brain/SKILL.md`)
- Lines changed: ~10 (3 doc edits + version bump in YAML frontmatter)
- Risk flags: 0
- Change type: docs (no production code touched)
- Skip:
  - smoke:e2e (not required for docs lane)
  - OpenSpec proposal (docs-only edits)
  - Integration tests (no code change)
- Required:
  - validate:quick (already PASS on master)
  - self-review:staged-files
