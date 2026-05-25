# Spec: CLI `workspaces` Command

## ADDED Requirements

### Requirement: `workspaces` command dispatch
The CLI MUST recognize a new top-level command `workspaces` that lists registered workspaces.

#### Scenario: Bare `workspaces`
- Given the nano-brain server is reachable
- When the user runs `nano-brain workspaces`
- Then it behaves identically to `nano-brain workspaces list`

#### Scenario: Explicit `list` subcommand
- When the user runs `nano-brain workspaces list`
- Then the CLI calls `GET /api/v1/workspaces` and renders results

#### Scenario: `ls` alias
- When the user runs `nano-brain workspaces ls`
- Then it behaves identically to `nano-brain workspaces list`

#### Scenario: Top-level `--json` without subcommand
- When the user runs `nano-brain workspaces --json`
- Then it behaves identically to `nano-brain workspaces list --json`

### Requirement: Human-readable table output
When `--json` is NOT set, the CLI MUST render a human-readable table to stdout.

#### Scenario: Non-empty list, default format
- Given the server returns two workspaces with hashes, names, paths, doc counts, and timestamps
- When `nano-brain workspaces` runs
- Then stdout contains a header row with columns `HASH`, `NAME`, `PATH`, `DOCS`, `LAST UPDATE`
- And stdout contains one data row per workspace
- And the HASH column shows the first 10 characters of the workspace hash followed by `..`
- And the DOCS column shows the integer document count right-aligned
- And the LAST UPDATE column shows `YYYY-MM-DD` or `never`

#### Scenario: Path longer than column width
- Given a workspace path of 80 characters
- When `nano-brain workspaces` runs
- And the PATH column width is 50 characters
- Then the rendered path starts with `..` and preserves the rightmost portion (so the trailing project/file name remains visible)

#### Scenario: `null` last_document_updated
- Given a workspace has never had a document written (`last_document_updated == null` in the API response)
- When `nano-brain workspaces` runs
- Then the LAST UPDATE column shows the literal string `never`

### Requirement: JSON passthrough
When `--json` is set, the CLI MUST write the API response body verbatim to stdout.

#### Scenario: --json with non-empty result
- Given the server returns a JSON array of workspace objects
- When `nano-brain workspaces --json` runs
- Then stdout contains the exact JSON the server returned (no reformatting, no extra wrapping)
- And stdout ends with a trailing newline
- And stderr is empty

#### Scenario: --json with empty result
- Given the server returns `[]`
- When `nano-brain workspaces --json` runs
- Then stdout contains exactly `[]\n`
- And stderr is empty
- And the exit code is 0

### Requirement: Empty result messaging
When the server returns an empty list and `--json` is NOT set, the CLI MUST inform the user without printing a confusing empty table.

#### Scenario: Empty list, default format
- Given the server returns `[]`
- When `nano-brain workspaces` runs
- Then stderr contains the line `No workspaces registered.`
- And stdout is empty (no table header)
- And the exit code is 0

### Requirement: Server error propagation
When the server returns a non-2xx status, the CLI MUST exit non-zero with a readable error.

#### Scenario: Server returns 500
- Given the server returns HTTP 500 with an error JSON body
- When `nano-brain workspaces` runs
- Then the CLI exits with a non-zero code
- And stderr contains a message describing the server error

#### Scenario: Server unreachable
- Given the nano-brain server is not running
- When `nano-brain workspaces` runs
- Then the CLI uses the existing connect-error UX from proposal #1 (formatted error + suggestion + optional auto-start prompt)
