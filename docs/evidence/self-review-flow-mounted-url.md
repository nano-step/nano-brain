# Self-Review — Issue #563 (#542 F1: memory_flow full-URL resolution)

Change-type: bug-fix · Lane: tiny · Branch: `fix/flow-mounted-url`
Author: kokorolx.

## Actions Taken

- `memory_flow` now resolves the full mounted URL an agent supplies to the
  stored router-local HTTP key. Added `resolveFlowEntry` (`internal/mcp/tools.go`):
  on an exact-match miss, an HTTP-shaped entry (`METHOD /path`) resolves to the
  stored HTTP key whose path is a trailing, `/`-aligned suffix of the requested
  path; the most specific (longest) wins. `registerMemoryFlow` builds the flow
  from the resolved key and surfaces `requested_entry` + `resolved_via` +
  `candidates` (when >1).
- Read-path only — no stored-key change, no re-indexing, no schema change.
- Deferred (noted in #563): composing `router.use`/`mount` prefixes into stored
  keys at extraction time (correct data) needs cross-file mount resolution +
  re-index across Express/NestJS/Rails — overlaps #542 F2, tracked separately.

## Files Changed

- `internal/mcp/tools.go` — `resolveFlowEntry` + wire into `registerMemoryFlow`
  + response fields (+73/−2).
- `internal/mcp/flow_entry_resolve_test.go` — white-box unit test (exact,
  full-URL, most-specific, method-mismatch, boundary/no-mid-segment, candidates).
- `internal/mcp/flow_url_563_integration_test.go` — e2e through the memory_flow
  handler (flow enabled): full URL → found:true + resolved_via + handler in flow.

## Findings Summary

- Boundary safety: because a stored path starts with `/`, `strings.HasSuffix`
  alone guarantees segment alignment — `/payment-intent` is NOT a suffix of
  `/prefix-payment-intent` (unit-tested). No mid-segment false match.
- **Red-green proven** at both layers: with the suffix fallback disabled, the
  integration test returns `found:false` (exact #542 F1 symptom); with it,
  `found:true` resolving to the router-local key.
- No regression: exact router-local entries still match exactly (no
  `resolved_via`); symbol entries unchanged (handled by `mcpHasFlowEntry` first).

## Resolution Status

- In scope resolved. No critical/major issues.
- `CGO_ENABLED=0 go build ./...` clean. `go test -race -short ./...` all ok
  (incl. the new white-box unit test).
- Integration (nanobrain_test): unit + e2e handler test PASS.
- smoke:e2e: `docs/evidence/smoke-e2e-flow-mounted-url.md` (MCP-over-HTTP on
  :3199, full URL → found:true, resolved_via:suffix-match). Dev DB never touched.

## Gemini Verification Triage

Gemini: COMMENTED (summary only), CI pass, MERGEABLE/CLEAN. **No inline
comments** — nothing actionable. No changes required.

| Comment ref | Agent verdict | Reasoning | Action |
| --- | --- | --- | --- |
| _(no inline comments)_ | — | Gemini left a summary review with 0 inline findings. | None. |
