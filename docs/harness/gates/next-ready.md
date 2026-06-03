# Gate: next-ready

Next-ready gate verifies you're ready to start the next feature after completing the previous one.

## Hard Rules

1. **On master** — Must be on master branch (not a stale feature branch)
2. **master up-to-date** — Local master must have the merged changes
3. **No open PRs** — All feature PRs should be merged or closed
4. **OpenSpec clean** — No active changes, all completed work archived

## Step-by-Step Procedure

1. Switch to master:
   ```bash
   git checkout master
   git pull origin master
   ```

2. Verify no open PRs:
   ```bash
   gh pr list --state open
   ```
   Should return empty.

3. Verify OpenSpec clean:
   ```bash
   openspec list
   ```
   Should show no active changes.

4. Clean up stale branches (optional):
   ```bash
   git branch -d <merged-branch>
   ```

## Evidence Requirements

- On master branch
- `git log -1` shows the merge commit
- Empty `gh pr list --state open`
- Clean `openspec list`

## FAIL Conditions

- Not on master → `git checkout master`
- master behind origin → `git pull origin master`
- Open PRs exist → merge or close them
- Active OpenSpec changes → archive completed changes

Cross-reference: [docs/HARNESS_GATES.md](../../HARNESS_GATES.md#next-ready)
