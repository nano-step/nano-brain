## Self-Review: feat/config-env-var (PR pending, closes #219)
Date: 2026-05-30
Reviewer: Sisyphus orchestrator

## Findings
| # | Severity | File | Description | Status |
|---|----------|------|-------------|--------|
| - | none | - | Surgical refactor: single helper + sed-style replacement in 12 call sites. Backward compat preserved (flag still wins; default unchanged when neither set). | n/a |

## E2E smoke (PG @ host.docker.internal:5432)
- Test #1: `NANO_BRAIN_CONFIG=/tmp/nb-test/config.yml /tmp/nb-cfg` → server starts on port 3199 (from env-pointed config); /health returns ok with 13 workspaces.
- Test #2: `NANO_BRAIN_CONFIG=/tmp/nb-test/config.yml /tmp/nb-cfg --config /tmp/nb-test/config-flag.yml` (flag points to port 3198, env to 3199) → server starts on 3198 (flag wins); port 3199 unreachable.
- Test #3: 4 unit tests cover all precedence cases (flag wins / env when no flag / default when neither / empty env falls back).

## Unit tests
- internal/config/config_test.go: 4 new TestResolveConfigPath_* tests, all pass
- Full suite: `go test -race -short -count=1 ./...` → 20 packages OK

## Build
- `CGO_ENABLED=0 go build ./...` → exit 0

## Files modified
- internal/config/config.go: +14 LOC (ResolveConfigPath helper)
- internal/config/config_test.go: +33 LOC (4 tests)
- cmd/nano-brain/main.go: 2 callsites refactored
- cmd/nano-brain/{bench,client,cmd_cleanup_stale_raw,cmd_reset_embeddings,config_cmd,doctor,init,migrate,ops}.go: 12 sites total via ast-grep
- README.md: env var table entry + Docker example
- CHANGELOG.md: new [Unreleased] section with this + #151/#156/#214

## Summary
- Critical: 0, Major: 0, Minor: 0
- E2E precedence verified with real server start
