# Closure Record — Issue #542 Dogfooding Report

## Scope

Issue #542 reported ten frictions encountered while reconstructing an HTTP
flow through nano-brain's MCP tools. This record closes the tracker without
claiming that its remaining graph-quality work is complete.

All links below were verified on 2026-07-17.

## Finding Disposition

| Finding | Disposition | Evidence |
| --- | --- | --- |
| F1 — mounted HTTP URL was not resolvable | Fixed | [#563](https://github.com/nano-step/nano-brain/issues/563) · [PR #564](https://github.com/nano-step/nano-brain/pull/564) |
| F2 — cross-repository bare-name call collisions | Mitigated and handed off | [#575](https://github.com/nano-step/nano-brain/issues/575) · [PR #576](https://github.com/nano-step/nano-brain/pull/576) · remaining root cause: [#609](https://github.com/nano-step/nano-brain/issues/609) |
| F3 — trace ignored `max_depth` | Fixed | [#561](https://github.com/nano-step/nano-brain/issues/561) · [PR #562](https://github.com/nano-step/nano-brain/pull/562) |
| F4 — hybrid query latency | Fixed | [#539](https://github.com/nano-step/nano-brain/issues/539) · [PR #540](https://github.com/nano-step/nano-brain/pull/540) |
| F5 — workspace resolution hid ancestor coverage | Fixed | [#565](https://github.com/nano-step/nano-brain/issues/565) · [PR #566](https://github.com/nano-step/nano-brain/pull/566) |
| F6 — reverse graph omitted route-to-handler edge | Fixed | [#569](https://github.com/nano-step/nano-brain/issues/569) · [PR #570](https://github.com/nano-step/nano-brain/pull/570) |
| F7 — hybrid query ignored `chunk_type` on vector legs | Fixed | [#571](https://github.com/nano-step/nano-brain/issues/571) · [PR #572](https://github.com/nano-step/nano-brain/pull/572) |
| F8 — flow emitted builtin and keyword nodes | Fixed | [#567](https://github.com/nano-step/nano-brain/issues/567) · [PR #568](https://github.com/nano-step/nano-brain/pull/568) |
| F9 — filtered multi-word search returned a false negative | Fixed | [#573](https://github.com/nano-step/nano-brain/issues/573) · [PR #574](https://github.com/nano-step/nano-brain/pull/574) |
| F10 — embedding conflated distinct domain terms | Won't fix / by design | Semantic distinction requires product-owned vocabulary and ranking policy; no correctness defect exists in generic embedding retrieval. |

## F2 Boundary

PR #576 adds a query-time path-proximity rule: when exactly one candidate is
nearest to the caller, trace uses it; a tie remains explicit ambiguity. That
is intentionally not proof that the candidate is source-reachable, and it
does not remove false call edges at extraction time or resolve flow and impact
ambiguity.

Issue #609 owns the required high-risk work: source-scoped JS/TS call
resolution using actual imports, same-module declarations, and known receiver
types, together with reindex and regression evidence. The tracker is therefore
complete as a report; F2's root cause remains open in its own lifecycle.

## Closure Decision

Close #542 after this record merges. F1 and F3–F9 are shipped, F2 has a clear
follow-up owner, and F10 is a documented product boundary.
