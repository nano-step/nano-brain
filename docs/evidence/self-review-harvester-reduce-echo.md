# Self-Review — Issue #550 (harvester reduce-echo pollution)

Change-type: bug-fix · Lane: normal · Branch: `fix/harvester-reduce-echo`
Author: kokorolx.

## Actions Taken

Root cause confirmed exactly as described in #550: the summarizer model
echoes its own reduce instructions, a first draft, and the output-format
template before the real "## Goal…## Key Learnings" summary, and
`Pipeline.runReduce`/`singleShot` stored the raw completion verbatim. Applied
the issue's suggested fix #1 (cheapest, most robust — "keep only from the
last `## Goal` heading onward"):

- **`internal/summarize/pipeline.go`** — added `extractFinalSection(raw string) string`:
  finds the LAST line-start `## Goal` heading and returns from there onward;
  if no such heading exists at all, returns the raw completion unchanged
  (never risk discarding a valid summary that used different wording — no
  defensive truncation without a named signal to key off of). Wired into
  `singleShot`'s result and both `runReduce` call sites (the leaf reduce and
  the batch/hierarchical reduce — both use the same `ReduceSystemPrompt`
  5-section contract, so both need the same post-processing). `runMap`'s
  results are untouched — the map step's `ACTIVITIES/DECISIONS/FILES/...`
  format never contains a `## Goal` heading, so the extraction would be a
  no-op there anyway, but leaving it out keeps the change scoped to exactly
  where the contract applies.

## Files Changed

- `internal/summarize/pipeline.go` — `extractFinalSection` + 3 call sites.
- `internal/summarize/pipeline_test.go` — 3 new tests:
  `TestPipeline_ReduceEchoStripped` (reproduces the issue's exact echoed text
  verbatim — reduce instruction, intermediate chunk drafts, template bleed —
  asserts it's gone and the 5 real sections survive),
  `TestPipeline_SingleShotEchoStripped` (same for the single-shot path),
  `TestPipeline_ReduceNoGoalHeading_KeptUnchanged` (no heading found →
  completion kept as-is, not blindly truncated to empty).

## Findings Summary

- No regression: all pre-existing `internal/summarize` tests (which already
  return clean `"## Goal\n..."` completions from their fakes) are unaffected
  — `extractFinalSection` is idempotent on already-clean input (the heading
  is still the last occurrence, at position 0).
- **Red-green proven**: reverted `pipeline.go` only (kept the new tests),
  confirmed `TestPipeline_ReduceEchoStripped` and
  `TestPipeline_SingleShotEchoStripped` FAIL with the exact scaffolding
  visible in the failure output; reapplied, confirmed all 3 PASS.
- **Live smoke** (not just unit-level): a real HTTP `POST /api/v1/summarize`
  call against `nanobrain_test`, backed by a local mock LLM that always
  returns the exact echoed text from the issue, persists a clean summary
  document with zero scaffolding (`docs/evidence/smoke-e2e-harvester-reduce-echo.md`).
  This proves the fix through the real harvester → pipeline → persist path,
  not just the pipeline function in isolation.
- Addresses #543 D4 (session/ticket recall pollution) at the root, per the
  issue's cross-reference — no separate fix needed there.

## Resolution Status

- In scope resolved. No critical/major issues.
- `go build ./...` clean; `go test -race -short ./...` all green.
- Live smoke (nanobrain_test/:3199): PASS, dev DB never touched.

## Gemini Verification Triage

_Pending — populate after the Gemini bot reviews the PR._

| Comment ref | Agent verdict | Reasoning | Action |
| --- | --- | --- | --- |
| _(none yet)_ | | | |
