## MODIFIED Requirements

### Requirement: Multi-word queries use per-term OR logic
The `sanitizeFTS5Query` function SHALL split multi-word input into individual quoted terms joined by `OR`, instead of wrapping the entire input as one exact phrase. Single-word queries SHALL remain wrapped as a single quoted term.

#### Scenario: Single word query unchanged
- **WHEN** `sanitizeFTS5Query("memory")` is called
- **THEN** the result is `"memory"` (single quoted term, same as current behavior)

#### Scenario: Multi-word query split into OR terms
- **WHEN** `sanitizeFTS5Query("instant sell price")` is called
- **THEN** the result is `"instant" OR "sell" OR "price"`
- **THEN** FTS5 returns documents matching ANY of the individual terms

#### Scenario: Code identifier in natural language query
- **WHEN** `sanitizeFTS5Query("instantSellPriceAdjustPercent config")` is called
- **THEN** the result is `"instantSellPriceAdjustPercent" OR "config"`
- **THEN** FTS5 matches documents containing either term

#### Scenario: Hyphenated words preserved as single terms
- **WHEN** `sanitizeFTS5Query("nano-brain search")` is called
- **THEN** the result is `"nano-brain" OR "search"`

#### Scenario: FTS5 operators neutralized
- **WHEN** `sanitizeFTS5Query("AND OR NOT")` is called
- **THEN** the result is `"AND" OR "OR" OR "NOT"` (operators treated as literal terms inside quotes)

#### Scenario: FTS5 column prefixes neutralized
- **WHEN** `sanitizeFTS5Query("filepath: test")` is called
- **THEN** the result is `"filepath:" OR "test"` (colon-suffixed word treated as literal)

#### Scenario: Internal double quotes escaped
- **WHEN** `sanitizeFTS5Query('hello "world" test')` is called
- **THEN** each term with internal quotes has them escaped: `"hello" OR """world""" OR "test"`

#### Scenario: Empty input returns empty string
- **WHEN** `sanitizeFTS5Query("")` is called
- **THEN** the result is `""`

#### Scenario: Whitespace-only input returns empty string
- **WHEN** `sanitizeFTS5Query("   ")` is called
- **THEN** the result is `""`

#### Scenario: Extra whitespace between words collapsed
- **WHEN** `sanitizeFTS5Query("  hello   world  ")` is called
- **THEN** the result is `"hello" OR "world"` (leading/trailing/extra whitespace ignored)

### Requirement: Existing single-term behavior preserved
Single-term exact-match queries SHALL produce identical results before and after the change. The `store.searchFTS()` method SHALL continue to use `sanitizeFTS5Query` as its sole query sanitizer.

#### Scenario: Single term search returns same results
- **WHEN** `store.searchFTS("memory")` is called
- **THEN** the results are identical to the pre-change behavior (single quoted term match)

#### Scenario: searchFTS still uses sanitizeFTS5Query
- **WHEN** `store.searchFTS(query)` is called with any query string
- **THEN** the query passes through `sanitizeFTS5Query()` before being used in the FTS5 MATCH clause
