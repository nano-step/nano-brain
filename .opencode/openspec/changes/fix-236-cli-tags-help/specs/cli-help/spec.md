# CLI Help

## ADDED Requirements

### Requirement: Per-command help and usage for `query`, `search`, and `vsearch` document all supported flags
The `nano-brain query`, `nano-brain search`, and `nano-brain vsearch` CLI commands SHALL accept `-h` and `--help` (printing usage to stdout and exiting `0` without issuing an HTTP request), and SHALL list `--workspace`, `--scope`, `--tags`, `--created-after`, `--created-before`, `--updated-after`, `--updated-before`, and `--json` in both explicit help output and missing-argument usage output on stderr.

#### Scenario: Explicit `--help` or `-h` on search subcommands
- **WHEN** a user invokes `nano-brain query --help`, `nano-brain query -h` (even alongside other flags such as `--scope=all`), `nano-brain search --help`, or `nano-brain vsearch --help`
- **THEN** the CLI writes usage containing `--workspace`, `--scope`, `--tags`, `--created-after`, `--created-before`, `--updated-after`, `--updated-before`, and `--json` to stdout, writes nothing to stderr, makes no HTTP request, and exits with code `0`

#### Scenario: Missing required arguments on search subcommands
- **WHEN** a user invokes `nano-brain query`, `nano-brain search`, or `nano-brain vsearch` without a query or required workspace
- **THEN** the CLI writes usage containing `--workspace`, `--scope`, `--tags`, `--created-after`, `--created-before`, `--updated-after`, `--updated-before`, and `--json` to stderr, writes nothing to stdout, and exits with code `1`

### Requirement: Top-level `nano-brain help` lists `--tags` on search subcommands
The top-level `printUsage` output (`nano-brain help` / `nano-brain --help`) SHALL include `--tags` alongside `--scope` and `--workspace` on the summary lines for `query`, `search`, and `vsearch`.

#### Scenario: Top-level help includes `--tags` on `query`, `search`, and `vsearch`
- **WHEN** `printUsage` renders the top-level CLI command table
- **THEN** each of the `query`, `search`, and `vsearch` lines contains `--workspace`, `--scope`, and `--tags`
