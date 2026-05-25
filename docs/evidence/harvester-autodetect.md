# Evidence: harvester-autodetect (#143)

## Summary

Server silently disabled the OpenCode session harvester when `session_dir` was
empty. This change adds cross-platform auto-detection so the harvester turns
itself on when OpenCode is installed.

## Scope

- `cmd/nano-brain/detect.go` — new `detectOpenCodeStorageDir()` + `platformOpenCodePaths()`
- `cmd/nano-brain/detect_test.go` — 7 unit tests (env var, env-missing, XDG, HOME-linux, HOME-mac, none-found, env-priority)
- `cmd/nano-brain/main.go` — server startup mutates `cfg.Harvester.OpenCode.SessionDir` in memory if empty and a path is detected
- `cmd/nano-brain/init.go` — wizard prompts to enable harvesting when OpenCode is detected; conditionally appends `harvester:` block to generated `config.yml`

`internal/` is untouched. No new Go dependencies.

## Detection precedence

1. `OPENCODE_STORAGE_DIR` env var (already wired in `internal/config` specialEnvVars; pre-applied at config load)
2. Platform paths:
   - linux: `$XDG_DATA_HOME/opencode/storage`, then `$HOME/.local/share/opencode/storage`
   - darwin: `$HOME/Library/Application Support/opencode/storage`
   - windows: `%APPDATA%\opencode\storage`

`stat` confirms the path exists before returning. First match wins.

## Validation

```
$ CGO_ENABLED=0 go build ./...
$ go vet ./...
$ go test -race -short ./cmd/nano-brain/...
ok  	github.com/nano-brain/nano-brain/cmd/nano-brain	1.054s
$ go test -race -short ./...
ok across all packages (cmd/nano-brain + 14 internal packages)
```

### Detect test output (linux host)

```
--- PASS: TestDetectOpenCodeStorageDir_EnvVar (0.00s)
--- PASS: TestDetectOpenCodeStorageDir_EnvVarMissing (0.00s)
--- PASS: TestDetectOpenCodeStorageDir_XDGDataHome (0.00s)
--- PASS: TestDetectOpenCodeStorageDir_HomeLinuxFallback (0.00s)
--- SKIP: TestDetectOpenCodeStorageDir_HomeMac (0.00s) — darwin-only path
--- PASS: TestDetectOpenCodeStorageDir_NoneFound (0.00s)
--- PASS: TestDetectOpenCodeStorageDir_EnvVarPriority (0.00s)
PASS
```

The mac and windows branches are guarded by `t.Skip` when `runtime.GOOS`
doesn't match. The cross-platform behaviour of `platformOpenCodePaths()` is
exercised end-to-end on the matching host; CI on each target OS will run the
branch relevant to that OS.

## Manual verification path (interactive wizard)

The wizard prompt path is not covered by automated tests (interactive stdin).
Manual smoke (linux host with `~/.local/share/opencode/storage` present):

```
$ ./nano-brain init
... PostgreSQL/embedding/port prompts ...
  Save this config? [Y]: <enter>
Config written to /home/user/.nano-brain/config.yml
...
  Register workspace directory? [/cwd]: <enter>
  OpenCode detected at /home/user/.local/share/opencode/storage
  Enable session harvesting? [Y]: <enter>
```

Resulting config.yml ends with:

```yaml
logging:
  level: info

harvester:
  opencode:
    session_dir: /home/user/.local/share/opencode/storage
  claudecode:
    enabled: false
    session_dir: ""
```

## Out of scope

- No `internal/` changes (config schema, harvest package untouched).
- Wizard does not auto-rewrite an existing config.yml — auto-detect at server
  startup (Commit 2) handles already-installed users without config edits.
- No new dependencies.
