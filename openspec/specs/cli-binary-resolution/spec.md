# cli-binary-resolution Specification

## Purpose
TBD - created by archiving change nano-brain-cli-ux-overhaul. Update Purpose after archive.
## Requirements
### Requirement: `nano-brain version --which` SHALL report resolved binary path, version, and invocation source

A new `--which` flag on the existing `version` subcommand SHALL print the absolute path of the running binary, its compile-time version, and the invocation source category (`npm-local`, `npm-global`, `dev-build`, `path`, or `env-override`). Without `--which`, the existing `version` output MUST remain unchanged.

#### Scenario: npx invocation reports `npm-local`
- **WHEN** the user runs `nano-brain version --which` and the binary's parent directory matches `*/node_modules/.bin/` or `*/node_modules/@nano-step/nano-brain/*` AND `os.Getenv("npm_execpath") != ""`
- **THEN** stdout contains three lines: `path: <absolute-path>`, `version: <Version>`, `source: npm-local`

#### Scenario: Global npm install reports `npm-global`
- **WHEN** the user runs `nano-brain version --which` and the binary's parent directory contains the npm global prefix (resolved by `npm root -g` heuristic, or matches `/usr/local/lib/node_modules/`, `~/.npm-global/`, or `~/.local/lib/node_modules/`)
- **THEN** stdout contains `source: npm-global`

#### Scenario: Source build reports `dev-build`
- **WHEN** the user runs `nano-brain version --which` and `Version == "dev"` (the default for `go build` without ldflags)
- **THEN** stdout contains `source: dev-build`

#### Scenario: PATH-resolved binary reports `path`
- **WHEN** the user runs `nano-brain version --which` and none of the above categories match AND the binary is found via `$PATH`
- **THEN** stdout contains `source: path`

#### Scenario: `NANO_BRAIN_BIN` override reports `env-override`
- **WHEN** the user runs `nano-brain version --which` and the binary's absolute path equals the value of `NANO_BRAIN_BIN`
- **THEN** stdout contains `source: env-override`

#### Scenario: `--which --json` machine-readable output
- **WHEN** the user runs `nano-brain version --which --json`
- **THEN** stdout contains a single JSON object `{"path": "...", "version": "...", "source": "..."}` with no surrounding text

### Requirement: `NANO_BRAIN_BIN` env var SHALL override binary resolution with strict validation

When `NANO_BRAIN_BIN` is set to a non-empty value, the nano-brain CLI (in any invocation context that re-execs the binary, including the npm `run.js` shim) SHALL use that path as the binary to execute. The path MUST be validated: file exists AND has at least one executable bit (`mode & 0111 != 0`). On validation failure, the CLI exits non-zero with a clear error message naming the env var. There is no silent fallback.

#### Scenario: Valid override
- **WHEN** `NANO_BRAIN_BIN=/usr/local/bin/nano-brain-custom` is set AND the file exists AND is executable AND the user runs any `nano-brain ...` command via the npm shim
- **THEN** the shim execs the path from `NANO_BRAIN_BIN` and the command runs normally

#### Scenario: Override file missing
- **WHEN** `NANO_BRAIN_BIN=/nonexistent/path` is set AND the user runs `nano-brain version`
- **THEN** the shim prints `Error: NANO_BRAIN_BIN points to /nonexistent/path which does not exist. Unset the variable or correct the path.` to stderr and exits with code 1

#### Scenario: Override file not executable
- **WHEN** `NANO_BRAIN_BIN=/tmp/nano-brain-build` is set AND the file exists AND has mode 0644 (no executable bit) AND the user runs `nano-brain version`
- **THEN** the shim prints `Error: NANO_BRAIN_BIN points to /tmp/nano-brain-build which is not executable. Run: chmod +x /tmp/nano-brain-build` to stderr and exits with code 1

#### Scenario: Override env var unset uses default resolution
- **WHEN** `NANO_BRAIN_BIN` is not set in the environment
- **THEN** binary resolution falls back to the default npm-bin path or PATH lookup with no validation override behaviour

#### Scenario: Override is reported by `version --which`
- **WHEN** `NANO_BRAIN_BIN=/custom/path/nano-brain` is set AND the user runs `nano-brain version --which`
- **THEN** stdout contains `source: env-override` and `path: /custom/path/nano-brain`

