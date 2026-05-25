# Design: Harvester Auto-Detect session_dir

## Architecture

```
cmd/nano-brain/detect.go (NEW)
  └─ detectOpenCodeStorageDir() string
       ├─ Check OPENCODE_STORAGE_DIR env
       ├─ runtime.GOOS == "linux"   → $XDG_DATA_HOME/opencode/storage
       │                              $HOME/.local/share/opencode/storage
       ├─ runtime.GOOS == "darwin"  → $HOME/Library/Application Support/opencode/storage
       └─ runtime.GOOS == "windows" → %APPDATA%\opencode\storage
       (returns first path where os.Stat succeeds, or "" if none found)

cmd/nano-brain/main.go (MODIFIED)
  └─ Before existing `cfg.Harvester.OpenCode.SessionDir != ""` check:
       if cfg.Harvester.OpenCode.SessionDir == "" {
           if detected := detectOpenCodeStorageDir(); detected != "" {
               cfg.Harvester.OpenCode.SessionDir = detected
               logger.Info().Str("path", detected).Msg("auto-detected opencode storage dir")
           }
       }

cmd/nano-brain/init.go (MODIFIED)
  └─ After workspace-registration prompt (line 159):
       detected := detectOpenCodeStorageDir()
       if detected != "" {
           fmt.Printf("  OpenCode detected at %s\n", detected)
           answer := promptWithDefault(scanner, "Enable session harvesting?", "Y")
           if answer != "n" && answer != "N" {
               sessionDir = detected
           }
       }
     Then include harvester block in YAML template if sessionDir != ""
```

## Key Decisions

### 1. Pure detection function with no side effects

`detectOpenCodeStorageDir()` only reads env vars and calls `os.Stat`. It does not write files, set env vars, or modify config. This makes it safe to call from both server startup AND the interactive wizard without risk of double-effect.

### 2. Candidate order

```
1. OPENCODE_STORAGE_DIR env var (explicit user override — highest priority)
2. Platform well-known path (auto-detect)
```

Return first candidate where `os.Stat(path) == nil` (exists). Don't require subdirectory structure (`session/`, `message/`, `part/`) for detection — just existence of the storage dir itself. Rationale: OpenCode may create subdirs lazily; the important thing is the dir exists.

### 3. In-memory config mutation (startup only)

The auto-detect in `main.go` modifies the in-memory `cfg` struct, NOT the config file. The user's `~/.nano-brain/config.yml` remains unchanged. This is intentional:
- Avoids surprising config file rewrites on every server start.
- Makes the effective session_dir visible in logs (the info log serves as the audit trail).
- If the user wants to persist the detected path, they can add it to config.yml (or use `OPENCODE_STORAGE_DIR`).

Alternative considered: write back to config file on first detection. Rejected because (a) auto-writes to user config files are surprising, (b) requires file locking, (c) on containers the config path may be read-only.

### 4. Init wizard: only prompt if detected, skip if not

```
OpenCode found  → show path + prompt Y/n
OpenCode absent → skip prompt (no nag)
```

Rationale: asking users for a path they don't have is confusing. The wizard already requires knowledge of PostgreSQL — adding an optional prompt for OpenCode when it IS present is low noise.

### 5. YAML template: harvester block only when accepted

Current template omits `harvester:` entirely. If the user accepts the prompt (or if sessionDir is pre-set from existing config), the template gains:

```yaml
harvester:
  opencode:
    session_dir: <detected path>
  claudecode:
    enabled: false
    session_dir: ""
```

If the user declines or nothing is detected, the template remains as-is (no empty `harvester:` block). Rationale: an empty `harvester:` section with `session_dir: ""` is functionally identical to no section — koanf will use the struct zero values — but it clutters the config file with unhelpful boilerplate.

### 6. Windows detection uses stdlib only

`os.Getenv("APPDATA")` is stdlib. No `golang.org/x/sys/windows` needed. `runtime.GOOS` is a compile-time constant — the Windows branch dead-strips on Linux/macOS builds.

## Files Changed

| File | Change |
|---|---|
| `cmd/nano-brain/detect.go` | NEW — `detectOpenCodeStorageDir() string` (~40 lines) |
| `cmd/nano-brain/detect_test.go` | NEW — table tests with `t.Setenv` for env vars, `t.TempDir()` for fake paths |
| `cmd/nano-brain/main.go` | Add auto-detect block before harvester `if` check (~6 lines) |
| `cmd/nano-brain/init.go` | Add OpenCode prompt after workspace prompt; conditional `harvester:` block in YAML template |

## Test Plan

### `detect_test.go`

1. `TestDetectOpenCodeStorageDir_EnvVar` — `OPENCODE_STORAGE_DIR=/tmp/x` (exists via `t.TempDir`) → returns `/tmp/x`
2. `TestDetectOpenCodeStorageDir_EnvVarMissing` — env var set but path does not exist → skips env var, falls through to platform paths
3. `TestDetectOpenCodeStorageDir_XDGDataHome` — `XDG_DATA_HOME=/tmp/x`, subdir `opencode/storage` created → returns path (Linux only via GOOS check, or just test the env var logic directly)
4. `TestDetectOpenCodeStorageDir_HomeLinux` — `HOME=/tmp/x`, `.local/share/opencode/storage` created → returns path
5. `TestDetectOpenCodeStorageDir_HomeMac` — `HOME=/tmp/x`, `Library/Application Support/opencode/storage` created → returns path
6. `TestDetectOpenCodeStorageDir_NoneFound` — all env vars unset, none of the paths exist → returns `""`
7. `TestDetectOpenCodeStorageDir_EnvVarPriority` — both env var (exists) and platform path (exists) → returns env var path

### `init.go` changes

No unit tests added for the interactive wizard (it's tested manually — same approach as the existing `runInteractiveInit` which has no unit tests). The behavioral spec scenarios cover the intent.

### `main.go` changes

No isolated unit test (server startup is integration-tested via existing `go test ./...`). The auto-detect logic is thin — it calls `detectOpenCodeStorageDir()` (well-tested) and sets `cfg.Harvester.OpenCode.SessionDir`. Covered by detect_test.go indirectly.

## Smoke Test (manual)

```bash
# 1. With OpenCode installed:
nano-brain serve -d
# Expect in logs:
# {"level":"info","path":"/Users/me/.local/share/opencode/storage","message":"auto-detected opencode storage dir"}
# {"level":"info","session_dir":"/Users/me/.local/share/opencode/storage","message":"opencode session harvester started"}

# 2. Without OpenCode:
# (OpenCode storage dir doesn't exist)
# Expect: {"level":"info","message":"opencode session harvester disabled (no session_dir configured)"}
# (unchanged behavior)

# 3. Init wizard with OpenCode present:
nano-brain init
# Expect after workspace prompt:
#   OpenCode detected at /Users/me/.local/share/opencode/storage
#   Enable session harvesting? [Y]:
# On Y: config.yml gains harvester.opencode.session_dir
```
