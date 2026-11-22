# Prevent code-summarize backlog stalls (#599, #608)

## Why

`GetUnsummarizedSymbols` has a `LIMIT` without `ORDER BY` or failure
eligibility: a symbol that repeatedly gets HTTP 524 is selected again every
cycle, while new symbols wait. A worker then gives an entire cycle to each
workspace in fixed order; a large or actively edited workspace can hold a
single worker for hours. A partial model response leaves unmatched symbols
eligible immediately, causing the same wasted batches.

## What

Prioritize never-attempted symbols, park permanent failures, retry transient
ones after bounded backoff with an attempt cap, and give every workspace one
batch per worker sweep. Record partial-response omissions so they do not
monopolize the next cycle.
