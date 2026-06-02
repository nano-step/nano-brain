# Tasks — Release-binary SHA-256 integrity verification (#320)

## Pre-implementation
- [x] GitHub issue #320 filed and labeled (`lane:normal`, `change-type:infrastructure`, `enhancement`).
- [x] Worktree created at `.opencode/worktrees/feat-320-release-sha256` on branch `feat/320-release-sha256` from `origin/master`.
- [x] Deep-design pipeline: Metis + Oracle Phase 1 + Momus sanity check = PASS.
- [x] OpenSpec proposal artifacts authored (this PR).

## Implementation

### Workflow — `.github/workflows/release.yml`
- [ ] Insert single shell step in the `release` job, AFTER `actions/download-artifact@v4` (current ~L50–53) and BEFORE `softprops/action-gh-release@v2` (current ~L55–60). Step body:
      ```yaml
      - name: Compute SHA-256 checksums
        run: cd artifacts && sha256sum nano-brain-* > SHA256SUMS
      ```
- [ ] Verify the existing `files: artifacts/*` glob in the release step picks up SHA256SUMS automatically (no glob change needed).

### postinstall — `npm/postinstall.js`
- [ ] Add `const crypto = require("crypto");` to the top-of-file requires block.
- [ ] Add pure helper `parseSHA256Line(sha256sumsContent, targetFilename)`:
  - Splits content by `\n`, regex-matches `/^([a-f0-9]{64})\s+(.+)$/` per line, returns the hex hash when the second capture group equals `targetFilename`, otherwise `null` after all lines.
- [ ] Add streaming `downloadWithHash(url, dest)`:
  - Same redirect-following pattern as existing `download()` (handle 301/302 by recursing).
  - Uses `crypto.createHash("sha256")` and tees the HTTPS response into both the hash AND `fs.createWriteStream(dest)`.
  - **Recommended pattern** (avoid `stream.pipeline` with a hash Transform — it requires the Transform to forward chunks and adds complexity for no gain):
    ```js
    const file = fs.createWriteStream(dest);
    const hash = crypto.createHash("sha256");
    res.on("data", (chunk) => hash.update(chunk));
    res.pipe(file);
    file.on("finish", () => { file.close(); resolve(hash.digest("hex")); });
    res.on("error", reject);
    file.on("error", reject);
    ```
  - Resolves to the lowercase hex digest after the write completes.
  - On any error: unlink `dest` if it exists; reject.
- [ ] Add `verifySHA256(tag, assetName, binPath, computedHex)`:
  - Build SHA256SUMS URL: `https://github.com/${REPO}/releases/download/${tag}/SHA256SUMS`.
  - Download SHA256SUMS to a temp path via the existing `download()` function.
  - Read the temp file as UTF-8; call `parseSHA256Line(content, assetName)`.
  - Delete the temp SHA256SUMS file.
  - If download itself failed (404 / network) → `console.warn("⚠ SHA256SUMS not found for tag ${tag}; skipping integrity verification.")` and return.
  - If `parseSHA256Line` returns null (malformed or missing filename) → `console.warn("⚠ Could not find ${assetName} in SHA256SUMS; skipping integrity verification.")` and return.
  - If parsed hex !== `computedHex` → `fs.unlinkSync(binPath)`; throw `new Error("SECURITY: SHA-256 mismatch for ${assetName}\n  expected: ${parsed}\n  computed: ${computedHex}\nBinary has been deleted. Build from source: CGO_ENABLED=0 go build -o nano-brain ./cmd/nano-brain")`.
  - Otherwise return (success — caller proceeds to chmod).
- [ ] Add escape-hatch check at the top of `main()` (after `const platformKey`):
      ```js
      const skipVerify = !!process.env.NANO_BRAIN_SKIP_SHA_VERIFY;
      if (skipVerify) {
        console.warn("⚠ NANO_BRAIN_SKIP_SHA_VERIFY is set; binary integrity check will be skipped.");
      }
      ```
- [ ] Refactor the two binary-download call sites:
  - **Candidate-tag loop (~L131–141)**: replace `await download(url, binPath)` with `const computedHex = await downloadWithHash(url, binPath)`. Then `if (!skipVerify) { await verifySHA256(tag, assetName, binPath, computedHex); }`. The existing chmod + success log stay; the existing try/catch keeps retry behaviour for tag-not-found cases.
  - **API fallback (~L144–153)**: same pattern as above.
- [ ] Export `parseSHA256Line` (and optionally `downloadWithHash`) so the test file can require them. Use CommonJS `module.exports = { parseSHA256Line };` at the bottom; the `main()` invocation at L161 still runs because the file is executed directly via shebang.
- [ ] Verify the file still has its `#!/usr/bin/env node` shebang at line 1.

### Tests — `npm/postinstall.test.js` (NEW file)
- [ ] Create the file using Node built-in `--test` runner. No new dependencies.
- [ ] Test cases for `parseSHA256Line`:
  - happy path: 1 line, exact filename match → returns hash
  - 4 lines (realistic 4-binary case), 3rd matches → returns 3rd hash
  - filename present but with extra whitespace → matches correctly (strip per regex)
  - filename absent → returns null
  - malformed line (no hex, wrong shape) → null
  - blank lines and `#` comments → ignored
  - trailing newline → no spurious null

### README — `README.md`
- [ ] Add new subsection "Verifying Downloads" right after the Quick Start section. ~10 lines:
  - One sentence introducing SHA256SUMS as a release asset.
  - Code block showing `curl -O .../SHA256SUMS && curl -O .../nano-brain-<platform> && sha256sum -c SHA256SUMS --ignore-missing`.
  - One sentence noting that `npm install` performs this check automatically.
  - One sentence noting the `NANO_BRAIN_SKIP_SHA_VERIFY=1` escape hatch for corporate proxies / air-gapped installs.
  - One sentence pointing to issue #320 / OpenSpec for the rationale.

## Validation ladder (per HARNESS.md, change-type: infrastructure)

- [ ] `validate:quick`: `go build ./... && go test -race -short ./...` — verifies no Go regression even though no Go was touched. Paste output to PR.
- [ ] `self-review:staged-files`: `git status` before each commit. Must contain ONLY: `.github/workflows/release.yml`, `npm/postinstall.js`, `npm/postinstall.test.js`, `README.md`, and the openspec change dir. NOT: `.opencode/`, `package-lock.json`, anything in `internal/`.
- [ ] `test:integration`: N/A — no Go change.
- [ ] `smoke:e2e`: N/A — `infrastructure` change-type is exempt per HARNESS.md change-type table.
- [ ] Run the new Node test: `node --test npm/postinstall.test.js`. All tests must pass. Paste output.

## Self-verify Review Gate (per HARNESS.md infrastructure)

- [ ] Author `docs/evidence/self-review-320-release-sha256.md` documenting:
  - Pipeline output (Metis + Oracle + Momus verdicts).
  - All 8 acceptance criteria, each with a pointer to the evidence (test name, code line, README excerpt).
  - Risk audit per `docs/FEATURE_INTAKE.md` flags.
  - Any pre-existing-master issues encountered (not introduced).
  - Final verdict.

## PR

- [ ] `git push origin feat/320-release-sha256` (verify `git branch --show-current` first).
- [ ] Open PR: `gh pr create --repo nano-step/nano-brain --base master --head feat/320-release-sha256 --title "feat(release): SHA-256 integrity verification for binaries (#320)"` with body summarising scope, ACs, evidence, deferred Phase 2 work.
- [ ] Apply labels: `lane:normal`, `change-type:infrastructure`, `enhancement`.
- [ ] Address Gemini PR bot comments per R31: triage every comment with closed-vocab verdict; fix `VALID:high`/`VALID:critical` immediately; record table in `docs/evidence/self-review-320-release-sha256.md` under `## Gemini Verification Triage`.

## Post-merge

- [ ] `bash scripts/harness-check.sh post-merge`.
- [ ] `openspec archive release-sha256-checksums --yes` to fold the ADDED requirements into `openspec/specs/release-pipeline/spec.md`.
- [ ] **Open PR for the archive commit** (do NOT push direct to master — that violates the no-direct-commits rule that bit me on #317).
- [ ] Issue #320 will auto-close via "Closes #320" in PR body.
- [ ] Clean up worktree: `git worktree remove .opencode/worktrees/feat-320-release-sha256`.
- [ ] Verify the next auto-tagged release contains a `SHA256SUMS` asset and `sha256sum -c` passes.

## Deferred follow-ups (separate issues — NOT in this PR)

- [ ] Phase 2: cosign or minisign signature over SHA256SUMS (key custody decision required).
- [ ] Phase 2: SLSA Level 3 provenance via GitHub OIDC attestations.
- [ ] Optional: backfill SHA256SUMS for past releases via one-off script + GH release-asset upload.
- [ ] Optional: add `nano-brain verify` CLI command for users to verify their already-installed binary against the live SHA256SUMS.
