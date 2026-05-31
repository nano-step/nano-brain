# Gemini Triage — fix-embed-queue-pending-counter-leak (#272)

PR: nano-step/nano-brain#273
Date: 2026-05-31
Bot reviewer: gemini-code-assist[bot]
Agent: Sisyphus

## Verdict: PROPOSAL_REJECTED

Gemini correctly identified that the entire premise of this fix is wrong. After tracing the code carefully (queue.go:97-114 Enqueue, queue.go:175-203 scanPending → Enqueue), I confirm:

**`pending` counter semantics:** Tracks chunks **currently in the in-memory channel + workers (in-flight)**. NOT a mirror of `COUNT(chunks WHERE embed_status='pending')` in DB.

**Enqueue (line 106) increments `pending` only on successful channel send.**
**scanPending (line 199) calls Enqueue, which re-increments `pending` on every scan re-enqueue.**

## Trace That Proves Gemini Right

1. Initial enqueue → `pending=1`, channel has 1
2. Worker pops → channel=0, `pending=1` (still in-flight)
3. MarkChunkEmbedded fails → if we don't decrement: `pending=1`, channel=0, **chunk no longer in flight**
4. 5min scan runs → finds `embed_status='pending'` → calls Enqueue → `pending=2`, channel=1
5. Worker pops → channel=0, `pending=2`
6. Embed succeeds, MarkChunkEmbedded succeeds → `pending.Add(-1)` → `pending=1`
7. Chunk now `embed_status='embedded'` → never scanned again → **`pending` permanently leaks at 1**

Over many such cycles under transient errors → leak accumulates → hits rejectionThreshold (50000) → permanent queue lockup. **This is exactly the saturation symptom my fix was supposed to address.**

## Triage Table (R31)

| Comment ref | Agent verdict | Reasoning | Action |
|-------------|---------------|-----------|--------|
| queue.go:310 (MarkChunkEmbedded — remove pending.Add(-1)) | VALID:critical | Gemini's leak trace is correct. Removing the decrement causes permanent counter leak as shown above. | REVERT |
| queue.go:366 (handleRetry channel-full — remove pending.Add(-1)) | VALID:critical | Same leak mechanic applies. | REVERT |
| queue_test.go:621 / 883 / 917 (test expectations changed to "pending unchanged") | VALID:medium | Tests encode the wrong invariant. Must assert pending decrements to 0. | REVERT to original expectations |
| design.md:50 / 57 (D1 + D2 decisions based on wrong invariant) | VALID:medium | The invariant `pending == COUNT(DB pending)` is false. pending tracks in-memory queue + workers. | REVERT proposal/design |

## What Was Wrong With My Analysis

I misread the relationship between `pending` and `embed_status`. The explore agent's report said "queue_pending = q.pending.Load() (lifetime backlog counter, can exceed channel)" — that was correct in describing the counter's value range, but the counter is NOT a mirror of DB state. It's a high-water-mark-style tracker of in-flight + queued work, where:
- Each successful Enqueue increments
- Each successful processChunk completion (any terminal state) decrements
- Drops from queue MUST decrement to avoid leak

The bug I sought (production saturation at 9999/10000) is REAL but the root cause must be elsewhere.

## Hypotheses for Next Investigation

1. **The rejectionThreshold of 50000 vs `len(q.ch)` cap of 10000**: status endpoint reports `queue_pending` (the `pending` atomic, can be up to 50000) but the "9999/10000" string suggests the operator is looking at channel depth (10K cap). Maybe production really IS at channel cap, meaning workers can't drain fast enough — that's not a leak, that's an embed-provider throughput problem.
2. **handleRetry leak from goroutine context cancellation**: If `ctx.Done()` fires while processChunk is mid-flight, are all paths properly decrementing pending?
3. **publishStatus event reporting `pending` not `len(channel)`** confusing operators about what's saturated.

## Resolution

- Close PR #273 without merging
- Close issue #272 as "invalid - proposal based on wrong invariant"
- Remove worktree
- File new investigation issue: "Confirm whether 9999/10000 is channel depth or pending counter, and identify the real saturation cause"

## Loop count
1/3 push cycles (this is cycle 1; the result is full revert).
