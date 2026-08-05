# Intake: Source-Scoped Call Resolution (#609)

## Classification

| Field | Decision |
| --- | --- |
| Tracking issue | [#609](https://github.com/nano-step/nano-brain/issues/609) |
| Lane | High-risk |
| Change type | Bug fix |
| Existing issue labels | `lane:high-risk`, `change-type:bug-fix`, `status:proposal` |
| Hard gate | Search quality: call identity changes graph extraction, trace, impact, and flow accuracy |
| Additional risk flags | Public contracts, existing behavior, weak proof, and multi-domain behavior |

The issue's stated acceptance criteria require source-reachable call targets,
safe unresolved behavior, and `memory_trace`, `memory_flow`, and reverse-impact
coverage. The implementation is therefore high-risk even though this task adds
only planning and evidence files.

## Authorization Provenance

The explicit user command `$omo:start-work issue 609` is the go-ahead for
planning and execution. It does not introduce a separate product or design
approval: the selected technical decisions are the ones recorded in the
approved #609 work plan and this change's proposal, design, specification, and
story packet. No additional human approval is claimed by this record.

## Gates

Planned implementation gates are: strict OpenSpec validation; focused unit
fixtures; `go build ./... && go test -race -short ./...`; integration tests
against `nanobrain_test`; a bounded `:3199` REST/MCP smoke; applicable
capability benchmark comparison; privacy scan; diff self-review; and an
independent review before archive.

This task does not claim the later implementation, smoke, benchmark, or review
gates have passed. The current planning state reports phase 13 as `verifying`,
so the delivery coordinator must reconcile the high-risk GSD/deep-design gate
before implementation. No planning-state file was changed here.
