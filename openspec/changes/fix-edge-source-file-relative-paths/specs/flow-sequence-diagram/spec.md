## ADDED Requirements

### Requirement: Edge extraction stores workspace-relative source file paths

The file watcher SHALL normalize file paths to workspace-relative before passing them to edge extractors. The `source_file` column in `graph_edges` SHALL contain workspace-relative paths (e.g., `server/controllers/trade.js`), NOT absolute paths (e.g., `/Users/tamlh/projects/tradeit-backend/server/controllers/trade.js`).

#### Scenario: Edge extraction produces relative paths
- **WHEN** the watcher processes a file at absolute path `/workspace/server/trade.js` for workspace `/workspace/`
- **THEN** the `source_file` column in `graph_edges` SHALL contain `server/trade.js`

#### Scenario: Nested file extraction
- **WHEN** the watcher processes a file at absolute path `/workspace/src/api/v1/handler.ts` for workspace `/workspace/`
- **THEN** the `source_file` column SHALL contain `src/api/v1/handler.ts`

### Requirement: deriveServiceName handles absolute paths defensively

The `deriveServiceName` function SHALL strip leading `/` and absolute path prefixes before extracting the service name from `source_file`. This is defense-in-depth — the watcher fix is the primary guarantee.

#### Scenario: Absolute path with nested service directory
- **WHEN** `deriveServiceName` receives edges with `source_file` = `/Users/tamlh/projects/tradeit-backend/server/trade.js`
- **THEN** it SHALL return `tradeit-backend`

#### Scenario: Relative path (normal case)
- **WHEN** `deriveServiceName` receives edges with `source_file` = `tradeit-backend/server/trade.js`
- **THEN** it SHALL return `tradeit-backend`

#### Scenario: Empty source file
- **WHEN** `deriveServiceName` receives edges with empty `source_file`
- **THEN** it SHALL return `"Backend"` (the existing fallback)
