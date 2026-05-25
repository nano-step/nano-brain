# Design: Workspaces List CLI + Status Count Fix

## Architecture

Two independent changes shipping together because they share the same discoverability gap:

```
┌─ CLI side (new) ──────────────────┐  ┌─ Server side (fix) ────────────┐
│                                   │  │                                │
│  cmd/nano-brain/workspaces.go     │  │  internal/storage/queries/     │
│    └─ runWorkspacesCmd            │  │    └─ workspaces.sql           │
│         └─ list / ls subcommand   │  │         + CountWorkspaces      │
│              │                    │  │                                │
│              ↓                    │  │  internal/server/handlers/     │
│         GET /api/v1/workspaces ───┼──┼→  health.go                    │
│              │                    │  │    └─ Health.{Health,Status}   │
│              ↓                    │  │         → real count via       │
│         table OR --json           │  │           WorkspaceQuerier     │
│                                   │  │                                │
│  cmd/nano-brain/main.go           │  │  internal/server/routes.go     │
│    └─ case "workspaces"           │  │    └─ wire querier into NewHealth │
└───────────────────────────────────┘  └────────────────────────────────┘
```

Both halves are additive — no existing CLI command or REST endpoint changes shape.

## CLI design

### Command surface

```
nano-brain workspaces            # alias for `workspaces list`
nano-brain workspaces list       # full form
nano-brain workspaces ls         # short alias
nano-brain workspaces --json     # JSON output (no subcommand needed)
nano-brain workspaces list --json
```

`runWorkspacesCmd(args)` mirrors `runCollectionCmd`: it inspects `args[0]`, dispatches to `runWorkspacesList`. If `args[0]` starts with `--` (or is empty), default to `list`.

### Output formats

**Default (table)** — writes to stdout:

```
HASH         NAME         PATH                                   DOCS  LAST UPDATE
7f44356179.. nano-brain   /Users/me/projects/nano-brain             0  never
8a9c1d4f12.. my-app       /Users/me/projects/my-app                42  2026-05-25
```

Column widths:
- HASH: first 10 chars + `..` (12 chars total)
- NAME: up to 30 chars, truncate with `..`
- PATH: up to 50 chars, truncate from the LEFT with `..` (preserve filename — `..nano-brain/cmd/main.go` style)
- DOCS: right-aligned, raw int
- LAST UPDATE: `YYYY-MM-DD` or `never` if `last_document_updated == null`

No tabs/spaces gymnastics — use `text/tabwriter` from stdlib.

**`--json`** — passthrough the API response body verbatim to stdout. Caller can `jq` it. No wrapping, no transformation.

### Empty result

If the response is `[]`:
- Default mode: print `No workspaces registered.` to stderr + exit 0.
- `--json`: print `[]` to stdout + exit 0.

Why stderr for the human "empty" message: it's a status note, not data. Scripts piping `--json` should never see noise on stdout, and the default mode is for humans who use stderr-aware terminals.

### Error handling

Reuses existing `doRequest` (which now has connect-error UX from proposal #1). When server unreachable, the user gets the same smart error + auto-start prompt for free.

For other HTTP errors (5xx, 4xx) — print server message to stderr, exit 1.

## Server-side design

### New sqlc query

Add to `internal/storage/queries/workspaces.sql`:

```sql
-- name: CountWorkspaces :one
SELECT COUNT(*) FROM workspaces;
```

This generates `CountWorkspaces(ctx) (int64, error)` via sqlc.

### Health handler wiring

`Health` struct gains a `workspaceCounter` field. Two implementation choices:

**Option A: Add to existing `WorkspaceQuerier` interface**
- Pro: one interface for all workspace operations.
- Con: `Health` doesn't need the other methods on `WorkspaceQuerier`. Bloats the interface for `Health`'s sake.

**Option B: New minimal interface `WorkspaceCounter`**
```go
type WorkspaceCounter interface {
    CountWorkspaces(ctx context.Context) (int64, error)
}
```
- Pro: interface segregation — `Health` only depends on what it uses.
- Con: yet another interface in the package.

**Decision: Option B.** Keeps the change small, matches the existing `PoolChecker` / `EmbedQueueInfo` pattern already used by `Health`.

### Constructor change

```go
// Before
func NewHealth(pool PoolChecker, logger zerolog.Logger, version string,
    startTime time.Time, queue EmbedQueueInfo,
    getCfg func() (config.HarvesterConfig, config.IntervalsConfig)) *Health

// After
func NewHealth(pool PoolChecker, logger zerolog.Logger, version string,
    startTime time.Time, queue EmbedQueueInfo,
    getCfg func() (config.HarvesterConfig, config.IntervalsConfig),
    counter WorkspaceCounter) *Health
```

Update call site: `internal/server/routes.go:14`. The querier is already available via `s.queries` (or however the server holds its sqlc Queries — to be confirmed during implementation).

### Behavior in handlers

```go
func (h *Health) Status(c echo.Context) error {
    ...
    n, err := h.counter.CountWorkspaces(c.Request().Context())
    if err != nil {
        h.logger.Warn().Err(err).Msg("failed to count workspaces; reporting 0")
        n = 0
    }
    resp := statusResponse{
        ...
        WorkspaceCount: int(n),
        ...
    }
    ...
}
```

Same treatment for `Health.Health()`. **Soft-fail on DB error** — never break the health endpoint over a count query. Log a warning, report 0.

## Key Decisions

### 1. Why `CountWorkspaces` not reuse `ListWorkspacesWithStats`

- `Status` is called from health probes, monitoring, dashboards — potentially every few seconds.
- `ListWorkspacesWithStats` does two correlated subqueries per row (doc count + last update). Even with 10 workspaces, that's 21 queries.
- `SELECT COUNT(*)` is a single integer round-trip.
- The cost of one extra sqlc-generated function is trivial.

### 2. Why no CountWorkspaces caching

Premature. PostgreSQL counts on small tables are fast. Revisit if `/api/status` shows up in profiles.

### 3. Why `workspaces` plural

- Matches REST endpoint (`/api/v1/workspaces`, plural).
- Future-proofs for `workspaces show <hash>`, `workspaces remove <hash>` etc.
- Trade-off: `nano-brain workspace status` reads slightly nicer in English but mismatches the API.

### 4. Table truncation from left for paths

Long paths like `/Users/tamlh/workspaces/self/AI/Tools/nano-brain` have the meaningful part at the right (the project name). Right-truncation would show `/Users/tamlh/works...` which is useless.

### 5. `--json` is passthrough, not re-encoded

- Caller can rely on the exact JSON shape the server returns.
- No risk of CLI drifting from API.
- If the API adds a field, `--json` users see it automatically.

## Files Changed

| File | Change |
|---|---|
| `cmd/nano-brain/workspaces.go` | NEW — subcommand + table render + JSON passthrough |
| `cmd/nano-brain/workspaces_test.go` | NEW — table format, --json passthrough, empty result, error paths |
| `cmd/nano-brain/main.go` | Add `case "workspaces"` to dispatch + help text |
| `internal/storage/queries/workspaces.sql` | Add `CountWorkspaces` query |
| `internal/storage/sqlc/workspaces.sql.go` | sqlc-generated (regenerate) |
| `internal/storage/sqlc/querier.go` | sqlc-generated (regenerate, adds method to Querier interface) |
| `internal/server/handlers/health.go` | New `WorkspaceCounter` iface, `Health` field, real query in both handlers |
| `internal/server/handlers/health_test.go` | Update mocks to satisfy new interface |
| `internal/server/routes.go` | Pass querier to `NewHealth` |

## Test Plan

### CLI tests (`workspaces_test.go`)

1. `TestWorkspacesList_DefaultTable` — `httptest` returns 2 workspaces → assert table headers + 2 rows + correct truncation
2. `TestWorkspacesList_Json` — `--json` flag → stdout matches body verbatim, no trailing whitespace beyond final newline
3. `TestWorkspacesList_Empty_Default` — `httptest` returns `[]` → stderr `No workspaces registered.`, stdout empty, exit 0
4. `TestWorkspacesList_Empty_Json` — `httptest` returns `[]` + `--json` → stdout `[]`, stderr empty
5. `TestWorkspacesList_ServerError` — `httptest` returns 500 → exit non-zero, error message on stderr
6. `TestWorkspacesList_LongPathTruncation` — path longer than column width → starts with `..`
7. `TestWorkspacesList_NeverColumn` — `last_document_updated == null` → column shows `never`
8. `TestWorkspacesAliasLs` — `runWorkspacesCmd([]string{"ls"})` behaves the same as `["list"]`
9. `TestWorkspacesNoArgsDefaultsToList` — `runWorkspacesCmd([]string{})` runs list
10. `TestWorkspacesFlagOnlyDefaultsToList` — `runWorkspacesCmd([]string{"--json"})` runs list with json

### Server tests (`health_test.go`)

1. `TestStatusReturnsRealWorkspaceCount` — mock counter returns 3 → response `workspace_count: 3`
2. `TestHealthReturnsRealWorkspaceCount` — same as above for `/health`
3. `TestStatusSoftFailsOnCountError` — mock counter returns error → response `workspace_count: 0`, no 5xx
4. `TestHealthSoftFailsOnCountError` — same for `/health`

### Integration smoke (manual, documented in PR)

```
$ nano-brain workspaces
HASH         NAME         PATH                                  DOCS  LAST UPDATE
...

$ nano-brain workspaces --json | jq '.[0].workspace_hash'
"7f44356179..."

$ curl -s localhost:3100/api/status | jq '.workspace_count'
1   # not 0 anymore
```

## Out-of-scope (Future Work)

- `workspaces show <hash>` — detail view with collections, docs, embeddings status
- `workspaces remove <hash>` — needs backend DELETE handler first
- `workspaces rename` — needs backend support
- Sort / filter flags (`--sort name|docs|recent`, `--name <pattern>`) — defer until requested
- Workspace list in `nano-brain config show` — explicitly rejected (config vs runtime separation)
