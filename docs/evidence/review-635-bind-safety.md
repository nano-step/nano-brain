# Independent Review: #635 — empty `server.host` bind-safety hole

**Branch:** `fix/635-empty-host-bind-safety`
**Commits reviewed:** `56a9122`, then `4ddca61` (follow-ups)
**Reviewer:** independent code-reviewer agent, separate context from the implementing agent (R88 — no self-approval)
**Date:** 2026-08-26

## Scope

Six attack areas were requested and all six were completed; none omitted. The reviewer probed Go's address semantics empirically (`net.ParseIP` / `net.SplitHostPort` in isolation) rather than reasoning from memory. **No server was started with an empty or non-loopback host** — that binds every interface on the developer machine; the implementing agent declined it and the reviewer made the same call independently.

## Findings

| # | Severity | Finding | Status |
|---|---|---|---|
| 1 | Low | `host: "[]"` is a third spelling of the same wildcard. `strings.Trim("[]", "[]")` yields `""`, so pre-fix `isLoopback` returned **true** → allowed unauthenticated, while `net.SplitHostPort("[]:3199")` returns host `""` with **no error** → a bind on every interface. The `isLoopback` change already closed it, but it is the only variant `config.Load` does not normalize, so this check is its sole defense — and it had no test. | **Fixed** in `4ddca61` |
| 2 | Low | `Load` tested for emptiness with `TrimSpace` but stored the untrimmed value, so a padded ` 127.0.0.1 ` survived, failed `ParseIP`, and was refused at startup as non-loopback. | **Fixed** in `4ddca61` |
| 3 | Low | The rejection message rendered `server.host="" binds to a non-loopback address` — self-contradictory beside an empty value. Reachable via `"[]"`. | **Fixed** in `4ddca61` |
| 4 | Low | The substitution is silent: an operator who wrote `host: ""` intending a wildcard now gets `localhost` with no runtime output. `config.Load` runs before the logger exists, so this would need `fmt.Fprintln(os.Stderr, ...)`. | Not addressed — CHANGELOG covers it; judged not worth a pre-logger stderr write |
| 5 | Info | `applyReloadedConfig` sets `s.fullCfg` but not `s.cfg`, so after PATCHing `server.host` the config endpoint reports a host the process is not bound to. Not a bind-safety hole (the PATCH response says `requires_restart`) but can misreport exposure. | Pre-existing, out of scope |
| 6 | Info | `serve -d` writes the PID file and prints "started" without checking the child survived, so a bind-safety rejection reads as success. Fails closed, but weakens visibility. | Pre-existing, out of scope |
| 7 | Info | `docs/SETUP_AGENT.md` references a `--host=0.0.0.0` flag that does not exist. | Pre-existing, out of scope |

## Verified by the reviewer

**Wildcard coverage.** Every spelling probed; no value found where `isLoopback` reports true but the resulting bind is not loopback:

```
""  "   "  "0.0.0.0"  "::"  "[::]"  "[]"  "0000:...:0000"   -> false, rejected
"::ffff:127.0.0.1"  "LOCALHOST"  "127.0.0.5"                -> true, allowed (genuinely loopback)
"localhost."                                                -> false, rejected (fail-closed)
```

**Every path that can set `Server.Host` passes through `config.Load`** — YAML file, `NANO_BRAIN_SERVER_HOST` (env loop precedes unmarshal), `POST /api/v1/config` PATCH (writes YAML → reloads via `Load`), `POST /api/reload-config` (`config.Reload` → `config.Load`), direct struct construction (defaults set `localhost`; one listener in the tree, one production caller). No `--host` CLI flag exists. **No path reaches a wildcard bind without normalization.**

**`checkBindSafety` is startup-only, and that is structurally guaranteed** — it is unexported in `package main`, and nothing under `internal/` imports `cmd/nano-brain`. Not a gap: `applyReloadedConfig` never touches `s.cfg` or the listener, so a host change cannot rebind a running process, and `reload.go` already classifies `server.host` as `requires_restart`.

**The auth escape is not hollow.** `middleware/auth.go` 401s any request with no `Authorization` header, and `validateBearer` short-circuits to false on an empty token list. Default `BypassPaths` is only `/health` and `/api/openapi.json`. The CSRF middleware's separate `isLoopback` does not contain `""` either, so an empty host never got a CSRF origin-match bypass.

**Not a breaking change.** Nothing ships or documents an empty host: `docker-compose.yml` sets `0.0.0.0` explicitly, `internal/config/template.go` writes `localhost`, `config.test.yml` is `localhost`, and the docs prescribe `0.0.0.0` for remote access.

## Reviewer's test evidence

```
go build ./...   clean
go test -race -short ./cmd/nano-brain/... ./internal/config/... ./internal/server/middleware/...
ok  	github.com/nano-brain/nano-brain/cmd/nano-brain	7.080s
ok  	github.com/nano-brain/nano-brain/internal/config
ok  	github.com/nano-brain/nano-brain/internal/server/middleware
```

No destructive operation, no server started, nothing killed, `nanobrain-pg` untouched, nothing run against `nanobrain_dev` or `:3100`.

## Verdict

The fix is complete and correct for the vulnerability. No hole, no regression. All findings are Low/Info polish; the three actionable ones were addressed in `4ddca61`.

Review Verdict: PASS
