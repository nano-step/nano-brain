## MODIFIED Requirements

### Requirement: Code intelligence pipeline support for .vue files

The system SHALL support `.vue` files in the code intelligence pipeline, including edge extraction, symbol extraction, and impact analysis.

#### Scenario: Extract edges from .vue files

- **WHEN** a `.vue` file is indexed
- **THEN** the system SHALL extract `contains`, `imports`, `calls`, and `component_usage` edges

#### Scenario: Include .vue in impact analysis

- **WHEN** `memory_impact` is called on a `.vue` file
- **THEN** the system SHALL return all dependent files including other `.vue` files that import or use it as a component

#### Scenario: Include .vue in call chain tracing

- **WHEN** `memory_trace` is called from a `.vue` file
- **THEN** the system SHALL follow call chains into imported modules and child components

#### Scenario: Include .vue in symbol search

- **WHEN** `memory_symbols` is called with a query matching a Vue component name
- **THEN** the system SHALL return the `.vue` file as a symbol result

### Requirement: Universal .vue extractor wiring

The system SHALL wire the Vue SFC extractor as a universal extractor (no framework detection required).

#### Scenario: Vue extractor runs for all .vue files

- **WHEN** any `.vue` file is encountered during indexing
- **THEN** the Vue SFC extractor SHALL run regardless of whether `vue` is in `package.json`

#### Scenario: Nuxt extractor coexists with Vue extractor

- **WHEN** a `.vue` file in a Nuxt project is indexed
- **THEN** both the Vue SFC extractor and Nuxt extractor SHALL run, with edge dedup logic preventing duplicate edges

#### Scenario: Vue extractor does not produce HTTP edges

- **WHEN** the Vue SFC extractor processes a `.vue` file
- **THEN** it SHALL NOT create `http` edges (only `contains`, `imports`, `calls`, `component_usage`)
