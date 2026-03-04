# Per-Chunk Embedding Specification

## Purpose

Embed each document chunk independently instead of one embedding per whole document, improving retrieval granularity and relevance.

## ADDED Requirements

### Requirement: Independent chunk embedding

The `embedPendingCodebase()` function SHALL re-chunk each document body and embed each chunk independently.

#### Scenario: Document split into multiple chunks
- **WHEN** a document with 5000 characters is processed
- **THEN** the document is split into multiple chunks (e.g., 3 chunks of ~1800 chars each)
- **THEN** each chunk is embedded independently via Ollama
- **THEN** each chunk's embedding is stored with a unique `hash:seq` identifier

#### Scenario: Small document creates single chunk
- **WHEN** a document with 500 characters is processed
- **THEN** the document creates a single chunk (seq=0)
- **THEN** one embedding is generated and stored as `hash:0`

### Requirement: Chunk identifier format

Chunk embeddings SHALL be stored in `vectors_vec` with `hash_seq` as primary key, formatted as `hash:seq` where `seq` is the zero-indexed chunk number.

#### Scenario: First chunk identifier
- **WHEN** the first chunk of document with hash `abc123` is embedded
- **THEN** the `hash_seq` value is `abc123:0`
- **THEN** the row is inserted into `vectors_vec` with this primary key

#### Scenario: Multiple chunk identifiers
- **WHEN** a document with hash `def456` is split into 3 chunks
- **THEN** three rows are inserted with `hash_seq` values: `def456:0`, `def456:1`, `def456:2`
- **THEN** each row contains the embedding vector for its respective chunk

### Requirement: Chunk text storage

Each chunk's text SHALL be stored or derivable for snippet extraction during vector search.

#### Scenario: Chunk text stored in content table
- **WHEN** a document is chunked and embedded
- **THEN** the full document body remains in `content.body` column
- **THEN** chunk boundaries are derivable from chunk size and seq index

#### Scenario: Vector search retrieves chunk snippet
- **WHEN** vector search returns a match for `hash:2` (third chunk)
- **THEN** the snippet is extracted from `content.body` using seq offset (2 * chunk_size)
- **THEN** the snippet contains approximately the chunk text that was embedded

### Requirement: Batch embedding of chunks

Chunks SHALL be embedded in batches to Ollama, not one HTTP request per chunk.

#### Scenario: Multiple chunks batched to Ollama
- **WHEN** 10 chunks are ready for embedding
- **THEN** chunks are grouped into batches (e.g., batch size 50)
- **THEN** Ollama `/api/embed` is called with `input` as an array of chunk texts
- **THEN** returned embeddings are matched to chunks by array index

#### Scenario: Batch size respects configuration
- **WHEN** `embedPendingCodebase()` is called with `batchSize=50`
- **THEN** up to 50 chunks are sent to Ollama in a single request
- **THEN** if 120 chunks exist, 3 batch requests are made (50, 50, 20)

### Requirement: Chunk size respects truncation limit

Each chunk's text length SHALL not exceed `OLLAMA_MAX_CHARS` before embedding.

#### Scenario: Chunk truncated to max chars
- **WHEN** a chunk's text is 7000 characters and `OLLAMA_MAX_CHARS=6000`
- **THEN** the chunk is truncated to 6000 characters before embedding
- **THEN** the truncated text is sent to Ollama

#### Scenario: Chunk under max chars not truncated
- **WHEN** a chunk's text is 4000 characters and `OLLAMA_MAX_CHARS=6000`
- **THEN** the full 4000 character chunk is sent to Ollama without truncation

### Requirement: Existing embeddings replaced

When re-embedding a document with per-chunk embeddings, all previous embeddings for that document SHALL be deleted first.

#### Scenario: Old single embedding replaced by chunks
- **WHEN** a document with hash `xyz789` previously had one embedding (no seq suffix)
- **THEN** the old embedding row is deleted from `vectors_vec`
- **THEN** new per-chunk embeddings are inserted as `xyz789:0`, `xyz789:1`, etc.

#### Scenario: Old chunks replaced by new chunks
- **WHEN** a document with hash `abc123` is re-indexed and chunk count changes
- **THEN** all existing rows with `hash_seq` starting with `abc123:` are deleted
- **THEN** new chunk embeddings are inserted with updated seq indices

### Requirement: Vector search returns chunk-level results

Vector search SHALL return individual chunk matches, not whole document matches.

#### Scenario: Multiple chunks from same document match
- **WHEN** vector search finds 2 chunks from document `abc123` relevant
- **THEN** two separate results are returned: one for `abc123:0`, one for `abc123:3`
- **THEN** each result includes the chunk-specific snippet and score

#### Scenario: Chunk snippet extracted correctly
- **WHEN** vector search returns a match for `def456:2`
- **THEN** the snippet is extracted from `content.body` starting at offset (2 * chunk_size)
- **THEN** the snippet length is approximately the chunk size or 700 chars (whichever is smaller)
