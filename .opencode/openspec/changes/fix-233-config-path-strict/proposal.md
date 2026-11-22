# Apply Strict Config Path Resolution Across All CLI Commands (#233)

## Why

Commit `c5b6973` (PR #231 for #224) added whitespace trimming to `ResolveConfigPath`
and missing-file warnings via `ResolveConfigPathStrict`, but only wired
`ResolveConfigPathStrict` into `startServer`. All other CLI commands still had
three defects:

1. **Silent fallback on missing explicit config:** `config show` and operational
   commands (`bench`, `db:migrate`, `logs`, `reset-embeddings`,
   `cleanup-stale-raw`, `cleanup-orphan-workspaces`, `backfill-summaries`, and
   HTTP client commands) called `ResolveConfigPath` instead of
   `ResolveConfigPathStrict`, emitting no `WARNING:` on stderr when `--config`
   or `NANO_BRAIN_CONFIG` pointed to a non-existent file.
2. **Untrimmed `--config` flag and unthreaded flag value:** `config show`,
   `doctor`, and `init` guarded `ResolveConfigPath` behind `if configPath == ""`,
   bypassing `TrimSpace` whenever `--config "  /path/to/config.yml  "` was set,
   while `bench`, `db:migrate`, `logs`, `reset-embeddings`, `cleanup-*`,
   `backfill-summaries`, and `client.go` hardcoded `ResolveConfigPath("")`.
3. **Misleading `Config... OK` in `doctor`:** When `--config` or
   `NANO_BRAIN_CONFIG` pointed to a non-existent file, `config.Load` fell back
   to defaults with a nil error, causing `doctor` to report `Config... OK`
   instead of `FAIL` ("file does not exist").

## What

- Trim whitespace (and expand leading `~`) consistently in `ResolveConfigPath`,
  `ResolveConfigPathStrict`, and `config.Load`, and never bypass resolution
  when `--config` is non-empty.
- Thread `--config` across all CLI subcommands and `client.go`, emitting
  `WARNING:` on stderr via `ResolveConfigPathStrict` when an explicit `--config`
  or `NANO_BRAIN_CONFIG` file does not exist (except `init`, which creates the
  config file when missing).
- Fail the `Config` check in `doctor` (and `config check`) with `"file does not
  exist"` + stderr `WARNING:` when an explicit config path is missing, while
  preserving `OK` for zero-config defaults when neither `--config` nor
  `NANO_BRAIN_CONFIG` is set.
