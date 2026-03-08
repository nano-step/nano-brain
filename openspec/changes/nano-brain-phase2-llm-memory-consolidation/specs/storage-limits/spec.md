## MODIFIED Requirements

### Requirement: Extracted facts count toward storage limits
Extracted facts SHALL be counted as documents and contribute to storage quotas. The storage limit enforcement SHALL apply equally to manually written memories and automatically extracted facts.

#### Scenario: Storage limit includes extracted facts
- **WHEN** `memory_status` reports storage usage
- **THEN** the document count includes extracted facts
- **AND** the storage size includes extracted fact content

#### Scenario: Storage limit reached with extracted facts
- **WHEN** storage limit is reached
- **AND** new facts are being extracted during harvest
- **THEN** extraction stops when limit is reached
- **AND** a warning is logged indicating storage limit reached
- **AND** session markdown is still indexed (facts are optional)

#### Scenario: Retention policy applies to extracted facts
- **WHEN** retention policy evicts old documents
- **THEN** extracted facts are eligible for eviction based on age
- **AND** facts follow the same retention rules as other documents

## ADDED Requirements

### Requirement: Extracted facts storage reporting
The `memory_status` tool SHALL report extracted fact statistics separately from other document types.

#### Scenario: memory_status shows fact count
- **WHEN** `memory_status` is called
- **AND** extracted facts exist in the database
- **THEN** the response includes `extractedFacts: { count: N, storageBytes: M }`

#### Scenario: memory_status with no extracted facts
- **WHEN** `memory_status` is called
- **AND** no extracted facts exist
- **THEN** the response includes `extractedFacts: { count: 0, storageBytes: 0 }`
