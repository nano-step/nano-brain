# Gate: pre-work

Pre-work gate verifies you're ready to start a new feature.

## Hard Rules

1. **Previous PR merged** — No open feature PRs should exist
2. **OpenSpec clean** — No active OpenSpec changes (archive completed ones first)
3. **Issue exists** — GitHub issue must exist for the feature (R89)
4. **master up-to-date** — Local master must be in sync with origin
5. **Tests pass** — `go build ./... && go test -race -short ./...` must pass
6. **Feature branch** — Must be on a feature branch, not master

## Step-by-Step Procedure

1. Run `gh pr list --state open` — should return empty
2. Run `openspec list` — should show no active changes
3. Check `gh issue view <number>` exists for your feature
4. Run `git fetch && git log origin/master..master` — should be empty
5. Run `go build ./... && go test -race -short ./...` — should pass
6. Create feature branch: `git checkout -b feat/NNN-short-name`

## Evidence Requirements

- GitHub issue URL
- Clean `openspec list` output
- Passing build/test output

## FAIL Conditions

- Open PRs exist → merge or close them first
- Active OpenSpec changes → archive with `openspec archive`
- No issue → create one via `gh issue create`
- Unpushed commits on master → push or reset
- Build/tests fail → fix before starting new work

Cross-reference: [docs/HARNESS_GATES.md](../../HARNESS_GATES.md#pre-work)
