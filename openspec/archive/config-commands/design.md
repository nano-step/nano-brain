## Design

### Architecture

`config` is a top-level command with subcommands `show` and `check`.

```
main.go switch → case "config" → runConfigCmd(args, configPath)
  → "show"  → runConfigShow(configPath, jsonFlag)
  → "check" → runConfigCheck(configPath, jsonFlag)
```

### New file: `cmd/nano-brain/config_cmd.go`

~100 lines. Three functions:

1. **runConfigCmd** — parse subcommand + flags, dispatch
2. **runConfigShow** — load config via `config.Load()`, mask sensitive fields, print YAML or JSON
3. **runConfigCheck** — load config, reuse doctor check functions, print results

### Masking

Password masking for database URL: regex replace password portion with `***`.
```
postgres://user:password@host → postgres://user:***@host
```

### Reusing doctor checks

`doctor.go` currently has check functions that print directly. To reuse:
- Extract check logic is not needed — `config check` simply calls `runDoctorCmd([]string{}, configPath)` directly
- This avoids duplication entirely

Actually, `config check` IS functionally identical to `doctor`. The difference:
- `doctor` = "are prerequisites installed?"
- `config check` = "is my config valid and services reachable?"

They do the same thing. So `config check` SHALL be an alias for `doctor`.

### Dependencies
- No new dependencies
- stdlib only (fmt, os, strings, regexp, encoding/json)
- Reuses `internal/config` package
