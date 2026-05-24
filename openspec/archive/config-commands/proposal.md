Tracking: #130

## Why

Users need to inspect and validate their nano-brain configuration without reading YAML files directly. Two subcommands address this:
- `config show` — display current effective config (merged defaults + file + env)
- `config check` — validate that configured services are reachable

## What Changes

### New: `nano-brain config show`
- Prints effective config as formatted YAML
- Masks sensitive values (database password, API keys)
- Supports `--json` flag
- Reads config from `--config` flag or default path

### New: `nano-brain config check`
- Validates config file exists and parses
- Tests PostgreSQL connection (3s timeout)
- Tests pgvector extension (if PG reachable)
- Tests embedding provider reachability
- Tests embedding model availability
- Supports `--json` flag
- Exit 0 if all pass, exit 1 if any fail
- Reuses doctor check logic where possible

### Modified: `cmd/nano-brain/main.go`
- Add `case "config"` to dispatch switch → routes to `runConfigCmd`

### New file: `cmd/nano-brain/config_cmd.go`
- `runConfigCmd(args, configPath)` — dispatches to show/check
- `runConfigShow(configPath, jsonFlag)` — loads and prints config
- `runConfigCheck(configPath, jsonFlag)` — runs validation checks
