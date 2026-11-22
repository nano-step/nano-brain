# Isolate integration-test database schemas (#632)

## Why

`SetupTestDB` hashes only `t.Name()`. Repeated tests and identically named tests
in different packages can share and concurrently drop the same schema. This
turns a full integration run into a nondeterministic test of concurrent fixture
mutation rather than application behavior.

## What

Assign a fresh PostgreSQL schema to each invocation, keep migration and
cleanup scoped to it, and verify same-name invocations are distinct.
