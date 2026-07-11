# Self-review — #592 (npm postinstall ENOENT crash)

Branch: `fix/npm-postinstall-unlink-crash`. Change-type: bug-fix. Lane: tiny.

## Summary of change
- `npm/postinstall.js`: added `safeUnlink(p)` (`if (fs.existsSync(p)) fs.unlinkSync(p)`, error-swallowing) and routed every `fs.unlinkSync` in `download()` and `downloadWithHash()` through it — the 302-redirect branch, the non-200 branch, and the socket `.on("error")` handler. Exported `safeUnlink`.
- `npm/postinstall.test.js`: 2 regression tests.
- `README.md`: npm downloads/week badge.

## Response shape
N/A — no API response struct changed. REST/MCP untouched.

## Staged files
Only `npm/postinstall.js`, `npm/postinstall.test.js`, `README.md`, and `docs/evidence/*` for this story. No `.opencode/`, no `package-lock.json`, no Go files. `package.json` NOT changed (stays `0.0.0-dev`).

## Verification
- `node --test npm/postinstall.test.js`: 20/20 pass (incl. 2 new safeUnlink tests).
- `CGO_ENABLED=0 go build ./...`: OK (JS/MD change does not affect Go).
- smoke:e2e (`docs/evidence/smoke-e2e-npm-postinstall.md`): old code crashes ENOENT at postinstall.js:53; fixed code prints "installed successfully" with no crash over the real 302→200 release download.

## Notes
- Root network is fine (45MB asset downloads); the bug was purely the double-unlink crash masking the flow. The fix makes the flow robust; where the network genuinely fails, it now falls back gracefully (verifySHA256 warns+skips; binary loop reports "Failed to download... build from source") instead of a confusing ENOENT stack.
- Scope kept minimal: did not refactor retry/proxy behavior (not the reported defect).
