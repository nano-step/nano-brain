# Gemini Review Triage — PR #311

## Cycle 1 — both MEDIUM findings accepted

### Finding 1 MEDIUM — unused strings import after refactor
**Verdict: ACCEPTED**

Gemini suggested removing strings.Contains in favor of map-key lookups → strings import would become unused → Go compile error.

**Fix:** Removed `strings` from imports as part of refactor below.

### Finding 2 MEDIUM — fragile string-matching in test assertions
**Verdict: ACCEPTED**

`strings.Contains(json, "\"Title\"")` produces false positives if a field VALUE contains the string `"Title"`. Better: unmarshal JSON to map[string]any, check map keys directly.

**Fix:**
- Unmarshal JSON output to map[string]any
- Check key presence via map lookup (`if _, ok := m[k]; !ok`)
- Expanded forbidden-key list to all 12 PascalCase variants
- Added mapKeys helper for diagnostic output

## Cycle 1 verification

- go test PASS for new behavior
- go vet clean
- Full suite still green

Both findings real, accepted, ~25 LOC refactor of the test only (production code unchanged).
