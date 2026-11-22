# Integration Test Isolation

## ADDED Requirements

### Requirement: Unique schema per test invocation
Every call to `SetupTestDB` SHALL use its own schema in `nanobrain_test`,
including calls with the same test name in different packages or processes.

#### Scenario: Repeated test name
Given two calls to create a schema for the same test name,
when both are executed,
then their schema names differ and are valid PostgreSQL identifiers.

### Requirement: Local cleanup
Cleaning up one test MUST NOT drop tables or rows belonging to another test.

#### Scenario: Two tests finish in different orders
Given two active tests with the same test name and separate schemas,
when the first test finishes and drops its schema,
then the second test's data remains available.

### Requirement: Local migration state
Each test schema MUST own its migration version and application tables. It
MUST NOT fall through to tables in the public schema when the public database
already contains a migrated application.

#### Scenario: Public schema is already migrated
Given a public `goose_db_version` at the latest version,
when a fresh test schema is provisioned,
then migrations are applied to that test schema from version zero.
