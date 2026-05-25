## 1. Store Layer — removeWorkspace method

- [x] 1.1 Add `removeWorkspace(projectHash: string)` method to the store in `src/store.ts` that deletes from all workspace-scoped tables in a single transaction: flow_steps (via cascade), execution_flows, symbol_edges, code_symbols, symbols, file_edges, then delegates to existing clearWorkspace logic for documents/embeddings/content/cache. Return a summary object with per-table deletion counts.
**Spec**: workspace-scoping/spec.md — removeWorkspace deletes from all tables, removeWorkspace is atomic
**Files**: `src/store.ts`, `src/types.ts`

- [x] 1.2 Add `RemoveWorkspaceResult` interface to `src/types.ts` with fields for each table's deletion count (documentsDeleted, embeddingsDeleted, contentDeleted, cacheDeleted, fileEdgesDeleted, symbolsDeleted, codeSymbolsDeleted, symbolEdgesDeleted, executionFlowsDeleted)
**Spec**: workspace-scoping/spec.md — removeWorkspace returns summary object
**Files**: `src/types.ts`

## 2. Workspace Resolution

- [x] 2.1 Add `resolveWorkspaceIdentifier(identifier: string, config: CollectionConfig, store: Store)` function in `src/index.ts` that resolves a workspace identifier (absolute path, hex hash prefix, or basename) to a `{ projectHash, workspacePath }` object. If ambiguous (multiple matches), throw with details of all matches. If not found, throw with error message.
**Spec**: workspace-remove/spec.md — CLI rm command accepts workspace identifier (all scenarios)
**Files**: `src/index.ts`

## 3. CLI Handler

- [x] 3.1 Add `handleRm(globalOpts: GlobalOptions, commandArgs: string[])` function in `src/index.ts` that parses `--dry-run` and `--list` flags, resolves the workspace identifier, calls `store.removeWorkspace()`, removes the config entry via `loadCollectionConfig`/`saveCollectionConfig`, and prints a summary of what was deleted.
**Spec**: workspace-remove/spec.md — Complete data removal, Config cleanup, Dry-run mode, List known workspaces
**Files**: `src/index.ts`, `src/collections.ts`

- [x] 3.2 Wire `handleRm` into the `main()` command dispatch in `src/index.ts` for the `rm` command. Add `rm` to the `showHelp()` output.
**Spec**: workspace-remove/spec.md — CLI rm command accepts workspace identifier
**Files**: `src/index.ts`

## 4. Config Cleanup

- [x] 4.1 Add `removeWorkspaceConfig(configPath: string, workspaceRoot: string)` function in `src/collections.ts` that removes the workspace entry from `config.workspaces` and saves. No-op if workspace not in config.
**Spec**: workspace-remove/spec.md — Config cleanup after removal (both scenarios)
**Files**: `src/collections.ts`

## 5. Tests

- [x] 5.1 Add tests for `store.removeWorkspace()` in `test/store.test.ts`: verify all tables are cleaned, shared content preserved, orphaned content deleted, transaction atomicity
**Spec**: workspace-scoping/spec.md — all scenarios
**Files**: `test/store.test.ts`

- [x] 5.2 Add tests for workspace identifier resolution: path resolution, hash prefix matching, name matching, ambiguous name error, not-found error
**Spec**: workspace-remove/spec.md — CLI rm command accepts workspace identifier (all scenarios)
**Files**: `test/cli.test.ts`

- [x] 5.3 Add tests for `removeWorkspaceConfig`: config entry removed, no-op when not in config
**Spec**: workspace-remove/spec.md — Config cleanup after removal
**Files**: `test/collections.test.ts`
