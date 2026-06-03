# Gate: post-merge-npm-release

Async gate that waits for the GitHub Actions release pipeline to complete and npm packages to publish.

## How This Gate Works

This is an **async gate** — the harness spawns a background watcher subagent that polls until the release completes or times out (max 30 minutes). The main session stays idle while the watcher polls.

Expected timeline:
- **Typical**: 3-5 minutes (auto-tag → release.yml → npm publish)
- **Slow**: 10-15 minutes (runner queue)
- **Timeout**: 30 minutes (something is wrong)

## What Gets Published

After a merge to master:
1. `auto-tag.yml` computes next tag (`v{YYYY}.{M}.{D}.{N}`) and pushes it
2. The tag push triggers `release.yml`
3. `release.yml` builds 4 platform binaries and creates a GitHub Release
4. `release.yml` runs `npm publish` for both `@nano-step/nano-brain` and `nano-brain` (unscoped alias)

## Verification Procedure

```bash
# 1. Check latest GitHub release tag
LATEST_TAG=$(gh release list --limit 1 --json tagName -q '.[0].tagName')
echo "Latest release: $LATEST_TAG"

# 2. Check npm published version
NPM_VERSION=$(npm view @nano-step/nano-brain version 2>/dev/null || echo "error")
echo "npm version: $NPM_VERSION"

# 3. Check if versions match
if [[ "v${NPM_VERSION}" == "$LATEST_TAG" ]]; then
  echo "✅ PASS: npm and GitHub release are in sync"
else
  echo "❌ FAIL or WAITING"
fi

# 4. Check if release workflow is still running
gh run list --workflow=release.yml --limit 5 --json status,conclusion,headBranch
```

## PASS Conditions

- Latest GitHub tag matches `npm view @nano-step/nano-brain version` (prefixed with `v`)
- Both `@nano-step/nano-brain` and `nano-brain` aliases are up to date

## WAITING Conditions

- `gh run list --workflow=release.yml` shows status=`in_progress` or status=`queued`
- Return WAITING with `wait_seconds: 60`

## FAIL Conditions

- Release workflow shows `conclusion=failure` → inspect with `gh run view <id> --log-failed`
- npm version doesn't match after workflow completed → check npm publish step in workflow logs
- Tag exists but release not created → rerun the release workflow manually

## Investigating Failures

```bash
# Find the failing run
FAILED_RUN=$(gh run list --workflow=release.yml --limit 5 --json databaseId,conclusion -q '.[] | select(.conclusion=="failure") | .databaseId' | head -1)

# View failed steps
gh run view $FAILED_RUN --log-failed
```

Cross-reference: [docs/HARNESS_GATES.md](../../HARNESS_GATES.md#post-merge)
