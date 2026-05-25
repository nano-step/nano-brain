# Harvester Auto-Detect session_dir

## Problem

After running `nano-brain init` (interactive setup), the server logs on every start:

```
{"level":"info","message":"opencode session harvester disabled (no session_dir configured)"}
```

Users who have OpenCode installed get this message and don't know how to fix it. The root causes:

1. **The init wizard never asks about `session_dir`.** The wizard generates a config with `server`, `database`, `embedding`, `search`, `watcher`, and `logging` sections — but no `harvester` section. Users who want session harvesting must manually edit `~/.nano-brain/config.yml` or set `OPENCODE_STORAGE_DIR`.

2. **The server never tries well-known paths.** There are no platform defaults in code — only one example in README docs. If `harvester.opencode.session_dir == ""`, the server immediately disables the harvester without checking whether OpenCode is actually installed.

3. **`OPENCODE_STORAGE_DIR` env var is the only documented workaround**, but: (a) it requires an edit to shell profile, (b) it isn't mentioned at init time, and (c) deep-nested env var override `NANO_BRAIN_HARVESTER_OPENCODE_SESSION_DIR` doesn't work due to koanf's key-flattening.

## Solution

Two complementary changes:

### A. Platform auto-detect at server startup

Before checking `cfg.Harvester.OpenCode.SessionDir`, probe platform-specific well-known paths for OpenCode storage in order:
- All platforms: `OPENCODE_STORAGE_DIR` env var (already wired, keep)
- Linux: `$XDG_DATA_HOME/opencode/storage` → fallback `$HOME/.local/share/opencode/storage`
- macOS: `$HOME/Library/Application Support/opencode/storage`
- Windows: `%APPDATA%\opencode\storage`

"Probe" means: `os.Stat(path)` succeeds. If a candidate is found AND `cfg.Harvester.OpenCode.SessionDir == ""`, auto-use it and log: `"auto-detected opencode storage at <path>"`. Behavior is fallback — if detection fails, current behavior preserved exactly.

### B. Init wizard includes harvester block

After the workspace-registration prompt, add:
- Auto-detect the same platform paths.
- If found: inform user `"OpenCode detected at <path>"` + prompt `"Enable session harvesting? [Y/n]:"`.
- If user accepts: write `harvester.opencode.session_dir: <path>` into the generated YAML.
- If NOT found: skip the prompt entirely — no nag for users without OpenCode.
- If `OPENCODE_STORAGE_DIR` is set: use that path as the candidate.

## Scope

- **In scope**:
  - `cmd/nano-brain/init.go` — add OpenCode detection + conditional prompt + `harvester` block in YAML template
  - `cmd/nano-brain/main.go` — add auto-detect call before the `cfg.Harvester.OpenCode.SessionDir != ""` check
  - `cmd/nano-brain/detect.go` (NEW, ~40 lines) — `detectOpenCodeStorageDir() string` (platform-aware, pure stdlib, no side effects)
  - Tests: `cmd/nano-brain/detect_test.go`
- **Out of scope**:
  - Claude Code harvester auto-detect (separate, lower priority)
  - Watching for OpenCode installed later (cold-start only)
  - Auto-trigger first harvest after detect (let user run `harvest` explicitly)
  - Config file hot-reload of newly detected path (server restart required)
  - Windows support for daemon (already Unix-only; detect logic is cross-platform stdlib)

## Risk Classification

- Multi-file change: 1 flag
- New behavior in server startup (auto-detect modifies effective config): 1 flag
- Interactive wizard changes (user-facing flow): 1 flag

**Total: 3 flags → lane:normal.** No DB schema change, no API change, no security-sensitive code. The fallback guarantee (if detection fails → current behavior) minimizes regression risk.

## References

- Issue #143
- Server startup: `cmd/nano-brain/main.go` lines ~212-226
- Init wizard: `cmd/nano-brain/init.go` lines 154-159 (workspace prompt — harvester prompt goes after)
- Config struct: `internal/config/config.go` `HarvesterConfig`
- Defaults: `internal/config/defaults.go` line 32 (`SessionDir: ""`)
- Env var wiring: `internal/config/config.go` `specialEnvVars["OPENCODE_STORAGE_DIR"]`
- Related: #131 (enhanced-init — explicitly omitted harvester from scope)
