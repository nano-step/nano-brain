## Why

Issue #542 is a dogfooding tracker whose nine actionable findings have shipped
as focused fixes or a bounded mitigation. Its remaining extraction-time
call-resolution work is high-risk and needs an independently scoped issue;
the final embedding-ranking finding is an intentional product boundary.
Closing the tracker without recording those dispositions would make the
remaining work and the won't-fix decision hard to discover.

## What Changes

- Add a closure record that maps every #542 finding to its delivered fix,
  follow-up issue, or explicit by-design decision.
- Link F2's remaining extraction-time JS/TS call-resolution work to #609,
  which owns its high-risk proposal and implementation lifecycle.
- Record F10 as won't-fix/by-design because embedding retrieval cannot safely
  infer domain-specific distinctions without product-owned vocabulary.
- Close the tracker after publishing the disposition record and GitHub handoff.

## Capabilities

### New Capabilities

- `issue-542-dogfooding-report-disposition`: Preserve the final disposition of
  every finding in issue #542, including shipped fixes, deferred follow-ups,
  and by-design decisions.

### Modified Capabilities

<!-- None. This change does not alter a runtime product requirement. -->

## Impact

- Documentation and OpenSpec artifacts only; no Go code, API, schema, or
  runtime behavior changes.
- GitHub issue #542 receives its final status and links to the new #609
  high-risk follow-up.
