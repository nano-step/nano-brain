## ADDED Requirements

### Requirement: BM25 and vector search queries SHALL accept optional document-level time-range bounds

The 8 sqlc search queries (`BM25Search`, `BM25SearchAll`, `BM25SearchWithTags`, `BM25SearchAllWithTags`, `VectorSearch`, `VectorSearchAll`, `VectorSearchWithTags`, `VectorSearchAllWithTags`) SHALL each accept four optional named `pgtype.Timestamptz` parameters: `@created_after`, `@created_before`, `@updated_after`, `@updated_before`. When any parameter is non-NULL, the query SHALL restrict results to chunks whose parent document's corresponding timestamp column satisfies the bound (`>=` for `_after`, `<=` for `_before`). When all four parameters are NULL, the query SHALL produce a plan and result set identical to the pre-change query for the same other inputs.

The filter SHALL be applied via the `(@param::timestamptz IS NULL OR d.<column> <op> @param)` idiom so the planner can short-circuit omitted predicates at planning time.

#### Scenario: Omit-all path — plan unchanged

- **GIVEN** a workspace with N chunks indexed
- **WHEN** `BM25Search` is invoked with all four time-range parameters as `pgtype.Timestamptz{Valid: false}`
- **THEN** EXPLAIN ANALYZE SHALL show identical scan-node selection and index usage compared to the same query on master pre-change
- **AND** median latency over 1000 invocations SHALL be within 10% of master pre-change

#### Scenario: updated_after with valid timestamp restricts result set

- **GIVEN** a workspace where 3 documents have `updated_at` = `now - 5d`, `now - 20d`, `now - 60d`
- **WHEN** `BM25Search` is invoked with `updated_after = now - 30d` and other filters NULL
- **THEN** only chunks of the 5-day-old and 20-day-old documents SHALL appear in results
- **AND** the 60-day-old document's chunks SHALL NOT appear

#### Scenario: All four filters combined apply AND semantics

- **GIVEN** a workspace with documents at various `created_at` and `updated_at` values
- **WHEN** `VectorSearch` is invoked with non-NULL `created_after`, `created_before`, `updated_after`, AND `updated_before`
- **THEN** only chunks satisfying ALL four bounds (AND combined) SHALL appear in results

#### Scenario: Index usage preserved on filtered path

- **GIVEN** a workspace with ≥10,000 chunks
- **WHEN** `BM25SearchWithTags` is invoked with `updated_after` set and a tag filter
- **THEN** EXPLAIN ANALYZE SHALL show neither the chunks table nor the documents table is sequentially scanned
- **AND** the planner SHALL use the chunks tsvector GIN index (or equivalent BM25 path) as a driving scan, with the documents join providing the time filter

### Requirement: Cursor query-hash SHALL include ALL filter inputs (query, tags, scope, collections, time-range)

The cursor query-hash function in `internal/search/cursor.go` (introduced by PR #359) SHALL incorporate ALL filter inputs into its hash, not just the query text. The hash input SHALL include:

1. The query text
2. Tags (deterministic ordering — sorted, then joined with delimiter)
3. Scope (string)
4. Collections (deterministic ordering — sorted, then joined with delimiter)
5. The four time-range filter values as **raw input strings** (`"30d"`, `"2026-05-04T00:00:00Z"`, or literal `"null"` when omitted)

The four time-range filters SHALL be hashed as raw input strings (not parsed absolute times) so that cursors using relative durations like `"30d"` remain stable across paginated calls (the parsed absolute time shifts by seconds between calls; the raw string does not).

This requirement subsumes a fix to a pre-existing correctness gap: pre-this-change, `QueryHash()` hashed only the query string, so tag/scope/collection changes between paginated calls silently returned wrong results.

#### Scenario: Time-range filter change invalidates cursor

- **GIVEN** a `Query` call with `updated_after = "30d"` returned a cursor `C1` for page 2
- **WHEN** a follow-up `Query` call is made with the same query text but `updated_after = "7d"` AND cursor `C1`
- **THEN** the pipeline SHALL return a "cursor invalidated" error (using the same error type as #359 query-text-change invalidation)

#### Scenario: Tag change invalidates cursor (pre-existing bug fix)

- **GIVEN** a `Query` call with `tags = ["bug"]` returned a cursor `C1` for page 2
- **WHEN** a follow-up `Query` call is made with the same query text but `tags = ["feature"]` AND cursor `C1`
- **THEN** the pipeline SHALL return a "cursor invalidated" error
- **AND** this scenario SHALL NOT have passed on master pre-change — it explicitly verifies the fix

#### Scenario: Scope change invalidates cursor (pre-existing bug fix)

- **GIVEN** a `Query` call with `scope = "all"` returned a cursor `C1` for page 2
- **WHEN** a follow-up `Query` call is made with the same query text but `scope = "memory"` AND cursor `C1`
- **THEN** the pipeline SHALL return a "cursor invalidated" error

#### Scenario: Unchanged filters preserve cursor validity

- **GIVEN** a `Query` call with `updated_after = "30d"` and `tags = ["bug"]` returned cursor `C1` for page 2
- **WHEN** a follow-up call uses the same query text, same `updated_after = "30d"`, same `tags = ["bug"]`, and cursor `C1`
- **THEN** the call SHALL return page 2 of the filtered result set
- **AND** no result SHALL appear in both page 1 and page 2
- **AND** no result in the filtered set SHALL be skipped between pages
- **AND** the raw-string hashing approach SHALL prevent micro-drift between page 1 and page 2 absolute time windows from invalidating the cursor
