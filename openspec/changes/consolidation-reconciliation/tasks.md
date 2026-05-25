## 1. Schema Migration

- [ ] 1.1 Add `applied_at TEXT DEFAULT NULL` and `applied_error TEXT` columns to `consolidation_log` table via safe v11 migration in `src/store/schema.ts` (follow existing `PRAGMA table_info` guard pattern)
- [ ] 1.2 Add index `idx_consolidation_log_pending ON consolidation_log(action, applied_at)` for efficient pending-entry queries

## 2. Prepared Statements

- [ ] 2.1 Add `getPendingConsolidationActions` statement to `initStatements()` in `src/store/schema.ts`: `SELECT id, document_id, action, target_doc_id FROM consolidation_log WHERE action = 'DELETE' AND applied_at IS NULL AND target_doc_id IS NOT NULL LIMIT 50`
- [ ] 2.2 Add `markConsolidationLogApplied` statement: `UPDATE consolidation_log SET applied_at = datetime('now') WHERE id = ?`
- [ ] 2.3 Add `markConsolidationLogError` statement: `UPDATE consolidation_log SET applied_at = datetime('now'), applied_error = ? WHERE id = ?`
- [ ] 2.4 Add `markNoopLogsApplied` statement: `UPDATE consolidation_log SET applied_at = datetime('now') WHERE action IN ('ADD', 'NOOP', 'FAILED') AND applied_at IS NULL`
- [ ] 2.5 Add `getDocumentActiveStatus` statement: `SELECT id, active, superseded_by FROM documents WHERE id = ?`

## 3. Store Methods

- [ ] 3.1 Expose `getPendingConsolidationActions(): Array<{id: number, document_id: number, target_doc_id: number}>` on the store in `src/store/documents.ts` (or `src/store/index.ts` — follow existing pattern)
- [ ] 3.2 Expose `markConsolidationLogApplied(id: number): void`
- [ ] 3.3 Expose `markConsolidationLogError(id: number, error: string): void`
- [ ] 3.4 Expose `markNoopLogsApplied(): void`
- [ ] 3.5 Expose `getDocumentActiveStatus(id: number): {id: number, active: boolean, supersededBy: number | null} | null`
- [ ] 3.6 Verify `supersedeDocument(targetId: number, newId: number)` already exists in `src/store/documents.ts` — no changes needed if so

## 4. ReconciliationRunner

- [ ] 4.1 Create `src/jobs/reconciliation.ts` with `ReconciliationRunner` class accepting a `Store` in constructor
- [ ] 4.2 Implement `applyPendingDecisions(dryRun = false): { applied: number; skipped: number; errors: number }`:
  - Call `store.markNoopLogsApplied()` first
  - Fetch pending DELETE entries via `store.getPendingConsolidationActions()`
  - For each entry: validate source doc (exists, active, not already superseded) and target doc (exists, active)
  - On guard failure: call `store.markConsolidationLogError(id, reason)` and continue
  - On success (not dryRun): call `store.supersedeDocument(document_id, target_doc_id)` then `store.markConsolidationLogApplied(id)`
  - Return counts of applied/skipped/errors

## 5. ConsolidationWorker Integration

- [ ] 5.1 Import and instantiate `ReconciliationRunner` in `src/jobs/consolidation-worker.ts` constructor
- [ ] 5.2 After successful `processConsolidationJob()` call, invoke `this.reconciler.applyPendingDecisions()` — fire-and-forget with error catch (reconciliation failure must NOT crash the worker)

## 6. Validation

- [ ] 6.1 Run `npx tsc --noEmit` — must exit 0
- [ ] 6.2 Verify schema migration: check `PRAGMA table_info(consolidation_log)` includes `applied_at` and `applied_error`
- [ ] 6.3 Verify guard clause for already-superseded source: manually set `superseded_by` on a doc, insert a DELETE log entry, run reconciliation — entry should be skipped with `applied_error` set
- [ ] 6.4 Verify idempotency: run reconciliation twice — second run applies 0 entries
