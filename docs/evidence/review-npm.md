Reviewer: gsd-code-reviewer (Sonnet)
Review Verdict: PASS

Scope: re-review of `npm/postinstall.js`, `npm/postinstall.test.js`, `README.md` (uncommitted working-tree diff on `fix/npm-postinstall-unlink-crash`). Prior verdict was FAIL (1 BLOCKER, 3 WARNINGs); all four have been addressed. Verified `node -c postinstall.js` clean and `node --test postinstall.test.js` → 22/22 pass.

## Findings — all resolved

- **BLOCKER (resolved)** — `npm/postinstall.js:60-68` (`download()`): `res.pipe(file)` now has a paired `res.on("error", (err) => { file.close(); safeUnlink(dest); reject(err); })`, mirroring `downloadWithHash()` (lines 95-103). This closes the mid-stream socket-reset crash path — an unhandled `'error'` on the piped response no longer escapes as an uncaught exception; it rejects the promise so `main()`'s per-tag retry loop continues. Confirmed by grep that both download functions now have identical pipe+error handling, and no `unlinkSync` remains outside `safeUnlink`.

- **WARNING (resolved)** — `npm/postinstall.js:143-147` (`verifySHA256`, SHA-mismatch branch): now `safeUnlink(binPath)` before throwing, with a comment explaining why. An unrelated fs error (e.g. `EACCES`) can no longer turn a SHA mismatch into a non-`"SECURITY:"` rejection that `main()` would treat as retryable — the hard-fail fail-safe is preserved.

- **WARNING (resolved)** — `npm/postinstall.js:130-132` (`verifySHA256` `finally`): replaced the inline `existsSync`/`try-catch unlinkSync` reimplementation with `safeUnlink(sumsPath)`. No more duplication; single cleanup helper used consistently across the file.

- **WARNING (resolved)** — `npm/postinstall.test.js`: `download` is now exported, and two new tests assert that both `download()` and `downloadWithHash()` **reject** (not throw, not crash) on a connection error (`https://127.0.0.1:1` → ECONNREFUSED) and leave no lingering `dest` file. This directly locks in the "error rejects the promise, cleanup runs, retry loop can proceed" contract that the helper-only tests previously missed. Suite is 22/22.

## Residual (non-blocking, accept)

- The two new download tests exercise the **request-level** `.on("error")` path (connection refused fires on the request object). The specific BLOCKER path — an `'error'` on `res` **mid-stream, after a 200** — now has a handler but is not covered by a dedicated test, because reproducing it requires stubbing `https.get`/injecting a fake response stream. I agree with the coordinator that a full https-mock for that one branch is disproportionate here: the handler is a verbatim mirror of the already-shipped `downloadWithHash` handler, the code is exercised structurally (grep confirms symmetry), and the request-error tests cover the same reject-and-cleanup contract on the adjacent path. If this file grows more download-path complexity later, a small `https.get` stub test for the mid-stream case would be the right follow-up — not a merge blocker now.
- Minor: on ECONNREFUSED the write stream's file may be created and then removed by `safeUnlink` (best-effort). Tests confirm no file lingers; this is inherent to `createWriteStream` opening before the request resolves and is acceptable.

## Unchanged / re-confirmed clean

- README npm-downloads badge (`img.shields.io/npm/dw/@nano-step/nano-brain`) — verified live HTTP 200 in the prior pass, valid endpoint, well-formed markdown, consistent with sibling badges. Untouched.
- `main()` retry loop + API fallback + `verifySHA256`'s `"SECURITY:"` → `process.exit(1)` short-circuit — intent intact; the mismatch branch now reaches the throw reliably regardless of unlink outcome.

## Verdict rationale

The reachable uncaught-exception crash path that drove the FAIL is closed, the two secondary unlink-guarding inconsistencies are fixed with the shared helper, and the regression is now under test at the reject/cleanup contract level. No blocking issue remains. PASS.
