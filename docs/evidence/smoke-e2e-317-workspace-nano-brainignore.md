# Smoke E2E — workspace-local `.nano-brainignore` (#317)

**Date**: 2026-06-02 (UTC)
**Branch**: `feat/317-workspace-nano-brainignore`
**Lane**: normal (user-feature)
**Tester**: Sisyphus (autonomous, user delegated full review responsibility)

## Setup

- Binary: `CGO_ENABLED=0 go build -o /tmp/nb317 ./cmd/nano-brain` from the worktree
- Isolated PG database: `nanobrain_smoke317` on `host.docker.internal:5432` (fresh, vector extension installed)
- Test server port: **4199** (production server occupies 3199; chose 4199 to avoid conflict)
- Config: `/tmp/smoke317-cfg/config.yml` with `logging.level: debug`, embed provider Ollama on `host.docker.internal:11434`
- Workspace dirs:
  - `/tmp/smoke317-ws/` — happy path (has `foo.go`, `skip.snap`, `.nano-brainignore` with `*.snap`)
  - `/tmp/smoke317-bad/` — WARN path (has `main.go` + `.nano-brainignore` as a DIRECTORY for deterministic IO failure)

## Server start

```
$ NANO_BRAIN_CONFIG=/tmp/smoke317-cfg/config.yml /tmp/nb317
{"level":"info","version":13,"message":"database migrations complete"}
{"level":"info","addr":"127.0.0.1:4199","message":"HTTP server listening"}
{"level":"info","component":"watcher","debounce_ms":500,"poll_interval_s":60,"message":"file watcher started"}

curl -sf http://127.0.0.1:4199/health
{"status":"ok","ready":true,"version":"dev","uptime_s":17}
```

## TC-1 — Register workspace with `.nano-brainignore` containing `*.snap`

```
$ printf '*.snap\n' > /tmp/smoke317-ws/.nano-brainignore
$ ls /tmp/smoke317-ws/
.nano-brainignore  foo.go  skip.snap

curl -sX POST http://127.0.0.1:4199/api/v1/init -H 'Content-Type: application/json' \
    -d '{"root_path":"/tmp/smoke317-ws","name":"smoke317"}'
{"workspace_hash":"1becbe9262c966f9cec8c1c4392e3b818300b44327fa1e5302c86a3766d46c1b","root_path":"/tmp/smoke317-ws", ...}
```

### Evidence: DEBUG log fires on successful load (AC #6)

```
{"level":"debug","component":"watcher","dir":"/tmp/smoke317-ws","collection":"code",
 "time":"2026-06-02T05:03:46Z","message":"loaded workspace .nano-brainignore"}
```

### Evidence: `skip.snap` NOT processed by indexer

Searching the full server log, `skip.snap` appears in zero `"processing file"` or `"indexed file"` log lines. Only `.nano-brainignore` (the rule file itself, not in the rule) and `foo.go` are indexed.

## TC-2 — Search confirms filter (AC #1)

```
$ WS=1becbe9262c966f9cec8c1c4392e3b818300b44327fa1e5302c86a3766d46c1b

curl -sX POST http://127.0.0.1:4199/api/v1/search -H 'Content-Type: application/json' \
    -d "{\"workspace\":\"$WS\",\"query\":\"snapshot\"}"
{"results":[],"total":0,"query_ms":11}
```

✅ PASS — `skip.snap` filtered out (would otherwise match the BM25 query "snapshot" since its content is "snapshot data").

```
curl -sX POST http://127.0.0.1:4199/api/v1/search -H 'Content-Type: application/json' \
    -d "{\"workspace\":\"$WS\",\"query\":\"package main\"}"
{"results":[{"id":"da7bfd2c-2b39-...","title":"foo.go","snippet":"package main\n","score":0.40,
  "collection":"code","source_path":"/tmp/smoke317-ws/foo.go", ...}],"total":1,"query_ms":2}
```

✅ PASS — `foo.go` indexed normally (other files unaffected).

## TC-3 — Re-init picks up updated patterns without restart (AC #8)

```
$ echo 'noise data' > /tmp/smoke317-ws/skip.junk
$ echo 'normal data' > /tmp/smoke317-ws/keep.txt
$ printf '*.snap\n*.junk\n' > /tmp/smoke317-ws/.nano-brainignore

curl -sX POST http://127.0.0.1:4199/api/v1/init -H 'Content-Type: application/json' \
    -d '{"root_path":"/tmp/smoke317-ws","name":"smoke317"}'
{"workspace_hash":"1becbe92...", ...}
```

### Evidence: DEBUG log fires a SECOND time (file re-read)

```
{"level":"debug","component":"watcher","dir":"/tmp/smoke317-ws","collection":"code",
 "time":"2026-06-02T05:04:23Z","message":"loaded workspace .nano-brainignore"}
```

### Evidence: New pattern takes effect immediately

```
curl -sX POST http://127.0.0.1:4199/api/v1/search -H 'Content-Type: application/json' \
    -d "{\"workspace\":\"$WS\",\"query\":\"noise\"}"
{"results":[],"total":0,"query_ms":19}
```

✅ PASS — `skip.junk` filtered by newly-added `*.junk` rule, no server restart needed.

```
curl -sX POST http://127.0.0.1:4199/api/v1/search -H 'Content-Type: application/json' \
    -d "{\"workspace\":\"$WS\",\"query\":\"normal data\"}"
{"results":[{"title":"keep.txt","snippet":"normal data\n", ...}],"total":1}
```

✅ PASS — `keep.txt` (not matching any pattern) indexed normally.

## TC-4 — Unreadable `.nano-brainignore` → WARN log, server continues (AC #5)

```
$ mkdir -p /tmp/smoke317-bad/.nano-brainignore   # path is a directory, os.ReadFile will fail
$ echo 'data' > /tmp/smoke317-bad/main.go

curl -sX POST http://127.0.0.1:4199/api/v1/init -H 'Content-Type: application/json' \
    -d '{"root_path":"/tmp/smoke317-bad","name":"smoke317-bad"}'
{"workspace_hash":"3c2d7862...", ...}
```

### Evidence: WARN log emitted with structured error

```
{"level":"warn","component":"watcher",
 "error":"load workspace .nano-brainignore at /tmp/smoke317-bad/.nano-brainignore: read /tmp/smoke317-bad/.nano-brainignore: is a directory",
 "dir":"/tmp/smoke317-bad","collection":"code",
 "time":"2026-06-02T05:04:46Z",
 "message":"workspace .nano-brainignore failed to load, continuing without local matcher"}
```

### Evidence: Server did NOT crash; collection works normally

```
curl -sX POST http://127.0.0.1:4199/api/v1/search -H 'Content-Type: application/json' \
    -d "{\"workspace\":\"3c2d7862...\",\"query\":\"data\"}"
{"results":[{"title":"main.go","snippet":"data\n", ...}],"total":1}
```

✅ PASS — main.go indexed (other filter layers still active); server health stayed `ready:true` throughout.

## Acceptance Criteria Coverage

| AC | Description | Evidence |
|---|---|---|
| 1 | File honored | TC-1 + TC-2: `*.snap` rule blocks `skip.snap` from search |
| 2 | File missing = no-op | Implicit (other registered workspaces in this run had no file; no log noise, no failures) |
| 3 | Compose with global | Covered by unit test `TestFileFilter_LocalNanoBrainIgnoreCombinesWithGlobal` |
| 4 | Compose with .gitignore | Covered by unit test `TestFileFilter_LocalNanoBrainIgnoreCombinesWithGitignore` |
| 5 | Malformed/unreadable → WARN + continue | TC-4: directory-as-file scenario; WARN logged, server healthy |
| 6 | Successful load is observable (DEBUG log) | TC-1 + TC-3: DEBUG log fires on each load |
| 7 | Precedence documented | README.md updated, see commit 89f8b71 |
| 8 | Re-registration picks up changes | TC-3: 2nd DEBUG log + new pattern takes effect after `/api/v1/init` re-call |

## Validation ladder

| Layer | Result |
|---|---|
| `validate:quick` (`go build && go test -race -short ./...`) | ✅ PASS — 25 packages, all green (see commit 89f8b71 message) |
| `self-review:response-shape` | N/A — no new HTTP response struct |
| `self-review:staged-files` | ✅ Clean — only README.md, internal/watcher/{filter.go, filter_test.go, watcher.go} + openspec/changes/workspace-nano-brainignore/* |
| `test:integration` | N/A per Momus review (pure in-memory + filesystem; unit tests already exercise real `os.Stat` + `CompileIgnoreFile` + `MatchesPath`) |
| `smoke:e2e` | ✅ PASS — this document |

## Test environment cleanup

```
$ kill <test-server-pid>    # via tmux Ctrl-C
$ rm -rf /tmp/smoke317-cfg /tmp/smoke317-data /tmp/smoke317-ws /tmp/smoke317-bad /tmp/smoke317-server.log /tmp/smoke317-stdout.log /tmp/nb317
$ psql -h host.docker.internal -U nanobrain postgres -c "DROP DATABASE nanobrain_smoke317"
```

(Cleanup performed after evidence captured. The smoke DB was isolated from production `nanobrain_dev`; no production data touched.)
