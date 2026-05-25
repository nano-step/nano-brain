## Context

`src/server/bootstrap.ts` already handles daemon mode correctly (lines 85–91): when `config.workspaces` has entries it ignores `--root` and always resolves to the first configured workspace. The bug only affects **non-daemon (stdio) mode**: in that branch, `resolvedWorkspaceRoot = root || process.cwd()` is set unconditionally, and then `createStore(effectiveDbPath)` is called on whatever path results — creating a new DB for any unconfigured path.

Current flow (non-daemon, problematic):
```
--root /some/unconfigured/path
  → resolvedWorkspaceRoot = "/some/unconfigured/path"
  → effectiveDbPath = ~/.nano-brain/data/unconfigured-<hash>.sqlite   ← new DB created
  → createStore(effectiveDbPath)   ← orphan DB born
```

## Goals / Non-Goals

**Goals:**
- Prevent `createStore()` from being called with a path that is not in `config.workspaces` (when workspaces are configured)
- Fall back to the closest matching configured workspace instead, so the server remains functional
- Emit a structured warning log when fallback occurs
- Preserve current behavior when `config.workspaces` is empty/absent

**Non-Goals:**
- Cleaning up existing orphaned databases (separate issue)
- Blocking daemon startup for unconfigured paths (daemon already resolves this correctly)
- Auto-registering new workspaces into config on first use
- CLI commands other than `serve` / MCP entrypoints

## Decisions

### Decision: Guard placement — `bootstrap.ts` non-daemon branch only

The guard should be inserted at the existing `else` branch (lines 88–91) in `bootstrap.ts`, immediately after `resolvedWorkspaceRoot` is set from `root || process.cwd()`. This is the only code path that allows an unconfigured path to reach `createStore()`.

**Alternative considered**: Guard inside `createStore()` itself. Rejected — `createStore()` has no access to config, and mixing config-awareness into the storage layer breaks layering.

### Decision: Fallback strategy — longest-prefix match, then first configured workspace

When `--root` is not in `config.workspaces`, use the configured workspace whose path is the longest prefix of `root`. If no prefix match exists, fall back to the first configured workspace. This preserves correct behavior for nested projects (e.g., `--root /repo/sub` falls back to `/repo` if that is configured).

**Alternative considered**: Hard-fail / exit(1) when root is unconfigured. Rejected — breaks backward compatibility for agents that pass `--root` dynamically without knowing the config.

### Decision: Extract as `resolveConfiguredWorkspace()` helper

The guard logic is ~10 lines and shares structure with the daemon branch (lines 85–91). Extract it into a local helper function in `bootstrap.ts` called `resolveConfiguredWorkspace(root, configuredWorkspaces)`. This avoids duplication and makes both branches use identical resolution logic.

**Alternative considered**: Inline the guard. Acceptable for now but the daemon branch already has the same shape — extraction prevents drift.

### Decision: No changes to `bin/cli.js`

`bin/cli.js` does not call `createStore()` directly — it delegates entirely to `startServer()` in `bootstrap.ts`. No changes needed there.

## Risks / Trade-offs

- **[Risk] Behavior change for existing stdio users who pass `--root` without configuring workspaces** → Mitigation: guard is gated on `configuredWorkspaces.length > 0`; if `config.workspaces` is absent or empty the code path is unchanged.
- **[Risk] Fallback to wrong workspace surprises the user** → Mitigation: warning log clearly states the requested path, the fallback path, and that indexing is disabled for the requested path.
- **[Risk] Longest-prefix match is wrong for unrelated paths** → Mitigation: if no prefix match found, fall back to first configured workspace and log; worst case is using the "wrong" DB, same as today's daemon behavior.

## Migration Plan

No data migration required. Orphaned databases already on disk are not touched. Users who want to clean them up can delete them manually or wait for a future cleanup command (issue tracked separately).

Deployment is a drop-in patch release — no config changes needed, no API surface changes.
