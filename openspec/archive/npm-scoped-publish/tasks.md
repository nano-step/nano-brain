# Tasks: npm Scoped Package + CI Auto-Publish

## Implementation

- [ ] Update `package.json`: name → `@nano-step/nano-brain`, add `publishConfig.access: public`, version → `0.0.0-dev`
- [ ] Update `release.yml`: add `npm-publish` job with dual publish + version-from-tag
- [ ] Delete `publish-beta.yml` and `publish-stable.yml` stubs
- [ ] Update `README.md`: add `@nano-step/nano-brain` references alongside `nano-brain`

## Verification

- [ ] Build passes
- [ ] Tag v2.0.0-beta.6, verify release workflow publishes both packages
- [ ] `npx @nano-step/nano-brain@beta` works
- [ ] `npx nano-brain@beta` still works
