# Design — Release-binary SHA-256 integrity verification

## Context

Current release pipeline (`.github/workflows/release.yml`):

1. **Per-platform build jobs** (matrix: ubuntu-latest + macos-latest × amd64 + arm64) cross-compile binaries via `CGO_ENABLED=0 go build` and upload each as a workflow artifact.
2. **Release job** (ubuntu-latest, `needs: build`) downloads all 4 artifacts into `artifacts/` and publishes a GitHub Release via `softprops/action-gh-release@v2` with `files: artifacts/*`.
3. **npm-publish job** (ubuntu-latest, `needs: release`) publishes both `@nano-step/nano-brain` and the unscoped alias `nano-brain` to npm.

Current install pipeline (`npm/postinstall.js`):

1. Reads `VERSION` from `package.json`, computes `<platform>-<arch>` key.
2. Builds candidate tags from `VERSION` (handles npm leading-zero normalization quirks — lines 58–77).
3. For each candidate tag: `https.get` → `res.pipe(file)` → `chmod 0755`. On failure, tries next candidate.
4. If all candidates fail, falls back to GitHub API to find the correct tag (lines 95–108).

There is no integrity verification anywhere.

## Architecture

```
release.yml release job (existing):
  download-artifact → softprops/action-gh-release
                          ↓
release.yml release job (NEW):
  download-artifact → [NEW: cd artifacts && sha256sum nano-brain-* > SHA256SUMS]
                    → softprops/action-gh-release   (files: artifacts/*  ← picks up SHA256SUMS automatically)

postinstall.js (existing):
  candidate-tag loop OR API fallback:
    download(binary) → chmod 0755

postinstall.js (NEW):
  Top-of-main escape hatch:
    if (process.env.NANO_BRAIN_SKIP_SHA_VERIFY)  → warn + use legacy download() path

  candidate-tag loop OR API fallback:
    downloadWithHash(binary) → returns computedHex
                              ↓
                     verifySHA256(tag, assetName, computedHex)
                              ↓
                     fetch SHA256SUMS from same tag
                          ├── 404 / network error    → WARN + return (skip verify)
                          ├── malformed              → WARN + return (skip verify)
                          ├── filename not listed    → WARN + return (skip verify)
                          ├── hash matches           → return (success)
                          └── hash mismatch          → unlink binary + throw (caller exits 1)
                              ↓
                     chmod 0755
```

### Why the design works

1. **No new dependencies**: `sha256sum` is in GNU coreutils on `ubuntu-latest`. `crypto` is a Node built-in.
2. **Atomic publish**: `softprops/action-gh-release@v2` uploads all `files: artifacts/*` in one API call → no race window between SHA256SUMS and binaries.
3. **Stream-tee** in `downloadWithHash`: pipes the HTTPS response through `crypto.createHash('sha256')` AND `fs.createWriteStream` in parallel, so the hash is computed during download — no double-read of the binary from disk.
4. **Backward compatible**: SHA256SUMS 404 → soft fail. Old releases (tagged before this change) install without verification, just with a WARN line.
5. **Hard fail only on actual mismatch**: differentiates "verification couldn't run" (soft) from "verification proved tampering" (hard).

## File-Level Touch Points (MVA)

| File | Lines added | Change summary |
|---|---|---|
| `.github/workflows/release.yml` | +3 | Insert single shell step `cd artifacts && sha256sum nano-brain-* > SHA256SUMS` between `download-artifact` and `softprops/action-gh-release` |
| `npm/postinstall.js` | +~70 | Add `crypto` require; add `parseSHA256Line()` pure helper; add `downloadWithHash()` streaming function; add `verifySHA256()` orchestrator; add escape-hatch env check; update both call sites (candidate-tag loop ~L134 and API fallback ~L147) |
| `npm/postinstall.test.js` | +~60 (new file) | Pure-function tests via Node built-in `--test` runner for `parseSHA256Line` |
| `README.md` | +~10 | New "Verifying Downloads" subsection after Quick Start |

Total: ~145 LOC across 4 files, no Go code changes, no schema/migration changes.

## Decision Log

| Decision | Alternatives considered | Chosen approach | Rationale |
|---|---|---|---|
| File format | GNU coreutils `<hex>  <file>` vs BSD `SHA256 (file) = <hex>` vs JSON | **GNU coreutils** | Universal Linux standard, `sha256sum -c` works out of the box, two-space separator is unambiguous |
| File name | `SHA256SUMS` (no ext) vs `SHA256SUMS.txt` vs per-binary `.sha256` sidecars | **`SHA256SUMS`** | De facto standard (Linux kernel, Hashicorp, Ubuntu, Debian) |
| Compute tool | `sha256sum` (coreutils) vs `shasum -a 256` (BSD/macOS) vs Node script | **`sha256sum`** | Release job runs on `ubuntu-latest`; no cross-platform concern; shell is more transparent than Node script for auditing |
| Where to compute | Per-platform build job vs release job | **Release job** | All 4 artifacts already in `artifacts/`; single command produces single file; no merge step needed |
| Hash computation in postinstall | Read file from disk after download vs stream-tee during download | **Stream-tee** | Idiomatic Node; no double-read of ~15MB binary; computed digest available as soon as download completes |
| Existing `download()` function | Modify to optionally hash vs add sibling `downloadWithHash()` | **Sibling function** | `download()` still needed for SHA256SUMS itself (small text file, no hash); cleaner separation than overloading download() with optional behavior |
| Failure on hash mismatch | WARN + continue vs hard fail | **Hard fail (exit 1)** | Mismatch = security event; silent install of corrupted/tampered binary defeats the purpose. **Note**: this is the FIRST `process.exit(1)` path in `postinstall.js` — the current file uses `process.exit(0)` for all failures (L156–158, best-effort install). The new mismatch path is intentionally fail-closed and breaks `npm install` if triggered. All other new failure modes (404, malformed, env-var skip) preserve the existing `process.exit(0)` best-effort semantics. |
| Failure on SHA256SUMS 404 | Hard fail vs soft fail with WARN | **Soft fail with WARN** | Old releases legitimately don't have SHA256SUMS; hard-fail would break `npm install <old-version>` |
| Failure on malformed SHA256SUMS | Hard fail vs soft fail | **Soft fail with WARN** | Can't distinguish malformed-by-tampering from malformed-by-bug; conservative path is don't-block-install. The hash-mismatch path remains the actual security gate. |
| Escape hatch env var | None vs `NANO_BRAIN_SKIP_SHA_VERIFY=1` | **YES — env var with WARN** | Corporate proxies / air-gapped environments / debugging need a bypass. Industry standard (`npm config set strict-ssl false`, `pip --no-verify`, etc.) |
| Self-verify smoke step in release.yml | YES vs NO | **NO** | `sha256sum` is deterministic — re-running it produces the same bytes; chicken-and-egg with the GH Release publish step; consumer-side verification is where it matters |
| Test approach | Mock HTTPS + run full `verifySHA256` | Pure-function tests on `parseSHA256Line` only | **Pure-function tests** | Higher signal-to-noise than mocked HTTPS; `downloadWithHash` is largely glue around well-tested Node stdlib (`crypto.createHash`, `stream.pipeline`); the real verification is the consumer install path which runs every `npm install` of every release |
| README placement | New "Security" section vs subsection under "Quick Start" | **Subsection under "Quick Start"** | Verification is part of the install flow, not a standalone topic; "Security" section can come in Phase 2 with cosign/SLSA |

## Risks & Mitigations

| Risk | Severity | Mitigation |
|---|---|---|
| `sha256sum` output includes path prefix → postinstall parser misses the filename | MEDIUM | Use `cd artifacts && sha256sum nano-brain-*` (not `sha256sum artifacts/*`) so filenames are bare; test with parseSHA256Line happy case |
| Old releases without SHA256SUMS get installed by new postinstall.js → install breaks | HIGH if not handled | Soft fail with WARN on SHA256SUMS 404; AC #4 explicitly covers this |
| User in corporate proxy / air-gap environment gets blocked | MEDIUM | `NANO_BRAIN_SKIP_SHA_VERIFY=1` escape hatch (AC #6) |
| Stream-tee bug causes hash to be wrong but binary to be intact → false positive mismatch | LOW | Pure-function tests on `parseSHA256Line`; `crypto.createHash` + `stream.pipeline` are battle-tested Node stdlib |
| Old releases re-tagged (force-push) → new SHA256SUMS doesn't match cached `npm install` | LOW | npm caches the postinstall script per package version, not per binary download; force-push is rare; if it happens, `npm uninstall && npm install` clears the cache |
| Race during release: SHA256SUMS upload partial-succeeds, binary upload succeeds, user downloads → mismatch | LOW | `softprops/action-gh-release@v2` is atomic (single API call); GH publishes only on success |

## Test Plan

### Unit tests (`npm/postinstall.test.js`, Node built-in `--test` runner)

```js
const test = require('node:test');
const assert = require('node:assert');
const { parseSHA256Line } = require('./postinstall');  // requires export

test('parseSHA256Line: happy path returns hash for matching filename', () => { ... });
test('parseSHA256Line: returns null when filename not present', () => { ... });
test('parseSHA256Line: returns null for malformed input', () => { ... });
test('parseSHA256Line: handles trailing newline and multiple entries', () => { ... });
test('parseSHA256Line: ignores blank lines and lines with wrong shape', () => { ... });
```

Run: `node --test npm/postinstall.test.js`

### Manual verification of release.yml change

- After merge, the next auto-tag run produces a `v2026.6.2.N` release.
- Inspect the release page: SHA256SUMS asset must be present alongside the 4 binaries.
- Download SHA256SUMS + one binary, run `sha256sum -c SHA256SUMS --ignore-missing` → expect `nano-brain-linux-amd64: OK`.

### Manual verification of postinstall.js change

- After merge + release, run `npm install @nano-step/nano-brain@<new-version>` in a clean dir.
- Verify postinstall logs include `nano-brain v<version> installed successfully from <tag>` (and no WARN).
- To test backward compat: `npm install @nano-step/nano-brain@<old-version-before-this-PR>` → expect WARN about missing SHA256SUMS + successful install.
- To test escape hatch: `NANO_BRAIN_SKIP_SHA_VERIFY=1 npm install @nano-step/nano-brain` → expect WARN about skip + successful install.

### Validation ladder (per harness, infrastructure change-type)

| Layer | Command | Required? |
|---|---|---|
| `validate:quick` | `go build ./... && go test -race -short ./...` | ✅ (no Go change, but verifies nothing else broke) |
| `self-review:staged-files` | `git status` — only `release.yml`, `postinstall.js`, `postinstall.test.js`, `README.md`, openspec change dir | ✅ |
| `test:integration` | N/A — no Go change | ❌ |
| `smoke:e2e` | N/A — `infrastructure` change-type exempt per HARNESS.md table | ❌ |

### Review gate

`infrastructure` change-type → `⚠️ self-verify` per HARNESS.md. PR bot review (Gemini) still required per harness flow.
