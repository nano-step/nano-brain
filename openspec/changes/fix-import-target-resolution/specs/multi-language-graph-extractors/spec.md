## ADDED Requirements

### Requirement: Import edge targets are resolved to canonical node identities

Import (`imports`) graph edges SHALL store a resolved, canonical `target_node`
that matches the node identity emitted by `resolveNodeAgainstWorkspace` for the
same file, so that reverse traversal (`memory_impact`, `memory_graph` with
`direction=in`) returns the real dependents. Resolution SHALL run at index time
at the single edge-upsert convergence point. Bare npm package specifiers SHALL be
stored unchanged. On any resolution failure the raw specifier SHALL be retained
(graceful degradation).

#### Scenario: Alias import resolves to the canonical file path
- **WHEN** a source file imports `~/utils/enums` and `~` maps to the workspace root
- **THEN** the edge `target_node` SHALL be the canonical workspace-relative path of the resolved file (e.g. `utils/enums.js`), not the raw specifier `~/utils/enums`

#### Scenario: Reverse lookup returns dependents via the real path
- **WHEN** N files import a module that resolves to `utils/enums.js`
- **AND** `memory_graph(node="utils/enums.js", direction=in, edge_type=imports)` is queried
- **THEN** the result SHALL contain all N importing files (not 0)

#### Scenario: Relative import resolves against the source directory
- **WHEN** `src/a/b.js` imports `../c/d`
- **THEN** the edge `target_node` SHALL resolve to the canonical path `src/c/d.js` (with extension probing)

#### Scenario: Bare package specifier is preserved
- **WHEN** a source file imports `ramda` or `@babel/core`
- **THEN** the edge `target_node` SHALL remain `ramda` / `@babel/core` (no filesystem resolution attempted)

#### Scenario: Unresolvable specifier falls back to raw
- **WHEN** an alias/relative specifier cannot be resolved to a known file
- **THEN** the edge `target_node` SHALL retain the raw specifier and a warning SHALL be logged (no edge is dropped)
