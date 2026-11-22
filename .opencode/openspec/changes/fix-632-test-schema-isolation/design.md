# Design

Keep the hashed test-name prefix for diagnosis and append a random UUID suffix
for per-invocation uniqueness. A PostgreSQL identifier stays below 63 bytes.
`AfterConnect` sets `search_path` to that exact schema; migrations and cleanup
operate only on it. Before goose runs, create a schema-local `goose_db_version`
initialized at version 0. Without it PostgreSQL finds the existing
`public.goose_db_version` via `search_path`, goose reports "no migrations to
run", and fixtures silently write to public tables. No change to the
production database or schema.

Tests check uniqueness even when names coincide, identifier length, and that
one test invocation cannot see another's fixtures.
