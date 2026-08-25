# npm-install-fallback Specification

## Purpose

When the npm postinstall runs at a version with no matching release asset — a source checkout at the `0.0.0-dev` placeholder, or any prerelease string — it must tell the user what it tried and what to do instead, rather than surfacing a bare HTTP status. Integrity enforcement must survive the change untouched.

## ADDED Requirements

### Requirement: Tag planning SHALL be an exported pure function

The logic that turns a package version into candidate release tags SHALL be exported so it can be unit-tested without performing any network request.

#### Scenario: Planner is callable from a test

- **WHEN** a test imports the postinstall module
- **THEN** the tag planner is available on the module exports
- **AND** calling it performs no network access

#### Scenario: Release version resolves on the first candidate

- **WHEN** the planner is called with a published version whose patch segment is four digits
- **THEN** the first candidate tag is the canonical `v<version>` form

#### Scenario: Short-patch version yields a zero-padded candidate

- **WHEN** the planner is called with a version whose patch segment is one to three digits
- **THEN** the candidate list also contains the zero-padded four-digit reconstruction

#### Scenario: Placeholder version yields no usable candidate

- **WHEN** the planner is called with `0.0.0-dev`
- **THEN** it reports that no release tag can be derived from that version

### Requirement: An unresolvable version SHALL produce actionable guidance

When no candidate tag and no API-resolved tag yields a downloadable asset, postinstall SHALL report the tags it attempted and name the supported alternatives: the one-line installer, the `NANO_BRAIN_BIN` override, and building from source.

#### Scenario: Source-checkout install explains itself

- **WHEN** postinstall runs with the package version set to the on-master placeholder
- **THEN** the failure message names the tags that were tried
- **AND** it names the installer script, the binary-override environment variable, and the source build as alternatives
- **AND** it does not present the failure as an unexplained HTTP status

#### Scenario: The diagnostic never recommends skipping verification

- **WHEN** any postinstall failure message is produced
- **THEN** it does not mention the checksum-skip environment variable as a workaround

#### Scenario: The lazy first-invocation path shows the same guidance

- **WHEN** the binary is missing and the npm wrapper triggers a download that cannot resolve a tag
- **THEN** the wrapper prints the same actionable message rather than a bare error

### Requirement: Integrity enforcement SHALL remain hard-failing

A checksum mismatch SHALL continue to delete the downloaded file, propagate as a security failure without being folded into any aggregated diagnostic, and exit non-zero.

#### Scenario: Checksum mismatch is not absorbed by the tag summary

- **WHEN** a candidate tag downloads an asset whose SHA-256 does not match the published sums
- **THEN** the process reports the security failure, removes the downloaded file, and exits non-zero
- **AND** the failure is not reported as part of a "tags tried" summary
- **AND** no further candidate tag is attempted

#### Scenario: API-fallback integrity failure also hard-fails

- **WHEN** the API-resolved tag downloads an asset failing checksum verification
- **THEN** the security failure propagates and the process exits non-zero

### Requirement: The npm test suite SHALL run in continuous integration

The repository CI workflow SHALL execute the Node test suite, so that postinstall regressions are caught before merge.

#### Scenario: CI runs the Node tests

- **WHEN** the CI workflow runs on a pull request
- **THEN** it executes the Node test suite under `npm/`
- **AND** a failing postinstall test fails the build

#### Scenario: Tests need no network or published release

- **WHEN** the Node test suite runs on a machine with no network access
- **THEN** every postinstall test passes or fails on its own logic, with no assertion depending on a live release download
