# Design

## Config Path Resolution & Propagation

1. **`internal/config/config.go` (`ResolveConfigPath`, `ResolveConfigPathStrict`, `Load`):**
   - `ResolveConfigPath`, `ResolveConfigPathStrict`, and `Load` share
     `expandTildeForConfig` (`"~"`, `"~/..."`, and `"~\\..."`, leaving
     `"~otheruser/..."` untouched) after `strings.TrimSpace` so `os.Stat` and
     `os.ReadFile` always agree with `config.Load` while preserving all `#674`
     validation, host normalization, and secret redaction invariants.

2. **Unified CLI Resolution (`cmd/nano-brain/`):**
   - Store the parsed global `--config` flag in `cliConfigPath` in `main.go`,
     reset warning deduplication state via `resetClientConfigWarnState()` after
     `flag.Parse()`, and thread `configPath` into `runBenchCmd`,
     `runDBMigrateCmd`, `runLogsCmd`, `runResetEmbeddingsCmd`,
     `runBackfillSummariesCmd`, `runCleanupStaleRawCmd`, and
     `runCleanupOrphanWorkspacesCmd`.
   - `resolveCLIConfigPath` and `resolveConfigPathOnce` /
     `resolveClientConfigPath` in `config_cmd.go` wrap
     `config.ResolveConfigPathStrict`, clearing `lastClientConfigWarn` whenever
     `warn == ""` so repeated in-process tests or healthy calls reset state
     cleanly while repeated HTTP client probes within a single command emit at
     most one `WARNING:`.
   - `recoverFromConnectionRefused` passes raw `cliConfigPath` to
     `runServeDaemonFn` so zero-config auto-start never mistakes
     `DefaultConfigPath()` for an explicit `--config` flag or misattributes
     `NANO_BRAIN_CONFIG` warnings.
   - `runConfigShow`, `startServer`, `runServeDaemon`, `runDoctorCmd`, and all
     direct-DB/log commands call `resolveCLIConfigPath(configPath)`
     unconditionally (removing `if configPath == ""` guards).
   - `runInteractiveInit` and `runNonInteractiveInit` call
     `config.ResolveConfigPath(configPath)` unconditionally so `--config` with
     whitespace is trimmed without emitting a missing-file warning before `init`
     writes the new file.

3. **`doctor` Explicit Missing-Config Failure (`cmd/nano-brain/doctor.go`, `internal/health/doctor/doctor.go`):**
   - When `resolveCLIConfigPath(configPath)` reports a non-empty warning (meaning
     `--config` or `NANO_BRAIN_CONFIG` was explicitly set and `os.Stat` failed)
     and `config.Load` returned `nil` error via default fallback, pass the
     `os.Stat` error (`os.ErrNotExist` or underlying stat error) into
     `doctor.RunAll`.
   - `doctor.CheckConfig` reports `Status: "fail"` with `Hint: "file does not
     exist — check --config or NANO_BRAIN_CONFIG"` when `errors.Is(err, os.ErrNotExist)`.
   - When neither `--config` nor `NANO_BRAIN_CONFIG` is set and
     `~/.nano-brain/config.yml` does not exist, `configWarn` is `""`, `cfgErr`
     remains `nil`, and `Config` continues to report `Status: "ok"`.
