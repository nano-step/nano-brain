## Context

`ConsolidationWorker` runs an LLM to detect conflicts between memory docs and writes decisions (DELETE/UPDATE/ADD/NOOP) to `consolidation_log`. Currently no code reads `consolidation_log` and applies decisions. `supersedeDocument(oldId, newId)` exists in the store and is a soft operation (sets `superseded_by` column, does not delete).

`consolidation_log` schema today:
```sql
id, document_id, action, reason, target_doc_id, model, tokens_used, created_at
```

No tracking of whether a decision was executed.

## Goals / Non-Goals

**Goals:**
- Execute pending DELETE decisions automatically after each consolidation job
- Idempotent: safe to run multiple times
- Fail-open: skip invalid entries, never halt the batch

**Non-Goals (v1):**
- UPDATE decision handling (requires content merging — v2)
- Qdrant vector cleanup for superseded docs (v2)
- Manual MCP trigger tool (v2)
- Reversibility/undo of applied decisions

## Decisions

### 1. Integration point: piggyback on ConsolidationWorker (not a separate job)
ConsolidationWorker already has a polling loop. After `processConsolidationJob()` succeeds, call `reconciler.applyPendingDecisions()`. No new scheduler, no new cron.

*Alternative rejected*: Separate polling job — adds operational complexity for no benefit.

### 2. Idempotency via `applied_at` column
Add `applied_at TEXT DEFAULT NULL` and `applied_error TEXT` to `consolidation_log`. Filter: `WHERE action = 'DELETE' AND applied_at IS NULL AND target_doc_id IS NOT NULL`. Per-entry stamping means partial failures retry cleanly on next run.

*Alternative rejected*: Separate `reconciliation_log` table — adds joins and complexity with no benefit.

### 3. Trigger: auto (not manual)
User confirmed: auto-apply after each consolidation job. Manual MCP trigger deferred to v2.

### 4. Guard semantics: fail-open
Before calling `supersedeDocument(sourceId, targetId)`:
1. Source doc must exist and be active
2. Source doc must not already have `superseded_by` set (already reconciled)
3. Target doc must exist and be active

On any guard failure: stamp `applied_error`, mark `applied_at`, continue batch.

### 5. ADD/NOOP entries: auto-stamp without action
Mark `applied_at = now()` immediately — no action needed. Keeps the pending queue clean.

## Risks / Trade-offs

| Risk | Mitigation |
|---|---|
| LLM made wrong DELETE decision | `supersedeDocument` is soft — doc still exists in DB, `superseded_by` can be manually cleared |
| Source doc deleted between consolidation and reconciliation | Guard clause: skip + log error |
| Qdrant vectors of superseded doc remain searchable | Accepted tech debt — scoring demotion (×0.05) reduces their rank; full cleanup deferred to v2 |
| Reconciliation fails mid-batch | Per-entry `applied_at` — entries 1-4 applied, entry 5 fails, retried next run automatically |

## Migration Plan

1. Schema migration v11 runs on server startup (existing pattern — `ALTER TABLE IF NOT EXISTS`)
2. Existing `consolidation_log` rows get `applied_at = NULL` by default — will be processed on first reconciliation run
3. No rollback needed — adding nullable columns is non-destructive

## Open Questions

None — all decisions settled with user input.
