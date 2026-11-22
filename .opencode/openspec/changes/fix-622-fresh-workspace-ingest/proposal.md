# Make cold workspace registration actually ingest files (#622)

## Why

The reported `/reindex` response counted files on disk but did not prove
that they reached `documents` or `chunks`. `/update` only re-extracts edges
and symbols from existing documents and cannot repair an empty workspace.

## What

Reproduce registration, watcher startup, reindex, and document persistence
on `nanobrain_test`; fix only the confirmed broken stage. Do not change
production data or issue a force-wipe on the running dev server.
