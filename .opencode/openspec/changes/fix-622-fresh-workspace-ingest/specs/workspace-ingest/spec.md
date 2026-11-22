# Workspace Ingest

## ADDED Requirements

### Requirement: Cold workspace ingestion
Registering an accessible workspace SHALL eventually persist documents and
chunks for its admitted files when its watcher runs and reindex is triggered.

#### Scenario: Newly registered source root
Given a new root containing a supported source file,
when the root is registered and reindex is requested with a running watcher,
then its document and chunks are persisted in that workspace.

#### Scenario: Idle watcher after registration
Given an empty registered source root whose initial scan has finished,
when a file appears without a filesystem notification and reindex requests
a rescan, then the watcher is woken and persists the file without waiting
for a later event or periodic poll.

#### Scenario: Idle watcher after registration
Given an empty registered source root whose initial scan has finished,
when a file appears without a filesystem notification and reindex requests
a rescan, then the watcher is woken and persists the file without waiting
for a later event or periodic poll.

### Requirement: Honest reindex response
A reindex response MUST NOT claim that documents were embedded or queued
when only an asynchronous disk scan was scheduled.

#### Scenario: Watcher not running
Given a registered workspace with a stopped watcher,
when reindex scans its root,
then the response distinguishes files needing ingestion from persisted or
queued chunks.

#### Scenario: Asynchronous progress event
When the reindex handler returns before watcher ingestion finishes,
then its event state is `queued`, not `completed`.

#### Scenario: Asynchronous progress event
When the reindex handler returns before watcher ingestion finishes,
then its event state is `queued`, not `completed`.
