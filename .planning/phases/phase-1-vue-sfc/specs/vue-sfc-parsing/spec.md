## ADDED Requirements

### Requirement: Vue SFC block splitting

The system SHALL parse `.vue` files using `grammars.VueLanguage()` and extract `template_element`, `script_element`, and `style_element` nodes.

#### Scenario: Parse .vue file with script setup

- **WHEN** a `.vue` file contains `<script setup lang="ts">`
- **THEN** the parser SHALL extract the script block as a `script_element` node with `lang="ts"` attribute

#### Scenario: Parse .vue file with options API

- **WHEN** a `.vue` file contains `<script>` (no setup)
- **THEN** the parser SHALL extract the script block as a `script_element` node with default lang (js)

#### Scenario: Parse .vue file with dual script blocks

- **WHEN** a `.vue` file contains BOTH `<script setup>` AND `<script>` (Options API)
- **THEN** the parser SHALL extract BOTH script blocks as separate `script_element` nodes

#### Scenario: Parse .vue file with no script

- **WHEN** a `.vue` file contains only `<template>` and `<style>` (no script)
- **THEN** the parser SHALL extract template and style elements, and return empty script list

#### Scenario: Parse .vue file with malformed script

- **WHEN** a `.vue` file contains a `<script>` block with syntax errors
- **THEN** the parser SHALL return `Status: "parse_error"` for that script block without crashing

### Requirement: Script block re-parsing

The system SHALL re-parse extracted `raw_text` content from `script_element` nodes using the appropriate grammar based on the `lang` attribute.

#### Scenario: Re-parse TypeScript script

- **WHEN** a script block has `lang="ts"` or `lang="typescript"`
- **THEN** the system SHALL re-parse the content using `grammars.TypescriptLanguage()`

#### Scenario: Re-parse JavaScript script

- **WHEN** a script block has no `lang` attribute or `lang="js"` or `lang="javascript"`
- **THEN** the system SHALL re-parse the content using `grammars.JavascriptLanguage()`

#### Scenario: Re-parse with unsupported lang

- **WHEN** a script block has an unsupported `lang` attribute (e.g., `lang="coffee"`)
- **THEN** the system SHALL skip script extraction and emit `parse_error` status

### Requirement: Edge extraction from script

The system SHALL extract `contains`, `imports`, and `calls` edges from re-parsed script content.

#### Scenario: Extract import edges

- **WHEN** a script block contains `import { ref } from 'vue'`
- **THEN** the system SHALL create an `imports` edge from the `.vue` file to `'vue'`

#### Scenario: Extract call edges

- **WHEN** a script block contains `const data = await useFetch('/api/data')`
- **THEN** the system SHALL create a `calls` edge from the `.vue` file to `useFetch`

#### Scenario: Extract contains edges

- **WHEN** a script block defines a function or variable
- **THEN** the system SHALL create `contains` edges from the `.vue` file to those symbols

### Requirement: Line number offset handling

The system SHALL correctly handle line numbers when script blocks start after line 1.

#### Scenario: Script starts at line 1

- **WHEN** a `.vue` file has `<script>` as the first element
- **THEN** line numbers in the re-parsed script SHALL start at line 1

#### Scenario: Script starts after template

- **WHEN** a `.vue` file has `<template>` before `<script>` (script starts at line 400+)
- **THEN** line numbers in the re-parsed script SHALL be correctly offset using `RootNodeWithOffset`
