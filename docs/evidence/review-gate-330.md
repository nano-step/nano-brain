# Review Gate — Issue #330 / PR #333

**Date**: 2026-06-02
**PR**: #333
**Lane**: tiny | **Change-type**: bug-fix

## 5-agent parallel /review-work execution

Reviews fired in parallel via `task(subagent_type="oracle"|"unspecified-high", run_in_background=true)`. Implementing agent (Sisyphus orchestrator) did NOT self-review — all 5 reviewers were independent subagents in separate sessions.

| # | Reviewer | Session ID | Verdict |
|---|---|---|---|
| 1 | Goal Oracle | ses_177d7fcfeffe5hwtowR46cbv8d | ✅ **PASS** — All 4 acceptance criteria met |
| 2 | Code Quality Oracle | ses_177d793e5ffeKuNwUG3zbjMucT | ✅ **PASS** 92/100 — clean idiom, no blockers |
| 3 | Security Oracle | ses_177d74658ffe54wO56dDU32578 | ✅ **PASS** severity NONE-LOW |
| 4 | QA Hands-on (Sisyphus-Junior) | ses_177d6e597ffeClvLPtN3Rr73Fl | ✅ **PASS** 5/5 scenarios independently verified |
| 5 | Context Mining (Sisyphus-Junior) | bg_8d2de77c | ⚠️ Backend SSE error mid-run (non-gate failure) — manual scan performed by orchestrator below |

## Review Verdict: PASS

All 4 core review gates (Goal, Code Quality, Security, Independent QA) passed. Review #5 (Context Mining) is a follow-up surfacing task — not a gate-blocker per harness rules. Best-effort manual scan performed inline.

## Reviewer #1 (Goal Oracle) summary

Verified all 4 acceptance criteria from issue #330:
1. ✅ Container + no env vars → `host.docker.internal:3100` (Smoke Test 1)
2. ✅ Explicit `NANO_BRAIN_HOST` always wins (Smoke Test 2)
3. ✅ Host users (no container) get `localhost:3100` — no regression (unit test)
4. ✅ Detection mirrors npm postinstall logic (`/.dockerenv` + `KUBERNETES_SERVICE_HOST`)

Confirmed no `localhost:3100` literals in production Go code (only test expectations).

## Reviewer #2 (Code Quality Oracle) summary

Score: 92/100. All idioms match codebase conventions:
- `isContainerFn` mirrors `isTTYFn`/`runServeDaemonFn` exactly
- 3 new tests + 2 amended tests cover all paths
- gofmt clean, no formatting issues
- Race conditions: `t.Setenv` + `t.Cleanup` pattern matches existing tests

Nits (non-blocking):
- `"host.docker.internal"` now in 4 places — backlog suggestion for a `const dockerInternalHost = "..."`. Not in scope for bug-fix.
- Second comment line on `isContainerFn` is slightly redundant — keeping for discoverability.

## Reviewer #3 (Security Oracle) summary

Severity: **NONE-LOW**. Findings:
- `/.dockerenv` planting attack — NONE (requires root, better attack vectors exist)
- DNS poisoning of `host.docker.internal` — NONE (no incremental risk vs `localhost`)
- `KUBERNETES_SERVICE_HOST` spoofing — NONE security, LOW usability (false positive → connection refused)
- Test hook injection — NONE (standard Go test-hook pattern)
- Data exfiltration — LOW (`sendRequest()` attaches NO auth headers — CLI is unauthenticated)
- Backward compat — NONE

**Conclusion**: No security blockers. Existing override path (`NANO_BRAIN_HOST`) preserves operator control.

## Reviewer #4 (QA Hands-on) summary

Independent re-execution of all smoke:e2e scenarios + 11 unit tests by Sisyphus-Junior. Build succeeded. Live server returned 30 workspaces (slight drift from 28 in evidence — server grew during test).

| Scenario | Verdict |
|---|---|
| A: container + no env | ✅ PASS — connected to host.docker.internal:3100 |
| B: NANO_BRAIN_HOST=localhost override | ✅ PASS — got expected "cannot connect" |
| C: KUBERNETES_SERVICE_HOST=10.0.0.1 | ✅ PASS — auto-detected |
| D: 3 subcommands (`workspaces list`, `tags`, `wake-up`) | ✅ PASS — all routed to host.docker.internal |
| E: 11 unit tests | ✅ ALL PASS (1.053s) |

No discrepancies with documented evidence. The fix is verified independently.

## Reviewer #5 (Context Mining) — manual scan

Reviewer #5 failed mid-execution due to a backend SSE streaming error (`response.output_text.delta` schema mismatch — unrelated to PR content). Orchestrator performed a best-effort manual scan of the same surface:

```
$ grep -rn "localhost:3100" --include="*.go" --include="*.md" ...
```

**Findings**:
1. **`cmd/nano-brain/AGENTS.md:48`** — Pre-existing line that claimed "Container environments auto-set NANO_BRAIN_HOST=host.docker.internal". This was a FALSE claim before PR #333; now after the fix it's an ACCURATE description. The doc was aspirational and now matches behavior. No action needed.

2. **`docs/reference-readme.md:496`** — MCP remote URL example uses `localhost:3100/sse`. This is MCP CLIENT config (e.g., Claude Code's mcp-remote bridge), not the nano-brain CLI. Out of scope for #330. Could file follow-up for MCP client documentation pattern, but not blocking.

3. **`docs/prds/prd-nano-brain-greenfield-2026-05-23/prd.md`** — historical PRD references `localhost:3100` as default port. Accurate then and now. No action.

4. **`docs/rri-t/web-ui/01-prepare.md:6`** — already says "`http://localhost:3100` (host) / `http://host.docker.internal:3100` (container)". Already documents the dual-environment reality. No action.

**No NEW issues surfaced.** All references are either accurate, out of scope, or aspirational docs that #333 brings into alignment.

## Final Review Verdict: **PASS**

All blocking review gates passed. Ready to merge.
