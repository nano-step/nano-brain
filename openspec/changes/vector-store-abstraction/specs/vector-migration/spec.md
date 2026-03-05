# Vector Migration Specification

## Purpose

Zero-cost migration tool that exports existing vectors from SQLite databases into Qdrant without re-embedding, preserving all 49K vectors and their metadata.

## ADDED Requirements

### Requirement: `qdrant migrate` exports SQLite vectors to Qdrant

The `qdrant migrate` command SHALL read vectors from all workspace SQLite databases and batch-upsert them into Qdrant.

#### Scenario: Migrate all workspaces
- **WHEN** `npx nano-brain qdrant migrate` is run with 4 healthy SQLite databases
- **THEN** for each database, sqlite-vec extension is loaded
- **THEN** vectors are read via: SELECT from vectors_vec JOIN content_vectors
- **THEN** each vector is upserted into Qdrant with payload: hash, seq, pos, model, projectHash, collection
- **THEN** progress is printed per workspace: "[zengamingx] 21714/21714 vectors migrated"
- **THEN** summary is printed: "Total: 48797 vectors migrated in 32s"

#### Scenario: Migrate specific workspace
- **WHEN** `npx nano-brain qdrant migrate --workspace=/Users/tamlh/workspaces/NUSTechnology/Projects/zengamingx` is run
- **THEN** only the zengamingx database is migrated
- **THEN** other workspace databases are skipped

#### Scenario: Batch upsert into Qdrant
- **WHEN** a workspace has 21714 vectors to migrate
- **THEN** vectors are uploaded in batches of 500 (default)
- **THEN** 44 batch requests are made (43 × 500 + 1 × 214)

#### Scenario: Dry run shows counts without writing
- **WHEN** `npx nano-brain qdrant migrate --dry-run` is run
- **THEN** each workspace's vector count is printed
- **THEN** no vectors are written to Qdrant
- **THEN** output shows: "[dry-run] Would migrate 48797 vectors from 4 workspaces"

#### Scenario: Qdrant not running
- **WHEN** `npx nano-brain qdrant migrate` is run and Qdrant is not reachable
- **THEN** the command prints "Qdrant is not running. Start with: npx nano-brain qdrant up" and exits with code 1

#### Scenario: Collection created automatically
- **WHEN** migration starts and the Qdrant collection does not exist
- **THEN** collection "nano-brain" is created with 1024 dimensions and cosine distance
- **THEN** payload indexes are created for "hash" and "collection" fields

#### Scenario: Skip corrupted databases
- **WHEN** a workspace database has corrupted vector tables
- **THEN** the error is logged: "[SudoX] Skipped: database disk image is malformed"
- **THEN** migration continues with remaining databases

### Requirement: Migration preserves vector identity

Migrated vectors SHALL have the same point ID format and metadata as newly-embedded vectors would.

#### Scenario: Point ID matches hash:seq format
- **WHEN** a vector with hash_seq "abc123:2" is migrated from SQLite
- **THEN** the Qdrant point ID is derived from "abc123:2"
- **THEN** searching for this vector in Qdrant returns the same result as SQLite would

#### Scenario: Metadata payload is complete
- **WHEN** a vector is migrated from SQLite
- **THEN** the Qdrant payload contains: hash, seq, pos, model, projectHash, collection
- **THEN** these fields match the values from content_vectors and documents tables
