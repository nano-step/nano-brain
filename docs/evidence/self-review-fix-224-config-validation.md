## Self-Review: fix/224-config-env-validation
Date: 2026-05-30
Reviewer: Sisyphus orchestrator

## E2E
- `NANO_BRAIN_CONFIG=/tmp/does-not-exist.yml ./nano-brain` → stderr line:
  `WARNING: NANO_BRAIN_CONFIG env points to "/tmp/does-not-exist.yml" but that file does not exist — defaults will be used.`
  Server continues on defaults (port 3100).
- `NANO_BRAIN_CONFIG=" /tmp/nb-test/config.yml "` (whitespace) → server starts on port 3199 from that config (whitespace trimmed before stat check).
- `--config /missing.yml` → same warning prefixed with `--config flag` instead of `NANO_BRAIN_CONFIG env`.

## Unit tests (5 new in internal/config/config_test.go)
- TestResolveConfigPath_TrimsWhitespace
- TestResolveConfigPathStrict_WarnsOnMissingFile (env)
- TestResolveConfigPathStrict_WarnsOnFlagMissingFile (flag)
- TestResolveConfigPathStrict_NoWarnWhenDefault
- TestResolveConfigPathStrict_NoWarnWhenFlagPathExists

## Design notes
- Backward-compat: existing `ResolveConfigPath(flagValue)` kept; new `ResolveConfigPathStrict` returns `(path, warning)`.
- Warning is non-fatal — server still starts. This was the explicit design choice in the issue ("Log a WARNING on stderr ... OR exit non-zero"). WARNING was chosen because in production, falling back to defaults is often what the user wants when the explicit config is mid-rollout.
- Trim happens BEFORE stat check so " /path/file.yml " resolves correctly.

## Build + tests
- `CGO_ENABLED=0 go build ./...` → exit 0
- `go test -race -short ./...` → 20 packages OK

## Summary
- Critical: 0, Major: 0, Minor: 0
- Closes #224
