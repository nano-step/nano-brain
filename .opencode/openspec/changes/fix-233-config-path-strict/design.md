# Design

## Config Path Resolution & Propagation

1. **`internal/config/config.go` (`ResolveConfigPath`, `ResolveConfigPathStrict`, `Load`):**
   - `ResolveConfigPath` and `ResolveConfigPathStrict` trim surrounding
     whitespace on both `flagValue` and `NANO_BRAIN_CONFIG`, and expand leading
     `~`/`~/` via `expandTildeForConfig` before returning or calling `os.Stat`
     so `os.Stat` and `os.ReadFile` agree with `config.Load`.
   - `Load(configPath)` also trims `configPath` as defense-in-depth while
     preserving all `#674` validation, host normalization, and secret redaction
     invariants.

2. **Unified CLI Resolution (`cmd/nano-brain/`):**
   - Store the parsed global `--config` flag in `cliConfigPath` in `main.go` and
     thread `configPath` into `runBenchCmd`, `runDBMigrateCmd`, `runLogsCmd`,
     `runResetEmbeddingsCmd`, `runBackfillSummariesCmd`, `runCleanupStaleRawCmd`,
     and `runCleanupOrphanWorkspacesCmd`.
   - Add `resolveCLIConfigPath(flagValue string) (string, string)` in
     `config_cmd.go` wrapping `config.ResolveConfigPathStrict`: prints non-empty
     `WARNING:` to `os.Stderr` and records the emitted warning so downstream
     HTTP client calls (`resolveHostPort` in `client.go`) do not duplicate the
     same warning on stderr within a single command invocation.
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
