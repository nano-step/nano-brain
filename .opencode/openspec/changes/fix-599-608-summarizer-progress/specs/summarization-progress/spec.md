# Summarization Progress

## ADDED Requirements

### Requirement: Failure-aware selection
The summarizer SHALL select never-attempted symbols before eligible retry
symbols and SHALL NOT repeatedly select permanent, capped or cooling-down
failures in every cycle.

#### Scenario: Persistent HTTP 524
Given an unsummarized symbol with an unresolved HTTP 524 failure,
when the cooldown has not expired,
then it is not selected ahead of unattempted symbols.

#### Scenario: Old transient HTTP 524
Given an unresolved transient failure below the cross-cycle cap with an
expired cooldown,
when no unattempted symbols remain,
then it is eligible for retry.

#### Scenario: Stale graph-context summary
Given an existing summary whose graph context needs recomputation,
when its latest attempt failed and is still cooling down,
then the automatic worker does not reselect it on every idle sweep.

#### Scenario: Explicit retry
Given a parked symbol with an unresolved failure,
when its failure ID is passed to the retry endpoint,
then only the selected symbol bypasses the cooldown and its failure is
resolved only after a successful summary.

#### Scenario: Explicit retry exceeds one cycle
Given more selected failures than `max_summaries_per_cycle`,
when retry-all runs,
then it attempts every selected failure in successive bounded cycles.

#### Scenario: Identical content hashes across files
Given two symbols sharing a content hash but only one failure record,
when that failure is retried,
then only the recorded file and symbol name are submitted.

#### Scenario: Failure state is scoped to the attempted symbol
Given two symbols with the same content hash in different files,
when only one has an unresolved failure and cooldown,
then the other remains eligible for automatic selection.

#### Scenario: Daily budget is exhausted during retry-all
Given multiple retry batches and a budget that expires after the first,
when retry-all returns,
then previously resolved failures count as succeeded and the response
reports that the budget was exhausted.

### Requirement: Bounded worker occupancy
The automatic worker SHALL give each registered workspace at most one
configured batch before advancing to the next workspace; it SHALL rotate
the starting workspace between sweeps.

#### Scenario: Two large workspaces
Given two registered workspaces each with more than one batch pending,
when one worker sweeps both,
then both receive a chance to submit one batch before either receives a
second batch.

#### Scenario: New backlog coexists with stale graph summaries
Given an existing summary needing graph-context refresh and fresh
unsummarized symbols arriving on successive sweeps,
when the worker keeps polling,
then a stale summary gets a bounded turn even while fresh work remains.

#### Scenario: Graph-stale page contains symbols without edges
Given a first page of stale symbols with no graph context and a later
symbol with changed graph edges,
when the worker checks the page,
then the no-context symbols are marked current and the later symbol can
be selected on the next stale turn.

### Requirement: Partial model response
Missing members of a successful model batch SHALL be reported and cooled
down so the next cycle can select unseen symbols instead.

#### Scenario: One of five summaries returned
Given a batch of five requested symbols,
when the provider returns only one matching summary,
then that summary is stored and the other four are recorded for later retry.
