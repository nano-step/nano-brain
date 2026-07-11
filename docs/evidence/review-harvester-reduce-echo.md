# Independent review — #550 (strip echoed reduce prompt/template from session summaries)

Reviewer: independent-review-agent (Opus orchestrator)

Independence (R88): the fix in commit 67db302 was written in a prior session by a different actor; this reviewer did not write that code and did not commit it, so this is a genuinely separate review pass.

Scope: `internal/summarize/pipeline.go` (`extractFinalSection` + its 3 wire-in sites) and the smoke evidence commit.

## Verdict: PASS

Review Verdict: PASS

## Findings

### Correctness — OK
- `reGoalHeading = (?im)^##\s+Goal\b`: case-insensitive, multiline, anchored to line start. `\b` correctly excludes `## Goals` (l→s is not a boundary) while allowing `## Goal:`. Compiled once at package scope, not per-call.
- `extractFinalSection` keeps from the **last** `## Goal` heading onward (`locs[len(locs)-1][0]`) — correct for the bug: echoed template/drafts carry earlier `## Goal` headings; the real summary's is last. Falls back to the raw completion when no heading exists, so a differently-worded valid summary is never truncated to empty. `TrimSpace` on the kept slice.
- Wired into all three completion paths: `singleShot` (:141), `runReduce` final (:217), and each intermediate batch result (:231) — so scaffolding cannot propagate up through recursive reduce.

### Low-severity note (not blocking)
- If a *legitimate* summary body contained a second `## Goal` heading (e.g. quoting the section name in prose), last-wins would truncate to that point. Probability is very low given the fixed 5-section ReduceSystemPrompt output format (exactly one opener heading). Accept as-is; revisit only if observed.

## Evidence checked
- `go build ./...` / `go test -race -short ./internal/summarize/...`: pass (see PR CI + executor run).
- smoke:e2e `docs/evidence/smoke-e2e-harvester-reduce-echo.md`: real HTTP run on :3199/nanobrain_test with a mock LLM that echoes reduce scaffolding; persisted summary contains only the real `## Goal` section, no "merge two chunks"/template lines. HTTP/1.1 200 responses captured.
- Cross-link: this fix is the root-cause resolution for #543 D4 (summarizer meta-sessions polluting ticket recall).
