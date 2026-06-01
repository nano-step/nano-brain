# Gemini Triage — serve-only-mode (#282)

PR: nano-step/nano-brain#283
Date: 2026-06-01
Bot reviewer: gemini-code-assist[bot]
Agent: Sisyphus

## Triage Table (R31)

| Comment ref | Verdict | Reasoning | Action |
|-------------|---------|-----------|--------|
| daemon.go:L49 (flags not forwarded to child) | VALID:high | When `serve -d` spawns child via `os.StartProcess`, child only gets `--daemon-child --config X` args. `unsafeNoAuth` and `serveOnlyFlag` package globals in parent process don't propagate. Child runs with defaults → silent breakage. | Applied: forward both flags to childArgs in `runServeDaemon`. Also register them as `flag.BoolVar` in `main()` so child process can parse them on the `--daemon-child` invocation. |
| main.go:L400 (embed queue worker disabled but Enqueue may block) | VALID:medium | When serve_only=true, eq.Run goroutine is not started. If `eq != nil` and routes.go passes it as ChunkEnqueuer to /api/v1/write, calling `eq.Enqueue` writes to a channel with no draining worker. Channel fills to cap=10000, then `select default` branch drops chunks silently. Worse: scan worker also disabled, so dropped chunks never recover. | Applied: skip queue construction entirely when serve_only. `eq = nil` → routes.go check `s.embedQueue != nil` falls through, handler enqueuer is nil, WriteDocument's `if enqueuer != nil` guard skips embed enqueue. Documents still saved; embedding happens later when host instance scans. |

## Resolution Summary

- 2 findings addressed in 1 push cycle (R31 limit: 3)
- 0 FALSE_POSITIVE / DEFER

## Verification Post-Fix

```
$ go build ./...                    exit 0
$ go test -race -short ./cmd/nano-brain/...    PASS
```

Live test on dev server (port 3199, serve_only=true):
- Startup log: "serve_only mode — embedder constructed but queue skipped (write handler will be no-op for enqueue)"
- `/api/v1/workspaces` returns 19 workspaces
- `/health` returns 200 OK

Daemon mode flag forwarding tested logically:
- `nano-brain serve --serve-only -d` → parent sets serveOnlyFlag=true → runServeDaemon called → childArgs now includes "--serve-only" → child process flag.BoolVar parses it → child startServer() sees serveOnlyFlag=true → cfg.Server.ServeOnly = true

## Loop count
1/3 push cycles.
