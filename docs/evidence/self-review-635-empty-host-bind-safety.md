# Self-Review: Issue #635 / PR (fix/635-empty-host-bind-safety)

**Change Type:** bug-fix (security)
**Story:** #635 — empty `server.host` binds every interface and bypasses the bind-safety auth requirement
**Lane:** high-risk (hard gate: authorization — the change alters a security control)
**Date:** 2026-08-26
**Reviewer (Self):** implementing agent — independent review delegated separately per R88 (no self-approval)

## Summary

`server.host: "0.0.0.0"` is correctly refused unless auth is configured. An empty host was the same bind with none of the protection: koanf applies a default only when a key is *absent*, so `host: ""` (or a bare `host:`) survived as `""`; `Server.Start` renders that as `":<port>"`, a wildcard bind; and `checkBindSafety` substituted `"localhost"` and returned nil. A security control failing open, silently, on a value a plausible typo produces.

## Actions Taken

1. **Reproduced each link independently** before writing any fix — config load leaves `""` intact (temporary probe), `fmt.Sprintf` renders `":3199"`, `checkBindSafety("")` returns nil (read from source).
2. **Fixed at the config layer** — `internal/config/config.go` resolves an empty or whitespace host to the `localhost` default after unmarshal, so the empty and omitted forms mean the same safe thing.
3. **Fixed at the control layer** — `cmd/nano-brain/bindsafety.go`: `isLoopback` no longer reports true for a host naming nothing; `checkBindSafety` no longer substitutes `localhost`. Correct for any value reaching it, not only values from `config.Load`.
4. **Corrected tests that asserted the vulnerability** — `TestIsLoopback` expected `isLoopback("") == true`, and `""` was in `TestCheckBindSafety_AllowsLoopback`'s allow-list. Any correct fix would have failed CI as a regression.
5. **Verified end to end** — a config with a bare `host:` now logs `addr="localhost:3199"` and binds `127.0.0.1` only (`lsof`).
6. **Applied three review follow-ups** in `4ddca61` — pinned the `"[]"` bracket-pair wildcard the reviewer found, trimmed the host *value* so a padded ` 127.0.0.1 ` stays usable, and rewrote the rejection message that told operators an empty value "binds to a non-loopback address".
7. **Recorded what was deliberately not run** — no server was started with an empty or non-loopback host; that binds every interface on the developer machine. Stated explicitly in the smoke evidence rather than left implicit.

## Files Changed

| File | Change |
|---|---|
| `internal/config/config.go` | Trim the host value; resolve empty/whitespace to the `localhost` default after unmarshal |
| `internal/config/config_test.go` | New: default restored for omitted / `""` / YAML-null / whitespace; explicit host preserved; padded host trimmed |
| `cmd/nano-brain/bindsafety.go` | `isLoopback("")` → false; drop the localhost substitution; distinct message for a host that names nothing |
| `cmd/nano-brain/bindsafety_test.go` | Corrected the two cases asserting the vulnerability; added `"[]"`, `"[::]"`, `"::"`, whitespace; new `TestCheckBindSafety_RejectsEmptyHost` covering both documented escapes |
| `CHANGELOG.md` | Fixed entry |
| `docs/evidence/smoke-e2e-635-bindsafety.md` | Smoke transcript |

No migration, no new dependencies, no sqlc regeneration, no adjacent refactoring.

## Findings Summary

| # | Severity | Finding | Source |
|---|---|---|---|
| 1 | **High** | Empty `server.host` → unauthenticated wildcard bind | Original issue |
| 2 | **Medium** | Existing tests encoded the vulnerable behavior as the specification, so a correct fix would fail CI | Found while fixing |
| 3 | **Low** | `host: "[]"` is a third spelling of the same wildcard — `strings.Trim` reduces it to `""`, and it is the one variant `config.Load` does not normalize | Independent reviewer |
| 4 | **Low** | `Load` tested for emptiness with `TrimSpace` but stored the untrimmed value, so ` 127.0.0.1 ` failed `ParseIP` and was refused at startup | Independent reviewer |
| 5 | **Low** | Rejection message said an empty value "binds to a non-loopback address" — self-contradictory | Independent reviewer |
| 6 | Info | `applyReloadedConfig` sets `fullCfg` but not `cfg`, so the config endpoint can report a host the process is not bound to | Independent reviewer — pre-existing, out of scope |
| 7 | Info | `serve -d` reports success when the child dies on a bind-safety rejection | Independent reviewer — pre-existing, out of scope |
| 8 | Info | `docs/SETUP_AGENT.md` references a `--host` flag that does not exist | Independent reviewer — pre-existing, out of scope |

## Resolution Status

| # | Status |
|---|---|
| 1 | **Fixed** — `56a9122`, both layers, verified end to end |
| 2 | **Fixed** — `56a9122`, both assertions corrected, dedicated test added |
| 3 | **Fixed** — `4ddca61`, pinned in the `isLoopback` table and the rejection list |
| 4 | **Fixed** — `4ddca61`, value trimmed; test added |
| 5 | **Fixed** — `4ddca61`, distinct message for a host that names nothing |
| 6-8 | **Deferred** — pre-existing, unrelated to this change, listed in the PR body for separate issues |

## Risk audit per FEATURE_INTAKE.md

| Flag | Applied? | Mitigation |
|---|---|---|
| Authorization | **Yes** | The change alters when auth is required. Verified the control still allows every legitimate loopback form and still refuses every wildcard spelling; both documented escapes (auth enabled, `--unsafe-no-auth`) confirmed working by test |
| Audit-security | **Yes** | Independent adversarial review completed all six requested attack areas; verdict PASS |
| Existing behavior | **Yes** | Nothing ships or documents an empty host — `docker-compose.yml` sets `0.0.0.0` explicitly, `template.go` writes `localhost`, `config.test.yml` is `localhost`. Not a breaking change for any supported configuration |
| Data-model / search-quality / embedding / external-provider / public-api-contract | No | n/a |

## Validation

| Step | Result |
|---|---|
| `go build ./...` | clean |
| `go vet ./...` | clean |
| `go test -race -short ./...` | 32 packages ok, 0 fail |
| `go test -race -tags=integration ./internal/config/... ./cmd/...` | both ok |
| smoke:e2e | bare `host:` → `addr="localhost:3199"`, `lsof` shows `127.0.0.1:3199` only, `/api/status` 200 |
| Independent review | **PASS** — 4 Low/Info findings, 3 addressed, 3 deferred as pre-existing |

## Scope discipline

Every changed line traces to #635 or to a review finding on it. No adjacent refactoring, no formatting cleanups. The one judgment call worth naming: fixing at *two* layers rather than one. The config fix alone would leave the control wrong for any value not arriving via `config.Load`; the bindsafety fix alone would turn a bare `host:` — a plausible typo — into a startup error demanding auth. Each addresses a different failure mode.
