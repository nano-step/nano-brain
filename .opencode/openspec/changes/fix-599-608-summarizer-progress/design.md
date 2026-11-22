# Design

Selection joins `code_summarization_failures` by workspace and content hash,
restricted to unresolved rows. It first returns symbols without failures;
transient failures become eligible after an exponential 1/2/4/8/16/24-hour
cooldown, up to 8 cross-cycle failures. Permanent and capped failures remain
visible through the failures API and can be explicitly requeued via
`/api/v1/code/summarize/retry`; a new content hash gets a fresh eligibility
record. Stable order is never-attempted first, then oldest eligible
`last_attempt_at`, then chunk ID.

`RunOnce` keeps its explicit/manual full-cycle limit; worker sweeps use
one configured batch (bounded by max_summaries_per_cycle) per workspace,
and rotate the first workspace across sweeps. This bounds per-workspace
occupancy even when one provider request takes 600s. A successful but
partial response records omitted requested symbols with error_type
`partial_omitted` so they are cooled down rather than selected immediately.
Existing per-request retry remains unchanged.

The explicit retry endpoints pass selected content hashes to
`RetryContentHashes`, bypassing the automatic cooldown/cap without selecting
unrelated backlog. Successful summaries resolve their own failure rows;
the endpoints compare unresolved rows after the call instead of marking
every requested failure successful. Tests cover SQL eligibility/order,
targeted retry, partial responses, and worker quota/rotation using a local
stubbed provider and isolated test schemas.

Independent reviewers found that retry-all selected only the first cycle of
failures. Explicit retry now chunks requested hashes by
`max_summaries_per_cycle` so a request larger than one cycle still attempts
every failure. The persisted failure row identifies a symbol by source file
and name as well as content hash; forced selection must match that identity
to avoid sending unrelated symbols with duplicate hashes. The unresolved
failure table remains keyed by workspace and content hash, so separate
same-hash failures still share retry state; changing that schema is out of
scope for this fix.

Graph-context-stale symbols use the same failure eligibility; otherwise the
automatic worker would retry the same stale summary on every empty-backlog
sweep. Explicit retries must also select stale symbols that already have an
old summary document.
To avoid starving graph-context-stale symbols when a workspace always has
new pending symbols, worker sweeps alternate which queue gets the first
chance. A stale sweep only takes its one bounded batch when it has eligible
work; otherwise it falls through to normal selection. The pre-existing
provider-429 error propagation still needs separate follow-up: a failed
batch becomes failure rows and does not reach the worker's outer 429 backoff.
