## 1. Extract resolveConfiguredWorkspace helper

- [x] 1.1 Add `resolveConfiguredWorkspace(root: string, configuredWorkspaces: string[]): { resolved: string; fallback: boolean }` function in `src/server/bootstrap.ts`
- [x] 1.2 Implement exact-match check: if `root` is in `configuredWorkspaces`, return `{ resolved: root, fallback: false }`
- [x] 1.3 Implement longest-prefix match: find the configured workspace with the longest path that is a prefix of `root`; return `{ resolved: match, fallback: true }` if found
- [x] 1.4 Implement first-workspace fallback: if no prefix match, return `{ resolved: configuredWorkspaces[0], fallback: true }`
- [x] 1.5 Implement no-op when `configuredWorkspaces` is empty: return `{ resolved: root, fallback: false }`

## 2. Wire guard into non-daemon bootstrap branch

- [x] 2.1 In the `else` branch of `bootstrap.ts` (currently `resolvedWorkspaceRoot = root || process.cwd()`), call `resolveConfiguredWorkspace()` with the configured workspaces list
- [x] 2.2 Assign the `.resolved` value to `resolvedWorkspaceRoot`
- [x] 2.3 When `.fallback` is `true`, emit a `warn`-level log: `"Workspace ${requestedRoot} is not in config.workspaces — falling back to ${resolved}"`
- [x] 2.4 Refactor the daemon branch (lines 85–91) to reuse `resolveConfiguredWorkspace()` instead of duplicating the first-workspace logic

## 3. Tests

- [x] 3.1 Add unit tests for `resolveConfiguredWorkspace()` covering: exact match, longest-prefix match, no-prefix fallback to first, empty list no-op
- [x] 3.2 Add integration test: server started with `--root /unconfigured` where config has workspaces → assert no new DB created for `/unconfigured`, assert fallback DB is opened
- [x] 3.3 Add integration test: server started with `--root /configured` where config has that workspace → assert DB opened for `/configured` (no fallback)
- [x] 3.4 Add integration test: server started with no configured workspaces → assert behavior unchanged regardless of `--root`

## 3b. Fix CLI pre-resolution (found during RRI-T)

- [x] 3b.1 In `src/cli/index.ts`, add `command !== 'mcp'` to the DB pre-resolution exclusion so CLI doesn't bypass bootstrap guard using `process.cwd()`

## 4. Verify and close

- [x] 4.1 Run `npm test` — all tests pass
- [x] 4.2 Manually verify: start server with `--root /tmp/not-in-config` and `config.workspaces` set; confirm warning in logs and no new `.sqlite` file in `~/.nano-brain/data/` for that path
- [x] 4.3 Bump patch version in `package.json`
