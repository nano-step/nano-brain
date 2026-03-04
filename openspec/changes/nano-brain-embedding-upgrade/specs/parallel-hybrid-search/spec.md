# Parallel Hybrid Search Specification

## Purpose

Execute FTS and vector search concurrently across all query variants using `Promise.all` to reduce total search latency.

## ADDED Requirements

### Requirement: Concurrent query variant processing

The `hybridSearch()` function SHALL process all query variants concurrently using `Promise.all`, not sequentially.

#### Scenario: Multiple query variants execute in parallel
- **WHEN** `hybridSearch()` is called with 3 query variants
- **THEN** all 3 variants start FTS and vector search concurrently
- **THEN** the total execution time is approximately equal to the slowest variant, not the sum of all variants

#### Scenario: Single query variant executes immediately
- **WHEN** `hybridSearch()` is called with 1 query variant
- **THEN** FTS and vector search for that variant execute concurrently
- **THEN** no sequential loop overhead is introduced

### Requirement: Parallel FTS and vector search per variant

For each query variant, FTS search and vector search SHALL execute concurrently, not sequentially.

#### Scenario: FTS and vector search run in parallel
- **WHEN** a single query variant is processed
- **THEN** `searchFTS()` and `searchVec()` are called concurrently via `Promise.all`
- **THEN** the variant's total time is the maximum of FTS time and vector time, not the sum

#### Scenario: Query embedding cached during parallel search
- **WHEN** a query variant's embedding is cached
- **THEN** the vector search starts immediately without waiting for Ollama
- **THEN** FTS and vector search still run concurrently

### Requirement: Result aggregation after parallel execution

The `hybridSearch()` function SHALL aggregate results from all query variants after all parallel searches complete.

#### Scenario: All variants complete before aggregation
- **WHEN** `hybridSearch()` processes 3 query variants in parallel
- **THEN** the function waits for all 3 `Promise.all` calls to resolve
- **THEN** results from all variants are combined using RRF before returning

#### Scenario: One variant fails without blocking others
- **WHEN** one query variant's vector search fails (e.g., Ollama timeout)
- **THEN** other variants continue executing
- **THEN** the failed variant contributes empty results to RRF aggregation
- **THEN** the function returns partial results without throwing

### Requirement: Query embedding cache integration

Parallel search SHALL use the query embedding cache to avoid redundant Ollama calls across variants.

#### Scenario: Identical query variants share cached embedding
- **WHEN** two query variants have identical text
- **THEN** the first variant caches the embedding
- **THEN** the second variant retrieves the cached embedding without calling Ollama
- **THEN** both variants' vector searches execute concurrently

#### Scenario: Cache miss triggers concurrent Ollama calls
- **WHEN** multiple unique query variants have no cached embeddings
- **THEN** Ollama `/api/embed` is called concurrently for each variant
- **THEN** each variant's embedding is cached independently
- **THEN** all vector searches proceed in parallel after embeddings return

### Requirement: Backward compatibility with sequential code paths

The parallel search implementation SHALL not break existing search tool interfaces or result formats.

#### Scenario: memory_query returns same result format
- **WHEN** `memory_query` calls `hybridSearch()` with parallel execution
- **THEN** the returned result structure matches the previous sequential implementation
- **THEN** RRF scores, snippets, and metadata are identical for the same query

#### Scenario: memory_search and memory_vsearch unaffected
- **WHEN** `memory_search` (FTS-only) or `memory_vsearch` (vector-only) are called
- **THEN** these tools continue to work without modification
- **THEN** only `memory_query` (hybrid) benefits from parallel execution
