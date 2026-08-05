## Context

The backfill query now selects `sessions` summary documents, while its test
fixture inserts the former `session-summary` collection. Cleanup tests
intentionally stop before the workspace foreign key migration so they can
model orphan records, but generated upserts now need columns introduced later.
The benchmark test performs live HTTP requests to port 3199 despite normal
integration execution not starting that server.
Additional full-suite failures use SQLite schemas without `session.parent_id`
or assert summary formats superseded by the session-unification migration.

## Decisions

### Align fixture with the production selection contract

Change only the backfill fixture collection to `sessions`. The current query
already describes the post-unification production format.

### Align remaining test fixtures with active contracts

Add the required `parent_id` column to the OpenCode SQLite test schemas and
make its stub summary persistence assert the production `sessions` collection.
Update the stale-raw summary fixture and persistence expectations to the same
collection and date-first disk naming. Production readers and writers already
agree on these formats.

### Preserve the pre-FK cleanup fixture

Keep the migration cap at version 10 and use raw INSERT statements limited to
pre-00011 document columns. Moving to migration 29 would add the foreign key
and invalidate the orphan-cleanup scenario.

### Gate live benchmarks explicitly

Use `integration && benchmark` as the build constraint. A missing server in a
requested benchmark remains an error; normal integration runs do not include
it.

## Risks

- Raw cleanup fixture SQL can drift from the pre-FK schema; it is localized to
  the test that explicitly models that historic schema.
- Benchmark CI must opt in with both build tags and provision port 3199.
