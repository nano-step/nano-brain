## ADDED Requirements

### Requirement: Watcher emits per-file processing log
Watcher MUST emit an INF log line `processing file path=<abs_path> collection=<name>` before reading each file during `processDirty`.

#### Scenario: File indexing is visible in logs
- Given the watcher processes a dirty directory
- When each file is read and chunked
- Then an INF log line with path and collection name appears before chunking

### Requirement: Embed queue emits per-chunk embedding logs at INF
Embed queue MUST emit `INF embedding chunk chunk_id=<uuid> file=<source_path>` before calling `embedder.Embed()`, and `INF chunk embedded` on success.

#### Scenario: Embedding progress is visible in logs
- Given the embed queue processes a chunk
- When embedder.Embed() is called
- Then an INF log line `embedding chunk chunk_id=<uuid> file=<path>` appears before the call
- And on success an INF log line `chunk embedded` appears

### Requirement: Embedding failures are fully logged
On embedder error, the queue MUST emit ERR with `chunk_id`, `file`, and full error text. No chunk may be silently dropped.

#### Scenario: Embedding failure is visible
- Given the embedder returns an error
- When processChunk handles the failure
- Then an ERR log line with chunk_id, file path, and error text is emitted
