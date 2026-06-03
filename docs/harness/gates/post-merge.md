# Gate: post-merge

Post-merge gate verifies that merge to master completed cleanly and artifacts are archived.

## Hard Rules

1. **PR merged** — The PR must be in merged state
2. **Issue closed** — Associated GitHub issue must be closed
3. **OpenSpec archived** — The change must be archived with evidence (R28)

## Step-by-Step Procedure

1. Verify PR merged:
   - `gh pr view <number> --json state` should show "MERGED"

2. Verify issue closed:
   - `gh issue view <number> --json state` should show "CLOSED"

3. Archive OpenSpec change:
   ```bash
   openspec archive "<change-name>"
   ```
   - This moves the change to `openspec/archive/`
   - Review evidence must be present before archiving

## Evidence Requirements

- PR merged state confirmation
- Issue closed state confirmation
- OpenSpec archive command output

## FAIL Conditions

- PR not merged → complete the merge first
- Issue still open → close with `gh issue close <number>`
- OpenSpec not archived → run `openspec archive`
- No review evidence in archive → ensure review doc exists before archiving

Cross-reference: [docs/HARNESS_GATES.md](../../HARNESS_GATES.md#post-merge)
