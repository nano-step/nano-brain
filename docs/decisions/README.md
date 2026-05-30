# Architecture Decision Records (ADRs)

Durable records of architectural and design decisions. Each ADR captures **why**
a decision was made, what alternatives were considered, and the tradeoffs accepted.

## When to create an ADR

Create an ADR when ANY of these apply:

- Story is `lane: high-risk`
- Decision affects ≥ 2 modules or services
- Decision is hard to reverse (data model, API contract, external dependency)
- Decision is intentionally different from prior pattern (record why)
- User says "rethink how X works" or "migrate from X to Y"

**Do NOT** create an ADR for: typo fixes, dependency bumps, refactors with
identical I/O, tactical decisions that fit existing patterns.

## File naming

```
NNNN-kebab-case-title.md
```

- `NNNN` — zero-padded sequential number (0001, 0002, ...). Never reuse.
- Title — short kebab-case summary (e.g. `0001-use-postgres-for-search`).

## Lifecycle

| Status | Meaning |
|---|---|
| `Proposed` | Draft; not yet implemented |
| `Accepted` | Implemented; current truth |
| `Superseded by NNNN` | Replaced by a newer ADR; keep file for history |
| `Rejected` | Considered and rejected; keep for institutional memory |

Never delete an ADR. Supersede or reject instead — history is part of the value.

## Template

See [`../templates/adr.md`](../templates/adr.md).

## Index

(Add entries as ADRs are created.)

| # | Title | Status | Date |
|---|---|---|---|
| 0001 | _none yet_ | — | — |
