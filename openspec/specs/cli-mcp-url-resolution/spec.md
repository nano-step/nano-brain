# cli-mcp-url-resolution Specification

## Purpose
TBD - created by archiving change nano-brain-cli-ux-overhaul. Update Purpose after archive.
## Requirements
### Requirement: nano-brain SHALL resolve the MCP URL using a fixed precedence

The nano-brain CLI and any shipped tooling that needs to print or use the MCP URL SHALL resolve it using exactly this precedence, evaluated in order, stopping at the first hit:

1. `NANO_BRAIN_MCP_URL` environment variable, after trimming leading/trailing whitespace, if the result is non-empty.
2. If `/.dockerenv` exists as a file on the filesystem, use `http://host.docker.internal:3100/mcp`.
3. Default: `http://localhost:3100/mcp`.

No probing, no DNS lookup, no HTTP request happens during resolution. Resolution is pure I/O over env var read + one `os.Stat` call.

#### Scenario: Env var wins over container detection
- **WHEN** `NANO_BRAIN_MCP_URL=http://my-vps.example.com:3100/mcp` is set AND `/.dockerenv` exists
- **THEN** resolved URL is `http://my-vps.example.com:3100/mcp`

#### Scenario: Whitespace trimmed from env var
- **WHEN** `NANO_BRAIN_MCP_URL="   http://localhost:9000/mcp  \n"` is set (literal whitespace and newline)
- **THEN** resolved URL is `http://localhost:9000/mcp`

#### Scenario: Empty env var falls through to detection
- **WHEN** `NANO_BRAIN_MCP_URL=""` (set but empty) AND `/.dockerenv` exists
- **THEN** resolved URL is `http://host.docker.internal:3100/mcp`

#### Scenario: Whitespace-only env var falls through to detection
- **WHEN** `NANO_BRAIN_MCP_URL="   "` is set AND `/.dockerenv` does NOT exist
- **THEN** resolved URL is `http://localhost:3100/mcp`

#### Scenario: Container without env var
- **WHEN** `NANO_BRAIN_MCP_URL` is unset AND `/.dockerenv` exists
- **THEN** resolved URL is `http://host.docker.internal:3100/mcp`

#### Scenario: Bare-metal default
- **WHEN** `NANO_BRAIN_MCP_URL` is unset AND `/.dockerenv` does NOT exist
- **THEN** resolved URL is `http://localhost:3100/mcp`

### Requirement: `nano-brain mcp-url` subcommand SHALL print the resolved URL on stdout

A new top-level subcommand `nano-brain mcp-url` SHALL print the URL resolved by the precedence above. The subcommand MUST take no flags and no positional arguments. It SHALL exit 0 on any successful resolution (which always succeeds, since the default is unconditional). It is designed for skill installers and shell substitution.

#### Scenario: Stdout shape
- **WHEN** the user runs `nano-brain mcp-url` in any environment
- **THEN** stdout contains exactly the resolved URL followed by a single newline (`\n`) AND stderr is empty AND exit code is 0

#### Scenario: Shell substitution usage
- **WHEN** the user runs `MCP_URL=$(nano-brain mcp-url)` in a bash shell
- **THEN** the shell variable `MCP_URL` contains the resolved URL with no trailing whitespace

#### Scenario: Unknown flag rejected
- **WHEN** the user runs `nano-brain mcp-url --json` or any other flag
- **THEN** the CLI prints `Usage: nano-brain mcp-url` to stderr and exits with code 1

#### Scenario: Subcommand registered in usage
- **WHEN** the user runs `nano-brain help` or `nano-brain` with no arguments after the binary path
- **THEN** the help output includes a line documenting `mcp-url` as a subcommand

