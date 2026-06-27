## ADDED Requirements

### Requirement: Graph node identity is workspace-relative end-to-end

Graph node identities SHALL use the **workspace-relative** form across stored `source_node`, stored import `target_node`, and the query canonicalizer (`resolveNodeAgainstWorkspace`), so the literal SQL match in reverse/forward lookups intersects. The canonicalizer SHALL NOT convert nodes to absolute paths.

#### Scenario: Query canonicalizer returns relative
- **WHEN** `resolveNodeAgainstWorkspace` is called with `utils/enums.js` for a workspace rooted at `/abs/proj`
- **THEN** it SHALL return `utils/enums.js` (workspace-relative), NOT `/abs/proj/utils/enums.js`

#### Scenario: Absolute input is stripped to relative
- **WHEN** the canonicalizer receives `/abs/proj/utils/enums.js` for workspace root `/abs/proj`
- **THEN** it SHALL return `utils/enums.js`

#### Scenario: Non-path token passes through
- **WHEN** the canonicalizer receives `context` (an extensionless/non-path token)
- **THEN** it SHALL return `context` unchanged

### Requirement: Import edge targets are resolved to workspace-relative files

Import (`imports`) graph edges SHALL store a resolved, workspace-relative
`target_node` that byte-matches the stored `source_node` form of the same file, so
reverse traversal (`memory_impact`, `memory_graph direction=in`) returns real
dependents. Resolution SHALL run at the single edge-upsert convergence point.
Bare and scoped npm package specifiers SHALL pass through unchanged. On any
ambiguity or resolution miss the raw specifier SHALL be retained (never a guessed
or extensionless path).

#### Scenario: Alias import resolves to the workspace-relative file
- **WHEN** a source file imports `~/utils/enums` and the alias map maps `~` to the workspace root
- **THEN** the edge `target_node` SHALL be `utils/enums.ts` (the resolved workspace-relative path), not the raw `~/utils/enums`

#### Scenario: Reverse lookup returns dependents via the relative path
- **WHEN** N files import a module that resolves to `utils/enums.ts`
- **AND** `memory_graph(node="utils/enums.ts", direction=in, edge_type=imports)` is queried
- **THEN** the result SHALL contain all N importing files (not 0)

#### Scenario: Relative import resolves against the source directory
- **WHEN** `src/a/b.ts` imports `../c/d` and `src/c/d.ts` exists
- **THEN** the edge `target_node` SHALL be `src/c/d.ts`

#### Scenario: Scoped npm package is preserved (not treated as alias)
- **WHEN** a source file imports `@org/pkg`
- **THEN** the edge `target_node` SHALL remain `@org/pkg` (the `@/` alias rule SHALL NOT match `@org/`)

#### Scenario: Bare package specifier is preserved
- **WHEN** a source file imports `ramda` or `lodash/fp`
- **THEN** the edge `target_node` SHALL remain unchanged (no filesystem resolution attempted)

#### Scenario: Unresolvable or ambiguous specifier falls back to raw
- **WHEN** an alias/relative specifier cannot be resolved to an existing file by extension/index probing
- **THEN** the edge `target_node` SHALL retain the raw specifier (NOT an extensionless half-path) and a warning SHALL be logged (no edge dropped)
