# skill-distribution-docs Specification

## Purpose
TBD - created by archiving change nano-brain-cli-ux-overhaul. Update Purpose after archive.
## Requirements
### Requirement: User-facing docs SHALL lead with the fast invocation path

`README.md` Quick Start, `.opencode/skills/nano-brain/SKILL.md`, and the SKILL.md shipped via `@nano-step/skill-manager` SHALL present invocation options in this order: (1) MCP (zero per-call overhead, default for agents), (2) `npm install -g` + `nano-brain ...` (cold-start ≤ 50 ms), (3) `npx @nano-step/nano-brain ...` (documented as fallback only). The order MUST NOT be reversed; `npx` MUST NOT appear before `npm install -g` in any new or modified Quick Start section.

#### Scenario: README Quick Start ordering
- **WHEN** a user reads `README.md` Quick Start top-to-bottom
- **THEN** the first invocation example uses MCP or `npm install -g`, and any `npx` example appears under a section explicitly labeled as fallback / no-Node-install case

#### Scenario: SKILL.md ordering
- **WHEN** an agent reads `SKILL.md` (either copy) top-to-bottom
- **THEN** the MCP transport is documented before any CLI invocation, and `npm install -g` is documented before `npx`

### Requirement: All `@beta` references SHALL be replaced with `@latest` in user-facing surfaces

Every user-facing string that names an npm tag for `@nano-step/nano-brain` or `nano-brain` (unscoped alias) SHALL use `@latest`, not `@beta`. Affected surfaces include but are not limited to: `README.md`, `.opencode/skills/nano-brain/SKILL.md`, `cmd/nano-brain/client_helpers.go:suggestStartCommand()`, any string literal in `cmd/nano-brain/*_test.go` that asserts on the suggested start command, and the SKILL.md shipped via skill-manager.

#### Scenario: No `@beta` in README
- **WHEN** a user runs `grep -n '@beta' README.md`
- **THEN** the command returns no matches

#### Scenario: No `@beta` in SKILL.md (either copy)
- **WHEN** a maintainer runs `grep -n '@beta' .opencode/skills/nano-brain/SKILL.md` and the same grep against the skill-manager-shipped SKILL.md
- **THEN** both commands return no matches

#### Scenario: `suggestStartCommand` returns `@latest`
- **WHEN** the test `TestSuggestStartCommand_NpxLaunch` (or equivalent) calls `suggestStartCommand()` with `npm_execpath` set
- **THEN** the returned string contains `@latest` and does not contain `@beta`

### Requirement: SKILL.md SHALL document the documented escape hatches and platform workarounds

The shipped `SKILL.md` (both the project-local copy at `.opencode/skills/nano-brain/SKILL.md` and the skill-manager-shipped copy) SHALL include a Troubleshooting section with at minimum the following entries:

- `NANO_BRAIN_SKIP_SHA_VERIFY=1` — when and why to use it (corp proxy, air-gapped installs).
- `npm install -g --prefix ~/.local` — how to install globally without sudo.
- macOS Gatekeeper quarantine workaround — `xattr -dr com.apple.quarantine <path>` and when to apply it.
- `NANO_BRAIN_MCP_URL` env var — how to override the MCP URL (with one container-mode example and one bare-metal example).
- `NANO_BRAIN_BIN` env var — how to point at a custom binary; failure modes documented.

#### Scenario: SHA-verify skip documented
- **WHEN** a user searches SKILL.md for `NANO_BRAIN_SKIP_SHA_VERIFY`
- **THEN** the surrounding text explains both when to use it (corp proxy modifying download stream) and the security trade-off (skips integrity check)

#### Scenario: Non-root install documented
- **WHEN** a user searches SKILL.md for `--prefix`
- **THEN** the surrounding text shows the exact command `npm install -g --prefix ~/.local @nano-step/nano-brain@latest` and notes the resulting binary path

#### Scenario: macOS Gatekeeper documented
- **WHEN** a user searches SKILL.md for `xattr` or `Gatekeeper` or `quarantine`
- **THEN** the surrounding text shows the exact remediation command and notes when it applies (downloaded binaries on macOS)

#### Scenario: `NANO_BRAIN_MCP_URL` documented with both modes
- **WHEN** a user searches SKILL.md for `NANO_BRAIN_MCP_URL`
- **THEN** the surrounding text shows at minimum one container example (`http://host.docker.internal:3100/mcp`) and one bare-metal example (`http://localhost:3100/mcp`) and notes the OpenCode `{env:NANO_BRAIN_MCP_URL}` syntax used in skill.json

### Requirement: skill.json MCP URL SHALL use OpenCode env-var substitution

`.opencode/skills/nano-brain/skill.json` (and the skill-manager-shipped copy) SHALL use `{env:NANO_BRAIN_MCP_URL}` for the MCP server URL field, not a hardcoded host. This delivers Phase 3 of the proposal — Phase 0 investigation confirmed OpenCode supports this syntax via `ConfigVariable.substitute`.

#### Scenario: skill.json MCP URL uses env-var syntax
- **WHEN** a maintainer reads `.opencode/skills/nano-brain/skill.json`
- **THEN** the `mcp.nano-brain.url` field value is exactly `{env:NANO_BRAIN_MCP_URL}` and not a hardcoded URL

#### Scenario: skill-manager-shipped skill.json matches
- **WHEN** a user installs the skill via `@nano-step/skill-manager` and inspects the installed skill.json
- **THEN** the `mcp.nano-brain.url` field value matches the project-local copy (uses `{env:NANO_BRAIN_MCP_URL}`)

