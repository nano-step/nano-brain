# Gemini Triage — Issue #368 / PR #369

This file documents Gemini bot review triage for the harness async-pr-review gate (check 3.5.4). Filename matches `self-review*${slug}*.md` where slug = branch name after `/` (= `368-mcp-graph-paths`) per `scripts/check-pr-review.sh:258`.

## Triage table

| # | Reviewer | File:line | Severity | Comment summary | Classification | Action |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | gemini-code-assist | internal/mcp/graph_paths.go:10 | medium | Use `path` package instead of `path/filepath` — DB stores Unix-style paths with `/`, `filepath` uses OS-aware separator (would break on Windows) | VALID:medium | fixed in fix-up commit |
| 2 | gemini-code-assist | internal/mcp/graph_paths.go:46 | medium | Same as #1 — apply `path` to `resolveNodeAgainstWorkspace` body | VALID:medium | fixed in fix-up commit |
| 3 | gemini-code-assist | internal/mcp/graph_paths_test.go:7 | medium | Same as #1 — use `path` in test imports | VALID:medium | fixed in fix-up commit |
| 4 | gemini-code-assist | internal/mcp/graph_paths_test.go:18 | medium | Same as #1 — use `path` in `resolveNodeNoDB` | VALID:medium | fixed in fix-up commit |

## Root cause shared by all 4 comments

The implementation used `path/filepath` (OS-aware) but the database stores Unix-style paths (`/Users/tamlh/...`, forward slash). On Linux/Darwin `filepath.Join` produces `/`-separated output — works coincidentally. On Windows `filepath.Join` would produce `\`-separated output that **would NOT match** the DB's `/`-separated keys, silently reintroducing the B1 bug on Windows.

`path` is the platform-neutral package whose separator is always `/`. Switching to `path` removes the cross-platform footgun without changing observed Linux/Darwin behavior.

## Fix applied

Single targeted refactor:

- `internal/mcp/graph_paths.go`: imports `path/filepath` → `path`; calls `filepath.IsAbs/Ext/Join` → `path.IsAbs/Ext/Join`
- `internal/mcp/graph_paths_test.go`: same in test helper `resolveNodeNoDB`

No behavior change on Linux/Darwin. Tests pass:
- `go test -race -count=1 -run "TestSplitNodeSymbol|TestStripWorkspacePrefix|TestResolveNodeAgainstWorkspace" ./internal/mcp/` — ok 1.064s
- `go test -race -count=1 -tags=integration -timeout=180s ./internal/mcp/` — ok 3.135s

## Notes

- Gemini Code Assist consumer version is being sunset (cease 2026-07-17 per the review body's deprecation notice). Not a blocker.
- Independent Review Verdict for PR #369 is PASS (`docs/evidence/review-368.md`) — predates the Gemini path fix; the fix is purely additive correctness improvement.
- No `VALID:critical` or `VALID:high` findings. All 4 medium findings resolved in the fix-up commit.

## Triage classification key

- VALID:critical — Production-blocking; must fix before merge.
- VALID:high — Should fix before merge unless explicitly justified.
- VALID:medium — Should fix before merge or document deferral.
- VALID:low — Optional polish.
- INVALID:<reason> — Not a real issue.
