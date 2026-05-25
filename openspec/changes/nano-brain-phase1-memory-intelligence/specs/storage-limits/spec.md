## ADDED Requirements

### Requirement: Access-aware retention eviction

When performing size-based eviction (storage exceeds maxSize), the system SHALL optionally consider access_count when selecting documents to evict. Documents with lower access_count SHALL be evicted before documents with higher access_count, within the same age tier. This behavior SHALL be enabled when `decay.enabled` is true in config.

#### Scenario: Two documents same age with different access counts

- **WHEN** two documents have the same age, one with `access_count` of 10 and one with `access_count` of 0
- **THEN** the document with `access_count` of 0 is evicted first
- **THEN** the document with `access_count` of 10 is retained

#### Scenario: Decay disabled uses age-only eviction

- **WHEN** `decay.enabled` is false in config
- **THEN** eviction uses age-only ordering (current behavior)
- **THEN** `access_count` is not considered in eviction decisions

#### Scenario: All documents have zero access count

- **WHEN** all documents have `access_count` of 0
- **THEN** the system falls back to age-based eviction
- **THEN** the oldest documents are evicted first
