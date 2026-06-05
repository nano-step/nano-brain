# Self-Review: Issue #235 - CLI JSON stdout pollution

**Issue**: #235
**Branch**: fix/235-cli-json-stdout-pollution
**Date**: 2026-06-05
**Reviewer**: Sisyphus (self)

## Actions Taken

1. Analyzed issue #235 - CLI commands with `--json` flag mix zerolog INFO logs with JSON response on stdout, breaking jq pipelines
2. Identified root cause: `initCLILog()` used `health.NewLogger()` which creates a multi-writer to both stdout and log file
3. Replaced `health.NewLogger()` with direct `zerolog.New(os.Stderr)` to route logs exclusively to stderr
4. Added `strings` import required by the new implementation
5. Verified fix with stream isolation tests (stdout-only and stderr-only)
6. Confirmed jq pipeline compatibility

## Files Changed

### cmd/nano-brain/main.go

**Change 1**: Added `strings` import (required by `strings.ToLower` and `strings.TrimSpace`)

**Change 2**: Rewrote `initCLILog()` function
- **Before**: Called `health.NewLogger(cfg.Logging)` which writes to stdout + file
- **After**: Direct `zerolog.New(os.Stderr)` with inline log level parsing
- **Lines changed**: ~20 lines (removed 5, added 24)

## Findings Summary

### Critical Issues
- None

### Major Issues
- None

### Minor Issues
- None

### Observations
1. The fix is backward-compatible - no API changes, only internal logging behavior
2. All CLI commands (`get`, `tags`, `multi-get`, `query`, `search`, `vsearch`) automatically benefit from the fix
3. Verification confirmed clean separation: stdout = JSON/data, stderr = logs

## Resolution Status

✅ **All findings resolved**

- Build passes: `go build ./...`
- Tests pass: `go test -race -short ./...`
- Integration tests pass: `go test -race -tags=integration ./...`
- Manual verification: Confirmed logs route to stderr, JSON to stdout
- jq pipeline test: `./nano-brain tags --json 2>/dev/null | jq` - parses successfully

## Verification Evidence

See `docs/evidence/235/fix-verification.md` for detailed test results.

## Commit

- f0cdfbb1c13a5cebdbaf92a64008dccd8cefe039
