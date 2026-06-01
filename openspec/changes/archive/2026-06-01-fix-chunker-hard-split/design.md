# Design — Chunker Hard-Split Safety Net

## Problem

`chunk.Split` produces unbounded chunks for inputs where any single line exceeds `TargetSize`. The line-aware algorithm has no fallback at sub-line granularity.

## Goals

1. **Bound chunk size.** Every chunk emitted by `chunk.Split` must satisfy `len(content) <= TargetSize + searchWindow/2`.
2. **Preserve quality for normal content.** Markdown / code with reasonable line lengths must produce the SAME chunks as today.
3. **UTF-8 safe.** Never cut a multibyte rune in half. Use `utf8.RuneStart` to validate boundaries.

## Non-goals

- Token-true counting (we measure bytes via `len()`).
- Reordering or merging across the new split boundaries.
- Markdown-aware re-rendering of split pieces (e.g. closing an open `**`).

## Decisions

### D1. Post-process, not in-place rewrite

**Decision:** Add a post-process pass at the end of `chunk.Split`:

```
chunks := buildChunks(...)   // existing
chunks = enforceMaxSize(chunks, cfg)  // NEW
// existing trailing-merge logic
// existing Sequence + Hash assignment
```

**Rationale:** Touch the smallest possible surface. `findSplitPoints` and `buildChunks` keep their existing semantics — they are line-aware and remain so. The new code path only fires on chunks already known to exceed the threshold.

**Alternative considered:** Rewrite `findSplitPoints` to consider char-level boundaries. Rejected because it complicates the well-tested break-score logic and risks regressing line-quality for normal inputs.

### D2. Threshold = `TargetSize + searchWindow/2`

**Decision:** Hard-split kicks in only when `len(content) > TargetSize + searchWindow/2` (4000 chars with defaults).

**Rationale:** `findSplitPoints` already accepts chunks up to `TargetSize + 400` as "acceptable" (the search window is `[target-400, target+400]`). Treating anything within that band as "no further action" preserves existing behavior on every test fixture.

### D3. Boundary priority

**Decision:** Within each oversized chunk, look for the best sub-line boundary in the LAST quarter of the allowed range (`[TargetSize*0.75, TargetSize]`), in priority order:

1. **Blank-line marker** `\n\n` — preserves paragraph integrity.
2. **Single newline** `\n` — preserves line integrity.
3. **Sentence terminator** `. `, `! `, `? `, `。` — preserves prose flow.
4. **Whitespace** ` `, `\t` — preserves word integrity.
5. **Rune boundary** — hard cut at the latest valid UTF-8 rune start ≤ `TargetSize`.

**Rationale:** Mirrors how `findSplitPoints` already prefers high-quality breaks. The "last quarter" window prevents creating tiny shards.

### D4. UTF-8 safety

**Decision:** Use `utf8.RuneStart(s[i])` to validate boundaries before cutting. If a chosen offset is mid-rune, walk left until `RuneStart(s[i]) == true`.

**Rationale:** Go's `len(string)` is byte-counted. Cutting at byte index `N` mid-rune produces invalid UTF-8 that breaks the embed provider (Ollama rejects, Voyage AI may error). All embed providers in nano-brain require well-formed UTF-8.

### D5. Recursive vs iterative

**Decision:** Iterative — keep splitting the tail until the remainder is within bounds.

```go
for len(remaining) > TargetSize + searchWindow/2 {
    cut := findBoundary(remaining, TargetSize)
    out = append(out, remaining[:cut])
    remaining = remaining[cut:]
}
out = append(out, remaining)
```

**Rationale:** Bounded loop, no stack risk on pathological 1MB minified files. Each iteration reduces the remainder by at least `TargetSize * 0.75` (worst case: smallest boundary found).

### D6. Line range tracking

**Decision:** All hard-split pieces of an oversized chunk inherit the **same** `StartLine` and `EndLine` as the parent chunk.

**Rationale:** Hard-split happens within a single line (or contiguous lines that defy splitting). We cannot derive accurate per-piece line numbers without re-parsing. Inheriting parent's range is correct for the common case (single giant line) and a minor over-report for the rare case (multi-line fence-trapped chunk). Acceptable trade-off; observability metrics still work.

### D7. Sequence and Hash

**Decision:** Re-assign `Sequence` and re-compute `Hash` AFTER the hard-split pass, in the existing trailing loop. The trailing-merge logic at chunk.go:68-74 runs BEFORE hard-split, so a tiny tail will already have been merged.

**Rationale:** Existing post-loop is the natural place; no duplication of hash logic.

## Algorithm

```go
func enforceMaxSize(chunks []Chunk, cfg Config) []Chunk {
    maxAllowed := cfg.TargetSize + searchWindow/2
    var out []Chunk
    for _, c := range chunks {
        if len(c.Content) <= maxAllowed {
            out = append(out, c)
            continue
        }
        out = append(out, hardSplit(c, cfg.TargetSize)...)
    }
    return out
}

func hardSplit(c Chunk, target int) []Chunk {
    var out []Chunk
    remaining := c.Content
    for len(remaining) > target + searchWindow/2 {
        cut := findHardBoundary(remaining, target)
        out = append(out, Chunk{
            Content: remaining[:cut],
            StartLine: c.StartLine, EndLine: c.EndLine,
        })
        remaining = remaining[cut:]
    }
    if remaining != "" {
        out = append(out, Chunk{Content: remaining, StartLine: c.StartLine, EndLine: c.EndLine})
    }
    return out
}

func findHardBoundary(s string, target int) int {
    upper := target
    if upper > len(s) { upper = len(s) }
    lower := target * 3 / 4
    if lower < 1 { lower = 1 }

    // Priority 1: blank line marker (\n\n) in [lower, upper]
    for i := upper; i >= lower; i-- {
        if i+2 <= len(s) && s[i] == '\n' && s[i+1] == '\n' {
            return i + 2  // cut AFTER the blank line
        }
    }
    // Priority 2: single newline
    for i := upper; i >= lower; i-- {
        if s[i] == '\n' {
            return i + 1
        }
    }
    // Priority 3: sentence terminator followed by space
    for i := upper - 1; i >= lower; i-- {
        if (s[i] == '.' || s[i] == '!' || s[i] == '?') && i+1 < len(s) && s[i+1] == ' ' {
            return i + 2
        }
    }
    // Priority 4: whitespace
    for i := upper; i >= lower; i-- {
        if s[i] == ' ' || s[i] == '\t' {
            return i + 1
        }
    }
    // Priority 5: nearest UTF-8 rune start <= target
    cut := upper
    for cut > 0 && !utf8.RuneStart(s[cut]) {
        cut--
    }
    if cut == 0 {
        cut = upper  // pathological — content is one giant rune sequence; cut anyway
    }
    return cut
}
```

## Test plan

1. **Existing tests pass** — no change to normal-content behavior.
2. **Long single line** — 10,000-char line of `a` produces 3 chunks, each `<= 4000` chars.
3. **Long single line, prose** — split prefers sentence terminator over arbitrary cut.
4. **Fence-trapped content** — text inside an unclosed `\`\`\`` fence is still split.
5. **UTF-8 multibyte** — input with emoji every 100 chars; verify no chunk has invalid UTF-8 via `utf8.ValidString`.
6. **CJK content** — 4000 chars of CJK (each 3 bytes UTF-8) → multiple chunks, all valid UTF-8.
7. **Pathological** — 1MB of `x` with no newlines → many chunks, all `<= 4000`, no panic.
8. **Mixed** — normal markdown + 1 giant minified blob → markdown chunks unchanged, blob hard-split.
9. **Round-trip** — concatenating all chunk Contents reproduces the input exactly (no data loss, no duplication) MODULO overlap.

## Rollback

Pure additive post-process. Rollback = revert the PR. No data migration. Existing chunks in `chunks` table remain valid.

## Validation

- `go test -race -short ./internal/chunk/...` PASS
- `go test -race -short ./...` PASS
- Real-world test: re-index capyhome workspace (contains long-line minified files) and grep server log for `chunk truncated before embedding` — should be zero after fix.
