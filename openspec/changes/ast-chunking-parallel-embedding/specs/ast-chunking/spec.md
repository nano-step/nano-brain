## ADDED Requirements

### Requirement: AST-aware chunk boundaries
The system SHALL use tree-sitter AST parsing to identify chunk boundaries at function, class, method, and interface declarations for supported languages (TypeScript, JavaScript, Python).

#### Scenario: Function extracted as single chunk
- **WHEN** a source file contains a function smaller than MAX_BLOCK_CHARS (3600)
- **THEN** the entire function body SHALL be extracted as a single chunk

#### Scenario: Class with methods chunked by method
- **WHEN** a source file contains a class larger than MAX_BLOCK_CHARS
- **THEN** each method SHALL be extracted as a separate chunk

### Requirement: Chunk size limits
The system SHALL enforce MIN_BLOCK_CHARS=50 and MAX_BLOCK_CHARS=3600 with 15% tolerance (up to 4140 chars) for AST chunks.

#### Scenario: Small node skipped
- **WHEN** an AST node contains fewer than MIN_BLOCK_CHARS (50)
- **THEN** the node SHALL be merged with adjacent content or skipped

#### Scenario: Large node with children recursed
- **WHEN** an AST node exceeds MAX_BLOCK_CHARS and has child nodes
- **THEN** the system SHALL recurse into child nodes for chunking

#### Scenario: Large leaf node falls back to line-based
- **WHEN** an AST node exceeds MAX_BLOCK_CHARS and has no suitable children
- **THEN** the system SHALL fall back to line-based chunking within that node

### Requirement: Fallback to regex chunking
The system SHALL fall back to existing regex-based chunking for unsupported languages or when tree-sitter parsing fails.

#### Scenario: Unsupported language uses regex
- **WHEN** a source file is in a language without tree-sitter support (e.g., Ruby, Go)
- **THEN** the system SHALL use existing `chunkSourceCode()` regex-based chunking

#### Scenario: Parse failure falls back gracefully
- **WHEN** tree-sitter parsing fails for a supported language file
- **THEN** the system SHALL fall back to regex chunking and log a warning

### Requirement: Chunk metadata header
Each chunk SHALL include a metadata header with file path, language, and line range (matching existing nano-brain format).

#### Scenario: Chunk includes metadata
- **WHEN** a chunk is extracted from lines 10-50 of `src/utils.ts`
- **THEN** the chunk SHALL be prefixed with metadata: `// File: src/utils.ts | Language: typescript | Lines: 10-50`

### Requirement: Deduplication via segment hash
The system SHALL compute a SHA-256 hash of (file path + start line + end line + content size) to deduplicate chunks.

#### Scenario: Identical chunks deduplicated
- **WHEN** the same file region is processed twice
- **THEN** only one chunk SHALL be stored (based on segment hash)
