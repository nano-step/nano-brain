## ADDED Requirements

### Requirement: Preserve embed_status on unchanged chunk upsert

The system SHALL preserve the existing `embed_status` value when upserting a chunk with unchanged content. The `embed_status` SHALL only be reset to `'pending'` when the chunk content actually changes.

#### Scenario: Unchanged chunk content preserves embed_status

- **WHEN** a chunk with `embed_status = 'embedded'` is upserted with the same content
- **THEN** the `embed_status` SHALL remain `'embedded'`

#### Scenario: Changed chunk content resets embed_status

- **WHEN** a chunk with `embed_status = 'embedded'` is upserted with different content
- **THEN** the `embed_status` SHALL be reset to `'pending'`

#### Scenario: Failed chunk with unchanged content preserves status

- **WHEN** a chunk with `embed_status = 'embed_failed'` is upserted with the same content
- **THEN** the `embed_status` SHALL remain `'embed_failed'`

#### Scenario: New chunk gets pending status

- **WHEN** a new chunk is inserted (no conflict)
- **THEN** the `embed_status` SHALL be `'pending'`
