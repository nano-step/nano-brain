# Spec: npm Scoped Package + CI Auto-Publish

## ADDED Requirements

### Requirement: Scoped package identity
The npm package MUST be published as `@nano-step/nano-brain` with public access.

#### Scenario: Scoped package install
- Given `@nano-step/nano-brain` is published
- When user runs `npx @nano-step/nano-brain@beta`
- Then the binary downloads and runs successfully

### Requirement: Dual publish on release
The CI MUST publish both `@nano-step/nano-brain` and `nano-brain` on every tag push.

#### Scenario: Beta tag push
- Given a tag `v2.0.0-beta.6` is pushed
- When release workflow runs
- Then `@nano-step/nano-brain@2.0.0-beta.6` is published with `--tag beta`
- And `nano-brain@2.0.0-beta.6` is published with `--tag beta`

#### Scenario: Stable tag push
- Given a tag `v2.1.0` is pushed
- When release workflow runs
- Then `@nano-step/nano-brain@2.1.0` is published with `--tag latest`
- And `nano-brain@2.1.0` is published with `--tag latest`

### Requirement: Version from git tag
The CI MUST derive npm version from the git tag, not from package.json.

#### Scenario: Version sync
- Given package.json has version `0.0.0-dev`
- And git tag is `v2.0.0-beta.6`
- When release workflow runs
- Then npm package is published as version `2.0.0-beta.6`

## MODIFIED Requirements

### Requirement: Cleanup stub workflows
The project MUST NOT contain unused stub workflow files.

#### Scenario: Stubs removed
- Given `publish-beta.yml` and `publish-stable.yml` exist as stubs
- When this change is applied
- Then both files are deleted
