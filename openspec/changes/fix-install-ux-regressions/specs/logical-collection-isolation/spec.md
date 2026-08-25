# logical-collection-isolation Specification

## Purpose

`memory` and `sessions` are DB-backed namespaces whose documents carry identifiers rather than file paths. They must never be walked from disk, so the reindex orphan-deletion loop can never reach them, while remaining fully targetable by force-wipe and by every consumer that filters on the collection name.

## ADDED Requirements

### Requirement: Incremental reindex SHALL NOT walk logical collections

`triggerIncremental` SHALL skip any collection named `memory` or `sessions` before calling `walkCollectionFiles`, regardless of whether that collection's `path` exists, is empty, or contains files.

#### Scenario: Logical collection with a missing root produces no warning

- **WHEN** an incremental reindex runs for a workspace whose `sessions` collection has `path = ~/.nano-brain/sessions` and that directory does not exist
- **THEN** no `walk collection "sessions": root path inaccessible` warning is logged
- **AND** the reindex response counts no scan, skip, or delete for that collection

#### Scenario: Logical collection with a populated root deletes nothing

- **WHEN** `~/.nano-brain/sessions` exists and contains one or more files, and the workspace has indexed `sessions` documents whose `source_path` values begin with `summary://`
- **THEN** an incremental reindex deletes **zero** documents and **zero** chunks from that collection
- **AND** no `disk walk returned empty for non-empty collection` warning is logged

#### Scenario: Documents with an empty source path are not deletion-eligible

- **WHEN** the `memory` collection contains documents whose `source_path` is the empty string, and `~/.nano-brain/memory` contains at least one file
- **THEN** an incremental reindex deletes **zero** of those documents

#### Scenario: The code collection is still walked

- **WHEN** an incremental reindex runs for a workspace whose `code` collection root exists and contains files
- **THEN** that collection is walked, scanned, and orphan-swept exactly as before this change

### Requirement: Force-wipe SHALL still target logical collections

`triggerForceWipe` SHALL continue to reset and re-enqueue chunks for `memory` and `sessions`. The walk exclusion applies only to the incremental path.

#### Scenario: Force-wipe resets session chunks

- **WHEN** a reindex is requested with force-wipe for a workspace holding `sessions` documents
- **THEN** `ResetAndReturnChunkIDsByCollection` is called for the `sessions` collection
- **AND** its chunks are re-enqueued for embedding

### Requirement: Logical collection rows SHALL be registered unconditionally

`POST /api/v1/init` SHALL create `memory`, `sessions`, and `code` collection rows for every workspace, independent of whether any of those directories exists. Registration SHALL NOT depend on filesystem state.

#### Scenario: Registration is deterministic across machines

- **WHEN** two machines register the same workspace root, one with `~/.nano-brain/memory` present and one without
- **THEN** both produce the same three collection rows

#### Scenario: Logical collection rows are never deleted by reindex

- **WHEN** any reindex runs, incremental or force-wipe
- **THEN** the `memory` and `sessions` collection rows still exist afterwards

### Requirement: Logical collections SHALL NOT be attached to the filesystem watcher

The post-init watcher attach list SHALL contain only disk-backed collections. `memory` and `sessions` SHALL NOT be watched at init time, matching the behavior already applied at daemon startup.

#### Scenario: Init-time attach matches restart-time attach

- **WHEN** a workspace is registered and the daemon is then restarted
- **THEN** the set of watched collections is identical before and after the restart
