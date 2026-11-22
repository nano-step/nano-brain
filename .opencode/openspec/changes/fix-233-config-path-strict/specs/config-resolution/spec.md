# Config Resolution

## ADDED Requirements

### Requirement: Whitespace-trimmed config path resolution and flag propagation across all CLI commands
Every CLI command that loads local configuration or resolves the server endpoint SHALL
resolve `--config` and `NANO_BRAIN_CONFIG` through `ResolveConfigPath` or
`ResolveConfigPathStrict` without bypassing resolution when `--config` is
non-empty, trimming surrounding whitespace and honoring `--config` precedence
over `NANO_BRAIN_CONFIG` and `~/.nano-brain/config.yml`.

#### Scenario: Padded --config flag loads target config across CLI commands
- **WHEN** the user invokes `nano-brain --config "  /tmp/custom/config.yml  "` with `config show`, `doctor`, `init --yes`, `status`, `logs`, `bench`, `db:migrate`, `reset-embeddings`, `backfill-summaries`, `cleanup-stale-raw`, or `cleanup-orphan-workspaces`
- **THEN** surrounding whitespace is trimmed, `/tmp/custom/config.yml` is used, and no spurious missing-file warning is emitted when the file exists

### Requirement: Missing explicit config emits stderr warning across non-init CLI commands
Non-`init` CLI commands that read configuration SHALL emit a `WARNING:` message
on stderr whenever `--config` or `NANO_BRAIN_CONFIG` is explicitly set to a
non-existent file, identifying the source (`--config flag` or
`NANO_BRAIN_CONFIG env`) and resolved path before falling back to defaults, while
emitting no warning when neither source is set and `~/.nano-brain/config.yml`
does not exist.

#### Scenario: config show warns on stderr when NANO_BRAIN_CONFIG or --config is missing
- **WHEN** `nano-brain config show` (with or without `--json`) runs with `NANO_BRAIN_CONFIG=/tmp/does-not-exist.yml` or `--config /tmp/does-not-exist.yml`
- **THEN** `WARNING:` naming the source and `/tmp/does-not-exist.yml` is written to stderr while stdout displays the redacted defaults or JSON output

### Requirement: Doctor fails Config check when explicit config file does not exist
`nano-brain doctor` (and `nano-brain config check`) SHALL report `FAIL` with a
`"file does not exist"` hint and emit `WARNING:` on stderr when `--config` or
`NANO_BRAIN_CONFIG` points to a non-existent file, while continuing to report
`OK` with empty stderr when neither `--config` nor `NANO_BRAIN_CONFIG` is set
and the default `~/.nano-brain/config.yml` file is absent.

#### Scenario: Doctor distinguishes missing explicit config from zero-config default
- **WHEN** `nano-brain doctor` runs with `NANO_BRAIN_CONFIG=/tmp/does-not-exist.yml` or `--config /tmp/does-not-exist.yml`, versus with both unset and `~/.nano-brain/config.yml` absent
- **THEN** the explicit missing-path invocations exit non-zero, write `WARNING:` to stderr, and report `Config` status `fail` with `"file does not exist"`, whereas the unset default invocation writes nothing to stderr and reports `Config` status `ok`
