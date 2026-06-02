# Gemini code review triage — PR #319

**PR:** [nano-step/nano-brain#319](https://github.com/nano-step/nano-brain/pull/319)
**Issue:** #316
**Reviewer:** gemini-code-assist[bot]
**Review submitted:** 2026-06-02T05:28:15Z
**Commit reviewed:** `baef491`
**Triage cycle:** 1 of 3 (≤3 push cycles per harness rule)

## Findings

### F1 (medium) — Non-idiomatic loop in integration test

**File:** `internal/server/handlers/workspace_resolve_integration_test.go:80`

**Original code:**

```go
for i := 0; i < 3; i++ {
    if _, _ = doResolve(t, resolveHandler, projectPath); false {
    }
}
```

**Reviewer feedback:** The `if` statement with constant `false` condition is non-idiomatic and confusing.

**Decision:** **Accept.** Replace with the suggested form. The `if ...; false {}` wrapper was a workaround for "call function for side-effects only, ignore return values" that I wrote when working around test-infra schema isolation issues. The cleaner form expresses intent directly.

**Fix:**

```go
for i := 0; i < 3; i++ {
    _, _ = doResolve(t, resolveHandler, projectPath)
}
```

---

### F2 (medium) — CLI does not convert relative path to absolute client-side

**File:** `cmd/nano-brain/workspaces.go:194` (inside `runWorkspacesCurrentWithIO`)

**Reviewer feedback:** If `--path` is passed as a relative path (e.g., `--path=.` or `--path=sub/dir`), the CLI sends it directly to the server. Since the server's working directory may differ from the client's (especially in container/host scenarios), the server will resolve the relative path against its own CWD — producing the wrong workspace hash.

**Decision:** **Accept.** This is a real bug. The skill explicitly documents that the CLI should resolve `$PWD` to the project — relative paths must be normalized client-side because the server has no knowledge of where the user invoked the CLI from.

**Design choice:** apply `filepath.Abs(path)` (which uses `os.Getwd()` internally when the input is relative) for the `--path` branch, mirroring the default branch's `os.Getwd()` call. This is simpler than the suggested `filepath.Join(cwd, path)` and handles edge cases like `--path=./foo/../bar` correctly.

**Fix:**

```go
path := pathFlag
if path == "" {
    cwd, err := os.Getwd()
    if err != nil {
        fmt.Fprintf(stderr, "failed to detect current directory: %v\n", err)
        return 1
    }
    path = cwd
} else if !filepath.IsAbs(path) {
    abs, err := filepath.Abs(path)
    if err != nil {
        fmt.Fprintf(stderr, "failed to resolve path %q: %v\n", path, err)
        return 1
    }
    path = abs
}
```

Plus adding `path/filepath` to imports.

---

## Items NOT raised by Gemini but worth flagging

None. The review covered the two most significant client-side correctness issues.

## Items intentionally NOT addressed (out of scope)

- Smoke:e2e via long-running server (env-blocked as documented in PR body). No change here.
- Schema isolation leak in `testutil.SetupTestDB` (pre-existing test-infra bug; tracked separately).

## Verification after fix

- `go build ./...` → expect exit 0
- `go vet ./...` → expect no warnings
- `go test -race -short ./cmd/nano-brain/... ./internal/server/handlers/...` → expect PASS
- `go test -race -tags=integration -run "TestResolveWorkspaceE2E" ./internal/server/handlers/...` → expect 3 PASS
- New CLI test added to cover relative-path normalization

## Outcome

Both findings accepted and fixed. No findings disputed. Cycle 1/3 complete.
