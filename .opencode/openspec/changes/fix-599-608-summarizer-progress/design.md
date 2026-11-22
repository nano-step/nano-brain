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
