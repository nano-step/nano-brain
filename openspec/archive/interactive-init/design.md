# Design: Interactive Init Wizard

## Approach

Pure Go stdlib implementation. No TUI framework — use `bufio.Scanner` for input, ANSI for minimal formatting.

## Flow

```
nano-brain init (no flags)
  → detect no --root flag
  → load existing config if present (for defaults)
  → prompt 5 questions with defaults
  → write config.yml via config.GenerateDefault or custom write
  → call runDoctorCmd to verify
```

## Files

- `cmd/nano-brain/init.go`: `runInteractiveInit(configPath string)` function
- `cmd/nano-brain/commands.go`: modify `runInitCmd` — if no `--root`, call interactive init
