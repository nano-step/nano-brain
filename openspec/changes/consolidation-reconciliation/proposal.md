## Why

nano-brain's consolidation job detects conflicting knowledge (e.g., "2025: use approach X" vs "2026: switched to Y"), decides to DELETE the outdated doc, but only logs the decision to `consolidation_log` — it never executes it. Both conflicting docs remain active in search results simultaneously, defeating the purpose of conflict detection.

## What Changes

- **New**: `ReconciliationRunner` class (`src/jobs/reconciliation.ts`) reads pending DELETE decisions from `consolidation_log` and executes them via `supersedeDocument()`
- **New**: Schema migration (v11) adds `applied_at TEXT` and `applied_error TEXT` columns to `consolidation_log` for idempotency tracking
- **New**: Prepared statements for pending decision queries and status updates
- **Modified**: `ConsolidationWorker` auto-triggers reconciliation after each successful consolidation job
- **Modified**: Store interface exposes reconciliation methods

## Capabilities

### New Capabilities
- `consolidation-reconciliation`: Execute pending DELETE decisions from `consolidation_log` by calling `supersedeDocument()`, with idempotency tracking, guard clauses, and dry-run support

### Modified Capabilities

## Impact

- `src/store/schema.ts` — schema migration v11, new prepared statements
- `src/store/documents.ts` — new store methods exposed
- `src/jobs/reconciliation.ts` — new file
- `src/jobs/consolidation-worker.ts` — integrate reconciliation trigger
- `src/types.ts` — extend Store interface if needed
