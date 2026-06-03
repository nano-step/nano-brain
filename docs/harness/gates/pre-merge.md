# Gate: pre-merge

Pre-merge gate is the final check before a PR can be merged. This is a high-stakes gate.

## Hard Rules

1. **Build passes** — `go build ./...` must succeed
2. **Short tests pass** — `go test -race -short ./...` must succeed
3. **Integration tests pass** — `go test -race -tags=integration ./...` must succeed
4. **Lint clean** — `golangci-lint run ./...` must have no issues
5. **Review verdict PASS** — Latest review doc must contain "Review Verdict: PASS" (R27)
6. **Gemini comments triaged** — Every bot comment must appear in triage table (R31)
7. **PR format correct** — PR body must have "Closes #N" (exactly one issue) (R1)
8. **PR targets master** — Base branch must be master
9. **Self-review evidence** — Self-review doc must exist for story
10. **Commit count reasonable** — Max 3 push cycles (R29)
11. **smoke:e2e evidence** — For user-feature/bug-fix, smoke output log must exist (R20)

## Step-by-Step Procedure

1. Run full validation ladder:
   ```bash
   go build ./...
   go test -race -short ./...
   go test -race -tags=integration ./...
   golangci-lint run ./...
   ```

2. Check review verdict in `docs/evidence/<slug>/review-*.md`:
   - Must contain "Review Verdict: PASS"
   - If FAIL, address all findings first

3. If PR has Gemini bot comments:
   - Open the self-review triage table
   - Every comment must have a row
   - No VALID:critical/high without "fixed in commit <sha>"

4. Verify PR body format:
   - Contains "Closes #N" or "Fixes #N"
   - Exactly one issue reference

5. Run smoke:e2e for user-feature/bug-fix:
   - `./scripts/smoke-e2e.sh`
   - Output saved to `docs/evidence/<slug>/smoke-e2e-output.log`

## Evidence Requirements

- All build/test output logs
- Review doc with PASS verdict
- Gemini triage table (if applicable)
- smoke-e2e-output.log (if applicable)

## FAIL Conditions

- Build fails → fix compilation errors
- Tests fail → fix test failures
- Lint issues → fix or justify
- Review Verdict: FAIL → address review findings
- Untriaged Gemini comments → add to triage table
- PR references multiple/no issues → fix PR body
- PR targets wrong branch → change base to master
- Missing self-review → create the document
- Too many push cycles → squash commits or escalate
- Missing smoke evidence → run smoke:e2e

Cross-reference: [docs/HARNESS_GATES.md](../../HARNESS_GATES.md#pre-merge)
