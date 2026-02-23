## ADDED Requirements

### Requirement: Storage configuration with safe defaults
The `config.yml` SHALL support a `storage` section with `maxSize`, `retention`, and `minFreeDisk` fields. All fields SHALL be optional with safe defaults: `maxSize: 2GB`, `retention: 90d`, `minFreeDisk: 100MB`.

#### Scenario: Config with all storage fields
- **WHEN** config.yml contains `storage: { maxSize: "1GB", retention: "30d", minFreeDisk: "200MB" }`
- **THEN** the server uses those values for eviction and disk safety

#### Scenario: Config with no storage section
- **WHEN** config.yml has no `storage` section
- **THEN** the server uses defaults: maxSize=2GB, retention=90d, minFreeDisk=100MB

#### Scenario: Config with partial storage section
- **WHEN** config.yml contains `storage: { maxSize: "500MB" }`
- **THEN** `maxSize` is 500MB, `retention` defaults to 90d, `minFreeDisk` defaults to 100MB

### Requirement: Human-readable size and duration parsing
The storage config parser SHALL accept human-readable size strings (`500MB`, `2GB`, `1TB`) and duration strings (`30d`, `90d`, `1y`). Invalid values SHALL cause a warning log and fall back to defaults.

#### Scenario: Valid size string
- **WHEN** `maxSize` is set to `"2GB"`
- **THEN** it is parsed as 2,147,483,648 bytes

#### Scenario: Valid duration string
- **WHEN** `retention` is set to `"30d"`
- **THEN** it is parsed as 30 days (2,592,000,000 milliseconds)

#### Scenario: Invalid size string
- **WHEN** `maxSize` is set to `"banana"`
- **THEN** a warning is logged: `[storage] Invalid maxSize "banana", using default 2GB`
- **THEN** the default value of 2GB is used

### Requirement: Retention-based eviction
During each harvest cycle, the system SHALL delete session markdown files older than the `retention` period and remove their corresponding documents from the SQLite database.

#### Scenario: Session older than retention period
- **WHEN** a session file has mtime older than `retention` (e.g., 91 days old with 90d retention)
- **THEN** the session markdown file is deleted from disk
- **THEN** the corresponding document rows are removed from the `documents` table

#### Scenario: Session within retention period
- **WHEN** a session file has mtime within the `retention` period (e.g., 30 days old with 90d retention)
- **THEN** the session file is not deleted
- **THEN** the document rows remain in the database

### Requirement: Size-based eviction
After retention eviction, if total storage (SQLite DB + sessions directory) still exceeds `maxSize`, the system SHALL delete the oldest remaining session files until total size is under the limit.

#### Scenario: Storage exceeds maxSize after retention eviction
- **WHEN** total storage is 2.5GB and `maxSize` is 2GB after retention eviction
- **THEN** the oldest session files are deleted one by one
- **THEN** deletion stops when total size drops below 2GB

#### Scenario: Storage under maxSize
- **WHEN** total storage is 1.5GB and `maxSize` is 2GB
- **THEN** no size-based eviction occurs

### Requirement: Original session JSON is never deleted
Eviction SHALL only remove harvested markdown files and their database entries. The original OpenCode session JSON files in `~/.local/share/opencode/storage/` SHALL never be touched by eviction.

#### Scenario: Session evicted
- **WHEN** a session is evicted due to retention or size limits
- **THEN** only the harvested markdown file in `~/.opencode-memory/sessions/` is deleted
- **THEN** the original JSON in `~/.local/share/opencode/storage/sessions/` remains untouched

### Requirement: Disk safety guard
Before any write operation (harvest, reindex, embed), the system SHALL check available disk space. If free disk space is below `minFreeDisk`, all write operations SHALL be skipped and a warning logged.

#### Scenario: Disk space below minFreeDisk
- **WHEN** available disk space is 50MB and `minFreeDisk` is 100MB
- **THEN** harvest, reindex, and embed operations are skipped
- **THEN** a warning is logged: `[storage] Disk space critically low (<100MB free), skipping writes`

#### Scenario: Disk space above minFreeDisk
- **WHEN** available disk space is 500MB and `minFreeDisk` is 100MB
- **THEN** all write operations proceed normally

#### Scenario: statfs unavailable
- **WHEN** `os.statfs()` is not available (older Node.js or restricted environment)
- **THEN** the disk check is skipped with a warning: `[storage] statfs unavailable, disk safety check disabled`
- **THEN** all other storage limits (maxSize, retention) still function normally

### Requirement: Orphan embedding cleanup
Periodically (every 10 harvest cycles), the system SHALL remove embedding vectors whose corresponding documents no longer exist in the `documents` table.

#### Scenario: Document deleted but embedding remains
- **WHEN** a document is evicted and its row removed from `documents`
- **THEN** on the next orphan cleanup cycle, the corresponding embedding vector is removed
- **THEN** no orphaned embeddings accumulate indefinitely
