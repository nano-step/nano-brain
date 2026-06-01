# Design — Tighten Chunker/Embed Contract

## Problem

Two thresholds in two different files have been allowed to drift:
- `internal/chunk/chunk.go:29` → `Config{TargetSize: 3600, ...}` → max chunker output = 4000.
- `internal/embed/queue.go:42` → `defaultMaxEmbedChars = 3000` → embed budget.

The 1000-char gap is silent: chunker emits, queue truncates, log fires. User cannot eliminate by raising `embedding.max_chars` because chunker's output ceiling doesn't move with it.

## Goals

1. **Eliminate the gap.** Chunker default's max output MUST equal embed queue's default budget.
2. **Preserve hard-split safety net from #297.** The 5-tier boundary-search logic still applies; only the threshold value changes.
3. **No retroactive re-chunking required.** Existing chunks in DB stay valid.

## Decisions

### D1. Change `TargetSize`, not `searchWindow`

**Decision:** Set `TargetSize = 2600`. Leave `searchWindow = 800`.

**Rationale:**
- `searchWindow` controls boundary-search slack around the target. Shrinking it would reduce break-quality for normal markdown (less room to find a good `\n\n` or `## ` break).
- `TargetSize` is the natural place to set the "we aim for chunks this big" knob.
- Math: `2600 + 400 = 3000` exactly matches the embed queue.

**Alternatives considered:**
- Raise embed queue's `defaultMaxEmbedChars` to 4000: rejected. The 3000 value was chosen to leave safety margin under nomic-embed-text's 2048-token limit (per `queue.go:36-41` comment: SentencePiece tokenizes dense CSV at ~1 char/token, 3000 chars → ~2300 tokens worst case). Raising to 4000 would risk 400-error rejection from the provider.
- Plumb `embedding.max_chars` into `chunker.Config`: future work. Requires touching 9 callsites. Defer to a follow-up; for now the simpler fix solves the user's immediate pain.

### D2. Adjust `Overlap` and `MinSize` proportionally?

**Decision:** Keep `Overlap = 200`, `MinSize = 200` unchanged.

**Rationale:** They're absolute (chars of context preserved between consecutive chunks; min size below which trailing chunks merge). Their meaning doesn't depend on TargetSize. Keep them at the proven values.

### D3. Are the #297 hard-split tests still valid?

**Decision:** Yes, with one tweak — all tests that assert `allowed = TargetSize + searchWindow/2` now compute to 3000 instead of 4000. Tests are PARAMETRIZED on the config; the change is mechanical.

Concretely:
- `TestSplit_HardSplit_SingleLongLine`: still produces multiple chunks. Now max chunk is 3000 instead of 3600.
- `TestSplit_HardSplit_Pathological`: 1MB → more chunks (about 385 instead of 278), still bounded.
- `TestSplit_HardSplit_UTF8_CJK`: still valid UTF-8; max chunk shrinks.
- `TestSplit_HardSplit_NormalContentUnchanged`: needs a fresh assertion — normal markdown still produces correct chunks, but the byte counts on each chunk DIFFER from pre-#297 by design.

### D4. Should the warning level change?

**Decision:** No. Keep WRN. After this fix, the warning fires only when a user explicitly sets `embedding.max_chars` LOWER than the chunker's natural output (3000). That's a misconfiguration worth surfacing.

## Algorithm change

```diff
-func DefaultConfig() Config {
-       return Config{TargetSize: 3600, Overlap: 200, MinSize: 200}
-}
+func DefaultConfig() Config {
+       return Config{TargetSize: 2600, Overlap: 200, MinSize: 200}
+}
```

Plus updated test expectations (search/replace `4000 → 3000` in the chunker test file's `allowed` variable, recompute expected chunk counts where assertion is on count).

## Test plan

1. **Live reproduction of #300 in test:** create a fixture mimicking the user's trace JSON, run `chunk.Split` with `DefaultConfig`, assert all chunks ≤ 3000.
2. **All #297 tests pass with updated thresholds.**
3. **Round-trip preserved.**
4. **`go test -race -short ./...` full suite green.**

## Rollback

Revert one constant + tests. Existing chunks in DB unaffected.

## Future work (separate issues)

- Plumb `embedding.max_chars` through every callsite of `chunk.Split` so the chunker adapts to user config.
- Migration command to re-chunk old oversized chunks (out of scope; not blocking).
