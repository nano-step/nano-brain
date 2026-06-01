# Gemini Review Triage — PR #301

## Cycle 1

### Finding 1 — MEDIUM — `chunk.go:36` (units doc + constant export)
**Verdict: ACCEPTED**

Gemini caught two real issues in one comment:

**1a. Doc says "chars" but code does bytes.** Earlier explore audit (during #297 work) flagged this same drift. Fixed in this PR by updating all three `Config` field doc strings from `in chars` → `in bytes` and adding an explicit units note on the package-level constant.

**1b. Eliminate the magic number 2600.** Excellent suggestion. Exporting `DefaultMaxChunkBytes = 3000` and computing `TargetSize = DefaultMaxChunkBytes - searchWindow/2` makes the contract enforced by construction. A future maintainer cannot break it without also touching the exported constant — much harder than "did I update both files?" sloppiness that caused #300 to recur after #297.

**Implementation:**
- Added `const DefaultMaxChunkBytes = 3000` at package scope with explicit units docstring
- Rewrote `DefaultConfig()` to compute `TargetSize: DefaultMaxChunkBytes - searchWindow/2`
- Updated `Config` field doc strings to say "in bytes"
- Updated `Config` invariant docstring to reference the exported constant

### Finding 2 — MEDIUM — `chunk_test.go:447` (use exported constant in tests)
**Verdict: ACCEPTED**

Tests had `const defaultMaxEmbedChars = 3000` and `const embedMaxChars = 3000` — magic numbers that would silently drift if the exported constant moved. Replaced with `DefaultMaxChunkBytes` references.

## Cycle 1 verification

- `go build ./...` exit 0
- `go vet ./...` clean
- `go test -race -short ./internal/chunk/...` PASS (15 chunker tests + fuzz seeds)
- `go test -race -short ./...` full suite PASS

Both findings real and useful, accepted in 1 cycle.
