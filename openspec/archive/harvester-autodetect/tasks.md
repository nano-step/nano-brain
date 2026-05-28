# Tasks: Harvester Auto-Detect session_dir

## Phase 1 — detectOpenCodeStorageDir function

- [x] **1.1** Create `cmd/nano-brain/detect.go` with `detectOpenCodeStorageDir() string`:
  ```go
  func detectOpenCodeStorageDir() string {
      // 1. OPENCODE_STORAGE_DIR env var
      if v := os.Getenv("OPENCODE_STORAGE_DIR"); v != "" {
          if _, err := os.Stat(v); err == nil { return v }
      }
      // 2. Platform paths
      candidates := platformOpenCodePaths()
      for _, p := range candidates {
          if _, err := os.Stat(p); err == nil { return p }
      }
      return ""
  }
  
  func platformOpenCodePaths() []string {
      // runtime.GOOS switch
      // linux:   XDG_DATA_HOME/opencode/storage, HOME/.local/share/opencode/storage
      // darwin:  HOME/Library/Application Support/opencode/storage
      // windows: APPDATA\opencode\storage
      // default: []
  }
  ```
  Use only stdlib: `os`, `path/filepath`, `runtime`.

- [x] **1.2** Create `cmd/nano-brain/detect_test.go` with 7 table-driven tests:
  - `TestDetectOpenCodeStorageDir_EnvVar` — `t.Setenv("OPENCODE_STORAGE_DIR", tmpDir)` → returns tmpDir
  - `TestDetectOpenCodeStorageDir_EnvVarMissing` — env set but path absent → skips, tries platform
  - `TestDetectOpenCodeStorageDir_XDGDataHome` — `t.Setenv("XDG_DATA_HOME", tmpDir)` + create `opencode/storage` inside → returns path (test platform-path logic directly, not just on Linux GOOS)
  - `TestDetectOpenCodeStorageDir_HomeLinuxFallback` — `t.Setenv("HOME", tmpDir)` + create `.local/share/opencode/storage` → returns path
  - `TestDetectOpenCodeStorageDir_HomeMac` — `t.Setenv("HOME", tmpDir)` + create `Library/Application Support/opencode/storage` → returns path
  - `TestDetectOpenCodeStorageDir_NoneFound` — all env vars unset or pointing to nonexistent paths → `""`
  - `TestDetectOpenCodeStorageDir_EnvVarPriority` — both env var path and platform path exist → returns env var path
  
  Note: tests exercise `platformOpenCodePaths()` by setting HOME/XDG and creating dirs — cross-platform logic is testable even on a single OS because we control the env vars.

## Phase 2 — Server startup auto-detect

- [x] **2.1** In `cmd/nano-brain/main.go`, find the harvester `if` block (around line 212). Add before it:
  ```go
  if cfg.Harvester.OpenCode.SessionDir == "" {
      if detected := detectOpenCodeStorageDir(); detected != "" {
          cfg.Harvester.OpenCode.SessionDir = detected
          logger.Info().Str("path", detected).Msg("auto-detected opencode storage dir")
      }
  }
  ```
  This must be placed AFTER `cfg` is fully loaded (after config load + env overrides).

## Phase 3 — Init wizard harvester prompt

- [x] **3.1** In `cmd/nano-brain/init.go`, after the workspace-registration prompt block (after line 159), add:
  ```go
  var sessionDir string
  if detected := detectOpenCodeStorageDir(); detected != "" {
      fmt.Printf("  OpenCode detected at %s\n", detected)
      answer := promptWithDefault(scanner, "Enable session harvesting?", "Y")
      if answer != "n" && answer != "N" {
          sessionDir = detected
      }
  }
  ```
- [x] **3.2** In the same file, update the YAML template string to conditionally include `harvester:` block:
  ```go
  var harvesterBlock string
  if sessionDir != "" {
      harvesterBlock = fmt.Sprintf("harvester:\n  opencode:\n    session_dir: %s\n  claudecode:\n    enabled: false\n    session_dir: \"\"\n", sessionDir)
  }
  yaml := fmt.Sprintf(`...existing template...
  %s`, ..., harvesterBlock)
  ```
  The `harvesterBlock` is appended after the `logging:` section (or after `watcher:`) in the template. When empty, no `harvester:` section appears.

## Phase 4 — Validation ladder

- [x] **4.1** `CGO_ENABLED=0 go build ./...` → success
- [x] **4.2** `go vet ./cmd/nano-brain/...` → clean
- [x] **4.3** `go test -race -short ./cmd/nano-brain/...` → all pass (including 7 new detect tests)
- [x] **4.4** `go test -race -short ./...` → all packages pass

## Phase 5 — Evidence + tasks complete

- [x] **5.1** Write `docs/evidence/harvester-autodetect.md` with smoke transcript or note.
- [x] **5.2** Mark all `[ ]` → `[x]` in this file.

## Phase 6 — PR (orchestrator)

- [ ] **6.1** Push branch, open PR linking issue #143.
