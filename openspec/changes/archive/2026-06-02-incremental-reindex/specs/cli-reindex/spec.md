# cli-reindex Delta — Incremental Reindex

## MODIFIED Requirements

### Requirement: reindex CLI command
The CLI SHALL expose a `reindex` command that runs codebase indexing only — scanning files, updating chunks, and rebuilding the tree-sitter symbol graph. It SHALL NOT harvest sessions, index collections, or generate embeddings unnecessarily. By default it SHALL operate **incrementally**: only re-chunk + re-embed files whose `content_hash` has changed since last index. It SHALL accept `--root=<path>` to specify workspace root and `--force-wipe` to fall back to the previous full-wipe behavior.

#### Scenario: Incremental reindex skips unchanged files
- **WHEN** user runs `nano-brain reindex --root=<path>` on a workspace where no files changed since last index
- **THEN** the response counters report `embedded: 0, skipped: N` where N is the number of indexed documents
- **AND** no calls are made to the embedding provider (Ollama/Voyage)

#### Scenario: Incremental reindex re-embeds changed files only
- **WHEN** the user modifies 1 file out of 1000 indexed and runs `nano-brain reindex`
- **THEN** the response reports `embedded: 1, skipped: 999`
- **AND** only the chunks for the modified file are deleted + reinserted
- **AND** chunks for the other 999 files are untouched

#### Scenario: Incremental reindex deletes orphan documents
- **WHEN** a file is removed from disk and the user runs `nano-brain reindex`
- **THEN** the response reports `deleted: 1` for that document
- **AND** the chunks for that document are removed via FK cascade

#### Scenario: --force-wipe restores legacy behavior
- **WHEN** user runs `nano-brain reindex --force-wipe`
- **THEN** ALL chunks + documents for the workspace are deleted before re-indexing
- **AND** the response reports `embedded: N, deleted: M` (matching the previous full-wipe semantics)

#### Scenario: Reindex with explicit root (unchanged behavior)
- **WHEN** user runs `nano-brain reindex --root=/path/to/project`
- **THEN** CLI indexes the specified workspace root

### Requirement: reindex returns incremental counters
The `POST /api/v1/reindex` REST endpoint and the CLI `reindex` command SHALL return counters describing the operation: `scanned` (files seen on disk), `skipped` (unchanged), `embedded` (chunked + embedded), `deleted` (orphan docs cleaned up), and `duration_ms` (wall-clock time).

#### Scenario: Counters reported in JSON response
- **WHEN** the `reindex` REST endpoint completes
- **THEN** the response body contains all five counters as integer fields
- **AND** `scanned == skipped + embedded` (excluding deletes which are independent)

#### Scenario: CLI prints counters in pretty mode
- **WHEN** user runs `nano-brain reindex` without `--json`
- **THEN** the CLI prints a one-line summary: `Reindex: N scanned, N skipped, N embedded, N deleted in NN ms`
