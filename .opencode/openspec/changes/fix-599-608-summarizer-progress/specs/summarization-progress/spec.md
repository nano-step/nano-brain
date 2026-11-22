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

#### Scenario: Explicit retry
Given a parked symbol,
when its failure is resolved by the retry endpoint,
then it is eligible as a never-attempted symbol again.

### Requirement: Bounded worker occupancy
The automatic worker SHALL give each registered workspace at most one
configured batch before advancing to the next workspace; it SHALL rotate
the starting workspace between sweeps.

#### Scenario: Two large workspaces
Given two registered workspaces each with more than one batch pending,
when one worker sweeps both,
then both receive a chance to submit one batch before either receives a
second batch.

### Requirement: Partial model response
Missing members of a successful model batch SHALL be reported and cooled
down so the next cycle can select unseen symbols instead.

#### Scenario: One of five summaries returned
Given a batch of five requested symbols,
when the provider returns only one matching summary,
then that summary is stored and the other four are recorded for later retry.
