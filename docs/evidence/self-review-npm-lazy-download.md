# Self-review — #594 (lazy-download binary in run.js)

Branch: `fix/npm-lazy-download-run`. Change-type: bug-fix. Lane: tiny.

## Summary
- `npm/postinstall.js`: extracted the download flow into an exported `async ensureBinary()` (and `binaryPath()`); it THROWS on failure instead of calling process.exit, so callers choose exit behavior. `main()` now wraps it (SECURITY → exit 1; other failure → warn + exit 0 so `npm install` never fails). Progress logs moved to stderr. Switched the version probe from a shell-string child-process call to an argv-array one (no shell).
- `npm/run.js`: when the binary is missing (postinstall failed, or the package manager discarded the postinstall-created file — npm 11 / node 26), call `ensureBinary()` at run time (writes persist post-install), then run it. `NANO_BRAIN_BIN` override and existing-binary fast paths unchanged.
- `npm/postinstall.test.js`: test that `ensureBinary()` returns an already-present valid binary without downloading.

## Response shape
N/A — no API/response struct changed.

## Staged files
`npm/postinstall.js`, `npm/run.js`, `npm/postinstall.test.js`, `docs/evidence/*`. No Go, no `.opencode/`, no `package.json` change (stays 0.0.0-dev).

## Verification
- `node -c` both files: clean. `node --test npm/postinstall.test.js`: 23/23 pass.
- `CGO_ENABLED=0 go build ./...`: unaffected (JS-only).
- smoke:e2e (`docs/evidence/smoke-e2e-npm-lazy-download.md`): packed@2026.7.1103 → installed (binary NOT persisted) → `run.js version` self-heals: downloads at run time, prints `nano-brain v2026.7.1103` on stdout, progress on stderr.

## Notes
- Builds on #592/#593 (crash + robustness fixes already on master).
- `ensureBinary` reuses the exact download/verify/retry/API-fallback logic; only its failure signalling changed (throw vs process-exit). SECURITY hard-fail preserved and propagated.
- Full network mock of the run-time download path deemed disproportionate (same rationale accepted in #592); the e2e exercises the real path against a live release.
