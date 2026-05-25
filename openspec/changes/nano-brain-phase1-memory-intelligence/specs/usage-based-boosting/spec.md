## ADDED Requirements

### Requirement: Usage boost in search scoring

The hybrid search pipeline SHALL apply a usage-based boost to results using the formula `usageBoost = log2(1 + access_count) * decayScore * boostWeight`. The boost SHALL be applied as an additive score adjustment.

#### Scenario: Document with no access history

- **WHEN** a document has `access_count` of 0
- **THEN** the usage boost is 0 (since log2(1) = 0)
- **THEN** the document's score is not affected by usage boosting

#### Scenario: Document with moderate access and recent activity

- **WHEN** a document has `access_count` of 7 and was recently accessed
- **THEN** a moderate usage boost is applied (log2(8) * decayScore * boostWeight)
- **THEN** the document ranks higher than identical documents with lower access counts

#### Scenario: Document with high access but stale

- **WHEN** a document has `access_count` of 100 but has not been accessed recently
- **THEN** the usage boost is reduced by the decay score
- **THEN** the boost is lower than a recently accessed document with the same access count

### Requirement: Boost pipeline position

The usage boost SHALL be applied after centrality boost and before supersede demotion in the search scoring pipeline.

#### Scenario: Result with high centrality and high usage

- **WHEN** a document has both high centrality and high usage scores
- **THEN** both the centrality boost and usage boost are applied
- **THEN** the boosts compound to increase the document's final score

#### Scenario: Superseded document with high usage

- **WHEN** a superseded document has high usage
- **THEN** the usage boost is applied first
- **THEN** the supersede demotion is applied afterward, reducing the final score

### Requirement: Configuration

The SearchConfig SHALL include `usage_boost_weight` (number, default 0.15). Setting to 0 effectively disables usage boosting without disabling access tracking.

#### Scenario: Usage boost disabled

- **WHEN** `usage_boost_weight` is set to 0
- **THEN** no usage boost is applied to search results
- **THEN** access tracking still occurs for all returned documents

#### Scenario: Stronger usage signal

- **WHEN** `usage_boost_weight` is set to 0.3
- **THEN** usage-based boosting has a stronger effect on search rankings

#### Scenario: Negative boost weight

- **WHEN** `usage_boost_weight` is set to a negative value
- **THEN** a warning is logged indicating the invalid value
- **THEN** the default value of 0.15 is used
