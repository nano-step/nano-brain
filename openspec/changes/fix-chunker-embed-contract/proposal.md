Tracking: #300

## Why

After PR #298 (v2026.6.0104), the embed queue's `chunk truncated before embedding` warning still fires on harvested JSON traces with `original_len=3671 truncated_len=3000`. User confirmed running on the released binary.

Root cause: a 1000-char gap between two thresholds.

| Component | Max output |
|---|---|
| Chunker `DefaultConfig` | `TargetSize 3600 + searchWindow/2 400 = 4000 chars` |
| Embed queue default | `defaultMaxEmbedChars = 3000 chars` |

Any chunk in the band `3000 < len ≤ 4000` passes chunker validation but trips the queue. PR #298 only closed the `> 4000` path (oversized lines via `enforceMaxSize`); the pre-existing `3000–4000` mismatch remained.

## What Changes

Lower the chunker's `DefaultConfig.TargetSize` from **3600 → 2600** so that the chunker's worst-case output (`TargetSize + searchWindow/2`) is **3000**, matching the embed queue's default truncate threshold exactly.

Additionally, the `hardSplit` safety net from #297 keeps the same algorithm, but its threshold updates automatically because it computes from `cfg.TargetSize + searchWindow/2`.

After this change:

| Component | Max output |
|---|---|
| Chunker `DefaultConfig` | `TargetSize 2600 + searchWindow/2 400 = 3000 chars` |
| Embed queue default | `3000 chars` (unchanged) |

No more silent truncation. The `WARN chunk truncated` log becomes a true safety net that fires only when user mis-configures `embedding.max_chars` below the chunker's output.

## Capabilities

### Modified Capabilities

- `chunker`: tighten the existing "Bounded Chunk Size" contract from `≤ TargetSize + searchWindow/2 = 4000` to `≤ 3000` (matching the embed pipeline's default budget).

## Impact

- **Code:** 1-line change to `internal/chunk/chunk.go:DefaultConfig`. 13 new + existing tests need their expected `maxAllowed` updated from 4000 → 3000.
- **Behavior:** Chunks are smaller on average. Documents previously producing N chunks of ~3600 chars now produce ~1.4N chunks of ~2600 chars. The vector index grows proportionally (estimated ~40% more chunks for large documents).
- **Risk:** Low — additive in the sense that chunks are smaller, never larger. Vector search recall may improve slightly (more granular semantic units) at the cost of more storage and embedding API calls.
- **Compatibility:** Existing chunks in the DB remain valid; only new chunks are smaller.

## Out of Scope

- Plumbing `embedding.max_chars` into the chunker as a runtime config (future enhancement — would let the chunker adapt automatically if user changes the embed limit).
- Re-chunking existing documents — separate migration concern.
- Token-true counting — independent issue.
