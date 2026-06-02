# release-pipeline Delta — SHA-256 integrity verification

## ADDED Requirements

### Requirement: Release workflow publishes SHA-256 checksums alongside binaries

The `release` job in `.github/workflows/release.yml` SHALL compute SHA-256 hashes over every binary produced by the `build` matrix and publish a single `SHA256SUMS` file as a GitHub Release asset alongside the binaries. The file SHALL use GNU coreutils `sha256sum` text format: one line per binary, each line of the form `<64-hex-chars><two-spaces><filename>\n`, with bare filenames (no path prefix).

The checksum file MUST be generated AFTER `actions/download-artifact@v4` consolidates all build outputs into the `artifacts/` directory and BEFORE `softprops/action-gh-release@v2` uploads the release. The existing `files: artifacts/*` glob SHALL pick up `SHA256SUMS` automatically without modification.

The workflow MUST NOT generate SHA-256 hashes inside per-platform build jobs (the `release` job on `ubuntu-latest` is the single point of computation — `sha256sum` from GNU coreutils is preinstalled there).

#### Scenario: SHA256SUMS asset present in release

- **GIVEN** a commit pushed to `master` that triggers the auto-tag → release pipeline
- **WHEN** `.github/workflows/release.yml` runs to completion
- **THEN** the resulting GitHub Release at `https://github.com/nano-step/nano-brain/releases/tag/v<YYYY>.<M>.<D>.<N>` has 5 assets: 4 binaries (`nano-brain-linux-amd64`, `nano-brain-linux-arm64`, `nano-brain-darwin-amd64`, `nano-brain-darwin-arm64`) PLUS one `SHA256SUMS` file
- **AND** the `SHA256SUMS` file contains exactly 4 lines, each matching the regex `^[a-f0-9]{64}  nano-brain-(linux|darwin)-(amd64|arm64)$`

#### Scenario: Checksum file is verifiable with standard tools

- **GIVEN** a release with a published `SHA256SUMS` asset and its associated binaries
- **WHEN** a user runs `curl -fLO https://.../SHA256SUMS` and `curl -fLO https://.../nano-brain-linux-amd64` and then `sha256sum -c SHA256SUMS --ignore-missing`
- **THEN** `sha256sum` reports `nano-brain-linux-amd64: OK`
- **AND** the exit code is 0

### Requirement: npm postinstall verifies binary integrity against published SHA-256

`npm/postinstall.js` SHALL verify the SHA-256 hash of every downloaded binary against the value published in the release's `SHA256SUMS` asset BEFORE `chmod 0755` is applied. The verification MUST use Node's built-in `crypto.createHash('sha256')`; no third-party hashing library is permitted.

The hash MUST be computed during the download via a streaming tee (the HTTPS response piped through both `crypto.createHash('sha256')` and `fs.createWriteStream` in parallel) rather than by reading the binary file twice. Implementations MUST NOT add a second disk read of the downloaded binary purely for hashing.

The script MUST treat the failure modes as follows:

| Condition | Action |
|---|---|
| SHA256SUMS download succeeds AND parses AND hash matches | Proceed silently to chmod |
| SHA256SUMS download succeeds AND parses AND hash mismatches | `fs.unlinkSync(binPath)`; throw with a SECURITY message containing both expected and computed hashes; exit code 1 |
| SHA256SUMS download fails (404 / 5xx / network error) | `console.warn` a single line; proceed to chmod without verification |
| SHA256SUMS parses but `assetName` not listed | `console.warn` a single line; proceed to chmod without verification |
| Environment variable `NANO_BRAIN_SKIP_SHA_VERIFY` is truthy at process start | `console.warn` a single line; skip verification entirely; proceed to chmod |

The verification step MUST integrate with BOTH existing download paths in `postinstall.js`: the candidate-tag retry loop AND the GitHub-API tag-resolution fallback.

The script MUST NOT modify the existing version-normalization logic (`candidateTagsForVersion`, `normalizeVersion`, `resolveTagFromAPI`).

#### Scenario: Hash matches — install succeeds silently

- **GIVEN** a published release with `SHA256SUMS` listing `nano-brain-linux-amd64  <hex>`
- **AND** the binary downloaded from that release computes to `<hex>` via SHA-256
- **WHEN** `npm install @nano-step/nano-brain` runs `npm/postinstall.js`
- **THEN** the install completes with exit code 0
- **AND** stdout contains `nano-brain v<version> installed successfully from v<YYYY>.<M>.<D>.<N>`
- **AND** stdout does NOT contain any WARN line about SHA verification

#### Scenario: Hash mismatch — install aborts

- **GIVEN** a binary download whose SHA-256 does NOT match the value listed in the release's SHA256SUMS (corruption, tampering, or wrong file)
- **WHEN** `npm install @nano-step/nano-brain` runs `npm/postinstall.js`
- **THEN** the postinstall script exits with code 1
- **AND** stderr contains the literal substring `SECURITY: SHA-256 mismatch`
- **AND** stderr contains both the expected and computed hashes
- **AND** the partially-downloaded binary at the install path has been removed via `fs.unlinkSync`

#### Scenario: Old release without SHA256SUMS — install proceeds with WARN

- **GIVEN** a release tagged BEFORE this change merges, which has 4 binaries but no SHA256SUMS asset (returns 404)
- **WHEN** `npm install @nano-step/nano-brain@<old-version>` runs the (new) `npm/postinstall.js`
- **THEN** the install completes with exit code 0
- **AND** stdout/stderr contains a single WARN line indicating SHA256SUMS was not found for the release and verification was skipped
- **AND** the binary is installed and `chmod 0755` is applied

#### Scenario: Transient SHA256SUMS network failure — install proceeds with WARN

- **GIVEN** a release that DOES have a SHA256SUMS asset
- **AND** the SHA256SUMS download fails for transient reasons (5xx response, DNS error, connection reset)
- **WHEN** `npm install @nano-step/nano-brain` runs `npm/postinstall.js`
- **THEN** the install completes with exit code 0
- **AND** stdout/stderr contains a single WARN line indicating SHA256SUMS could not be fetched and verification was skipped
- **AND** the binary is installed and `chmod 0755` is applied

This scenario shares its code path with "Old release without SHA256SUMS" — both surface as failures of the SHA256SUMS download step inside `verifySHA256` and degrade gracefully. Hash MISMATCH (proven tampering) is distinct from hash UNAVAILABLE (couldn't run the check).

#### Scenario: Escape hatch — verification disabled by env var

- **GIVEN** the environment variable `NANO_BRAIN_SKIP_SHA_VERIFY=1`
- **WHEN** `npm install @nano-step/nano-brain` runs `npm/postinstall.js`
- **THEN** the install completes with exit code 0
- **AND** stdout/stderr contains exactly ONE WARN line indicating `NANO_BRAIN_SKIP_SHA_VERIFY` is set
- **AND** no SHA256SUMS download is attempted
- **AND** the binary is installed and `chmod 0755` is applied

### Requirement: README documents the SHA-256 verification flow

`README.md` SHALL contain a subsection (immediately after the Quick Start section) titled `Verifying Downloads` that documents:

1. Where to find the `SHA256SUMS` asset (in every release after this change merges).
2. The standard verification command: `sha256sum -c SHA256SUMS --ignore-missing` after downloading both the binary and SHA256SUMS into the same directory.
3. That `npm install` performs this verification automatically.
4. The `NANO_BRAIN_SKIP_SHA_VERIFY=1` escape hatch for corporate-proxy / air-gapped environments.
5. A pointer to issue #320 (or the archived OpenSpec change) for the rationale.

#### Scenario: Manual verification follows the documented flow

- **GIVEN** the README "Verifying Downloads" subsection
- **WHEN** a user follows the documented `curl` + `sha256sum -c` flow
- **THEN** `sha256sum` reports each downloaded binary as `OK`
- **AND** the exit code is 0
