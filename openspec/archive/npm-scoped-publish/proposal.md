# npm Scoped Package + CI Auto-Publish

## Problem

1. `nano-brain` npm package is unscoped — not associated with `@nano-step` org
2. npm publish is manual — friction on every release
3. Stub workflow files (`publish-beta.yml`, `publish-stable.yml`) serve no purpose

## Solution

### Dual Publish
- Primary: `@nano-step/nano-brain` (scoped, public)
- Alias: `nano-brain` (unscoped, backward compat)
- Both published simultaneously on tag push

### CI Automation
Extend `release.yml` with npm publish job:
- After binaries are built + GitHub Release created
- Determine npm tag from git tag: `v*-beta*` → `--tag beta`, else → `--tag latest`
- Publish `@nano-step/nano-brain` first, then copy `package.json` name → `nano-brain` and publish again
- Uses `NPM_TOKEN` GitHub secret

### Cleanup
- Remove `publish-beta.yml` and `publish-stable.yml` stubs
- Deprecate old `nano-brain` v1 versions with redirect message

## Scope
- `package.json`, `release.yml`, `publish-beta.yml`, `publish-stable.yml`
- `npm/postinstall.js` and `npm/run.js` (if name referenced)
- `README.md` (npx command references)

## References
- Issue: #139
