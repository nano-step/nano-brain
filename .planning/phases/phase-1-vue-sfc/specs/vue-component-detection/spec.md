## ADDED Requirements

### Requirement: Component detection via AST

The system SHALL detect child component references in Vue templates using AST `tag_name` nodes, filtered by PascalCase naming convention.

#### Scenario: Detect PascalCase component in template

- **WHEN** a `.vue` template contains `<MyChild />` or `<MyChild prop="value">`
- **THEN** the system SHALL create a `component_usage` edge from the parent `.vue` file to `MyChild`

#### Scenario: Detect PascalCase component with closing tag

- **WHEN** a `.vue` template contains `<MyChild>...</MyChild>`
- **THEN** the system SHALL create a `component_usage` edge from the parent `.vue` file to `MyChild`

#### Scenario: Ignore lowercase HTML elements

- **WHEN** a `.vue` template contains `<div>`, `<span>`, `<p>`, or other lowercase elements
- **THEN** the system SHALL NOT create `component_usage` edges for these elements

#### Scenario: Ignore comments

- **WHEN** a `.vue` template contains `<!-- <MyChild /> -->` (comment)
- **THEN** the system SHALL NOT create `component_usage` edges for commented components

#### Scenario: Ignore PascalCase in text content

- **WHEN** a `.vue` template contains text like "Use MyChild for this"
- **THEN** the system SHALL NOT create `component_usage` edges for text references

### Requirement: Component edge deduplication

The system SHALL deduplicate component usage edges when the same child component is referenced multiple times.

#### Scenario: Multiple references to same component

- **WHEN** a `.vue` template references `<MyChild>` in 3 different places
- **THEN** the system SHALL create only ONE `component_usage` edge to `MyChild`

#### Scenario: Component referenced in script and template

- **WHEN** a `.vue` file imports `MyChild` in script AND uses it in template
- **THEN** the system SHALL create both `imports` and `component_usage` edges (no dedup across edge types)
