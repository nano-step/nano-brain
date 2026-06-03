# Gate: in-progress

In-progress gate verifies development is on track during story implementation.

## Hard Rules

1. **On feature branch** — Must not be on master
2. **OpenSpec active** — An OpenSpec change must be in progress
3. **Tests pass** — `go build ./... && go test -race -short ./...` must pass
4. **Self-review done** — For TRACE_SPEC Tier 2 changes, self-review doc must exist with required sections

## Step-by-Step Procedure

1. Verify on feature branch: `git branch --show-current`
2. Verify OpenSpec change active: `openspec list` shows your change
3. Run validation ladder: `go build ./... && go test -race -short ./...`
4. If Tier 2 change:
   - Create `docs/evidence/<slug>/self-review-<slug>.md`
   - Include: Risk Assessment, Summary of Changes, Behavior Validation, Evidence Pointers

## Evidence Requirements

- Feature branch name
- OpenSpec change name
- Passing build/test output
- Self-review doc (Tier 2 only)

## FAIL Conditions

- On master → create feature branch first
- No OpenSpec change → run `/opsx-propose` to create one
- Build/tests fail → fix the issues
- Missing self-review (Tier 2) → create the self-review document

Cross-reference: [docs/HARNESS_GATES.md](../../HARNESS_GATES.md#in-progress)
