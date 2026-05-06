## MODIFIED Requirements

### Requirement: RRF fusion applies length normalization
After computing RRF scores, the hybrid search pipeline SHALL apply a log-based length normalization penalty: `score *= 1 / (1 + Math.log2(Math.max(1, charLength / lengthNormAnchor)))` where `lengthNormAnchor` defaults to `2000` characters. This MUST be applied before recency boost.

#### Scenario: Large document score is reduced
- **WHEN** a 20000-char document and a 2000-char document have equal RRF scores
- **THEN** the 20000-char document's final score is approximately 70% of the 2000-char document

#### Scenario: Short documents are not penalized
- **WHEN** a document is shorter than the length norm anchor
- **THEN** the penalty factor is less than 1 and the document is not boosted above its RRF score
