## ADDED Requirements

### Requirement: Execute pending DELETE decisions from consolidation_log
The system SHALL automatically execute DELETE decisions from `consolidation_log` by calling `supersedeDocument(sourceId, targetId)` after each successful consolidation job.

#### Scenario: DELETE decision applied successfully
- **WHEN** `consolidation_log` contains a row with `action = 'DELETE'`, `applied_at IS NULL`, and valid `document_id` + `target_doc_id`
- **THEN** `supersedeDocument(document_id, target_doc_id)` is called, the source doc's `superseded_by` is set to `target_doc_id`, and `applied_at` is stamped with the current timestamp

#### Scenario: Source doc already superseded
- **WHEN** the source doc already has `superseded_by` set (manually or by a previous reconciliation)
- **THEN** the entry is skipped, `applied_error` is set to a descriptive message, `applied_at` is stamped, and the batch continues

#### Scenario: Source doc missing or inactive
- **WHEN** `document_id` does not exist in `documents` or `active = 0`
- **THEN** the entry is skipped, `applied_error` is set, `applied_at` is stamped, and the batch continues

#### Scenario: Target doc missing or inactive
- **WHEN** `target_doc_id` does not exist in `documents` or `active = 0`
- **THEN** the entry is skipped, `applied_error` is set, `applied_at` is stamped, and the batch continues

#### Scenario: Idempotent — already applied entries are skipped
- **WHEN** reconciliation runs again and entries already have `applied_at` set
- **THEN** those entries are not processed again and `supersedeDocument` is not called twice

### Requirement: Auto-stamp ADD and NOOP entries without action
The system SHALL mark `applied_at` on entries with `action IN ('ADD', 'NOOP', 'FAILED')` without taking any action, to keep the pending queue clean.

#### Scenario: NOOP entry stamped
- **WHEN** `consolidation_log` contains a row with `action = 'NOOP'` and `applied_at IS NULL`
- **THEN** `applied_at` is set to the current timestamp and no document modifications are made

### Requirement: Reconciliation runs automatically after each consolidation job
The system SHALL trigger reconciliation immediately after `ConsolidationWorker` successfully processes a job, with no manual intervention required.

#### Scenario: Auto-trigger after consolidation
- **WHEN** `processConsolidationJob()` completes successfully
- **THEN** `ReconciliationRunner.applyPendingDecisions()` is called before the worker picks up the next job

#### Scenario: Consolidation job fails — no reconciliation
- **WHEN** `processConsolidationJob()` throws an error
- **THEN** reconciliation is NOT triggered for that cycle

### Requirement: Dry-run mode available as opt-in
The system SHALL support a `dryRun` flag on `applyPendingDecisions(dryRun?: boolean)` that returns what would be applied without making any changes.

#### Scenario: Dry-run returns planned actions
- **WHEN** `applyPendingDecisions(true)` is called
- **THEN** the return value lists all pending entries that would be processed, and no `supersedeDocument` calls are made, and no `applied_at` timestamps are written
