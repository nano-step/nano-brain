# Design

The old report came from migration v29; current source is v30. A real
initialization, watcher loop and reindex in an isolated test schema persisted
the new source file and its chunks, so the original ingestion failure has
not been reproduced on current code. Keep that integration test as a
regression. Do not claim this proves every server configuration ingests
correctly; the old runtime failure still needs a reproduction or its logs.

The confirmed bug is in the response: incremental reindex calls the
asynchronous watcher, but counts changed files as `embedded` and
`chunks_enqueued` before the watcher has inserted a single chunk. Keep the
existing numeric fields, set them to zero when nothing has yet happened,
and add `files_pending_ingest` for the number of new/changed files found.
When a force-wipe has no embedding queue, it cannot claim chunks were
queued. The `watcher_triggered` boolean means a rescan was requested, not
that ingestion has finished; callers should check stats to verify persistence.
