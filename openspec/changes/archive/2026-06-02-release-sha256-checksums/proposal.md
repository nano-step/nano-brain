# Release-binary SHA-256 integrity verification

## Issue
[#320 — Release-binary integrity verification (SHA-256 checksums + npm postinstall verify)](https://github.com/nano-step/nano-brain/issues/320)

## Lane
normal — infrastructure change. Touches release workflow + npm postinstall + README. No hard gates (no auth, data-model, search-quality, embedding-provider, public-api-contract, audit, authorization, external-provider). The `audit-security` flag is additive (this change *adds* a security verification path; it does not modify existing security surfaces).

## Why

The release pipeline today produces 4 cross-platform Go binaries and uploads them to GitHub Releases. The npm wrapper (`npm/postinstall.js`) downloads one of those binaries over HTTPS and `chmod 0755`s it. **There is no integrity verification anywhere in this chain.**

Concrete gaps in `master`:

1. **`.github/workflows/release.yml`** uploads binaries but does NOT compute or publish checksums.
2. **`npm/postinstall.js`** pipes the HTTPS response straight to disk with no hash verification, no Content-Length sanity check, and no signature check.
3. Users have no published reference (e.g. `SHA256SUMS` file) to compare against if they want to verify a downloaded binary independently.

### Threat model

Phase 1 (this change) mitigates:
- Download corruption (bit-flip, truncation, partial write)
- CDN cache serving wrong binary
- Build-pipeline mismatch (wrong-platform binary slipped through)

Phase 1 does NOT mitigate (deferred to Phase 2, separate issue):
- Compromised GitHub credentials / repo takeover (attacker swaps both binary AND SHA256SUMS in lockstep)
- Compromised CI runner producing a malicious binary with a valid hash
- GitHub infrastructure compromise

SHA-256 alone provides **integrity** (the binary you got matches what we published) but not **authenticity** (the binary we published was ours). Authenticity is Phase 2 work (cosign/minisign + SLSA provenance).

## Desired Outcome

A user can verify their downloaded `nano-brain` binary against a published `SHA256SUMS` file using standard `sha256sum -c` tooling, AND the npm wrapper performs this verification automatically during `npm install`.

## Constraints

- Backward compatible: old releases without `SHA256SUMS` continue to install (soft fail with WARN, do not break `npm install` for users pinning old versions).
- No new external dependencies in `npm/postinstall.js` (use Node built-in `crypto`).
- No new workflow steps requiring third-party actions (use shell `sha256sum` from GNU coreutils, preinstalled on `ubuntu-latest`).
- Must NOT modify the existing version-normalization logic in `postinstall.js` (`candidateTagsForVersion`, `normalizeVersion`, `resolveTagFromAPI` — lines 58–108).
- Must NOT break the existing single-PR, auto-tag → release → npm-publish pipeline. The change is additive in `release.yml` (one new step between existing steps).

## Out of Scope (deferred to follow-up issues)

- **Cryptographic signatures** (cosign, minisign, GPG) — Phase 2. Requires key custody decision.
- **SLSA Level 3 provenance** via GitHub OIDC attestations — Phase 2.
- **Backfilling SHA256SUMS for past releases** — one-off task, separate issue if requested.
- **Content-Length sanity check** — orthogonal to SHA integrity; covered transitively by hash check.
- **`nano-brain verify` CLI command** — feature, not infra.
- **Self-verification step in `release.yml`** that downloads its own SHA256SUMS + binary and re-verifies before publishing npm — `sha256sum` is deterministic; not worth the CI time + chicken-and-egg with the GH Release publish step.
- **Per-binary `.sha256` sidecar files** — single `SHA256SUMS` file is the universal convention (Linux kernel, Hashicorp, Ubuntu, Debian).

## Acceptance Criteria

1. **SHA256SUMS published**: every GitHub Release tagged after this change merges has a `SHA256SUMS` asset listing the SHA-256 of all 4 binaries in GNU coreutils format (`<64-hex-chars>  <filename>\n`).
2. **Manual verification works**: a user can `curl -O https://.../SHA256SUMS && curl -O https://.../nano-brain-linux-amd64 && sha256sum -c SHA256SUMS --ignore-missing` and see `nano-brain-linux-amd64: OK`.
3. **Automatic verification on install**: `npm install @nano-step/nano-brain` (or unscoped `nano-brain`) verifies the downloaded binary against the published hash before `chmod 0755`. On mismatch, postinstall exits with code 1 and prints a SECURITY error showing both expected and computed hashes.
4. **Backward compat — old release without SHA256SUMS**: installing a version tagged before this change merges does NOT fail; postinstall prints a WARN line and proceeds to chmod the binary without verification.
5. **Backward compat — SHA256SUMS network failure**: if SHA256SUMS download itself fails (5xx, network drop), postinstall prints a WARN and skips verification rather than blocking the install. (Hash MISMATCH ≠ hash UNAVAILABLE.)
6. **Escape hatch**: `NANO_BRAIN_SKIP_SHA_VERIFY=1` env var causes postinstall to skip verification entirely, with a one-line WARN. The binary still downloads and runs.
7. **README updated**: a new "Verifying Downloads" subsection appears in `README.md` after Quick Start, documenting the `sha256sum -c SHA256SUMS` flow.
8. **Tests**: pure-function unit tests for `parseSHA256Line` cover happy path, missing-filename, and malformed-line cases. Runnable via `node --test npm/postinstall.test.js` (Node 18+ built-in runner, no test framework dependency).
