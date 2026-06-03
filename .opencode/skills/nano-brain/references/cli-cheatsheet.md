---
name: nano-brain CLI cheatsheet
description: npx nano-brain ... subcommand reference. Use when writing shell scripts or one-off command invocations; for in-agent calls prefer the MCP tools in the parent SKILL.md.
---

# nano-brain CLI cheatsheet

Every CLI subcommand is a thin HTTP client over the daemon at `http://localhost:3100` (override via `NANO_BRAIN_HOST` / `NANO_BRAIN_PORT`). Container agents auto-route to `host.docker.internal`.

Common flags: `--workspace <hash>` (required for workspace-scoped ops), `--json` (machine-readable output).

## Search / recall

| Command | Flags | Notes |
|---|---|---|
| `npx nano-brain query "..."` | `--workspace`, `--scope=workspace\|all`, `--tags=a,b`, `--json` | Hybrid (BM25 + vector + RRF); default for most questions |
| `npx nano-brain search "..."` | same | BM25 only; fastest |
| `npx nano-brain vsearch "..."` | same | Vector only; semantic fuzzy match |
| `npx nano-brain wake-up` | `--workspace`, `--limit=10`, `--json` | Session-start briefing |

There is no `--compact` flag on search commands — use `--json` and pipe to `jq` for projection.

## Write / fetch

| Command | Flags | Notes |
|---|---|---|
| `npx nano-brain write "<content>"` | `--workspace`, `--tags=a,b`, `--collection=memory`, `--json` | Upsert by content hash; new doc OR replaces same source_path |
| `npx nano-brain get <id-or-path>` | `--workspace`, `--json` | Fetch full doc by `#<uuid>` or source_path |
| `npx nano-brain multi-get <id...>` | `--workspace`, `--json` | Batch fetch; returns `{results, not_found}` |
| `npx nano-brain tags` | `--workspace`, `--json` | List tags with counts |

`--collection` is only on `write` (sets new doc's collection). For READ filtering by collection, use HTTP: `GET /api/v1/documents?workspace=<hash>&collection=<name>`.

## Workspace lifecycle

| Command | Flags | Notes |
|---|---|---|
| `npx nano-brain init` | `--root=<path>` (required), `--workspace=<hash>` (optional override), `--json`, `--force` | Register a workspace; idempotent |
| `npx nano-brain workspaces` | (no flags) | List registered workspaces (`workspaces list` is alias) |
| `npx nano-brain workspaces list` | `--json` | Same as above |
| `npx nano-brain workspaces remove <hash>` | (path arg) | Full delete: docs + chunks + embeddings + graph + registration |

## Code intelligence

| Command | Flags | Notes |
|---|---|---|
| `npx nano-brain context <symbol>` | `--workspace`, `--json` | 1-hop callers + callees + imports |
| `npx nano-brain code-impact <symbol>` | `--workspace`, `--depth=2`, `--json` | Reverse-impact BFS (what breaks if symbol changes) |
| `npx nano-brain detect-changes` | `--workspace`, `--staged` (bool, default true), `--all` (bool) | Maps git diff to affected symbols |

**`--staged` is a boolean flag, NOT a `--scope=staged` string.** Pass `--all` instead to scan staged + unstaged + untracked.

## Reindex / harvest / embed

| Command | Flags | Notes |
|---|---|---|
| `npx nano-brain reindex` | `--root=<path>` (required at client), `--workspace=<hash>` | Re-scan disk → DB → embed queue |
| `npx nano-brain harvest` | (none) | Force-run all configured harvesters (OpenCode + Claude Code) |
| `npx nano-brain reset-embeddings` | `--workspace` | Re-queue all chunks for embedding |
| `npx nano-brain backfill-summaries` | `--dry-run`, `--output-dir=<path>`, `--workspace=<hash>`, `--since=<RFC3339-date>` | Export existing summary docs from DB to `.md` files on disk |

**Reindex `--root` gotcha**: the client requires the flag (validates non-empty), but the daemon's `/api/v1/reindex` handler scopes by the body's `workspace` field, not `root`. Always pass `--workspace` explicitly OR run via `workdir`:

```
# ✅ Both work
npx nano-brain reindex --root=/path --workspace=<hash>
bash(command="npx nano-brain reindex", workdir="/path/to/workspace")  # client picks $PWD
```

## Ops / debug

| Command | Flags | Notes |
|---|---|---|
| `npx nano-brain status` | `--json` | Daemon health, queue depth, harvester status |
| `npx nano-brain serve` | `--config=<path>`, `--unsafe-no-auth`, `--serve-only`, `-v` (verbose) | Start daemon. Add `-d` for background. |
| `npx nano-brain serve --serve-only` | — | Disables embed queue + file watcher + harvester + summarizer (read-only HTTP/MCP frontend; #282) |
| `npx nano-brain doctor` | — | Probes daemon + PG + ollama + model availability |
| `npx nano-brain cleanup-stale-raw` | `--dry-run` | Remove pre-PR-#192 orphan raw session docs |
| `npx nano-brain cleanup-orphan-workspaces` | `--dry-run` | Delete workspaces with 0 docs and >30d unused |
| `npx nano-brain bench <generate\|run\|compare\|stress>` | per subcommand | Search-quality + write-stress benchmarks |
| `npx nano-brain db:migrate` | — | Apply pending PostgreSQL migrations |
| `npx nano-brain auth ...` | (subcommands) | Manage HTTP auth users/tokens |
| `npx nano-brain version` | — | Print version |
| `npx nano-brain help` | — | Print top-level help |

## Diagnostics

| Command | Flags | Notes |
|---|---|---|
| `nano-brain version --which` | `--json` | Print resolved binary path, version, and invocation source (`npm-local`, `npm-global`, `dev-build`, `path`, `env-override`) |
| `nano-brain mcp-url` | (none) | Print resolved MCP URL — precedence: `NANO_BRAIN_MCP_URL` env → `/.dockerenv` detection → `localhost:3100` |
| `nano-brain doctor` | `--json` | Offline prerequisite checks (config, PostgreSQL, pgvector, Ollama, embedding model, binary exists) |
| `nano-brain doctor --online` | `--json` | All offline checks + runtime: server reachable, embed queue health (WARN ≥ 80%, FAIL ≥ 95%), CLI↔server version skew, MCP endpoint reachable |

**Quick health check workflow:**
```bash
nano-brain version --which    # confirm which binary is running
nano-brain mcp-url            # confirm MCP URL for your environment
nano-brain doctor --online    # full health report
```

**CI-friendly (JSON + exit code):**
```bash
nano-brain doctor --online --json | jq '.all_passed'
# exit 0 if all pass, exit 1 if any fail
```

**Override binary:**
```bash
NANO_BRAIN_BIN=/custom/nano-brain nano-brain version --which
# NANO_BRAIN_BIN is validated: file must exist and be executable
```

## Recipes (CLI form)

### Session start
```
WS=$(npx nano-brain init --root="$PWD" --json | jq -r .workspace_hash)
npx nano-brain wake-up --workspace="$WS" --limit=8
npx nano-brain query "<task topic>" --workspace="$WS" --json
```

### Recall before grep
```
npx nano-brain search "ECONNREFUSED redis" --workspace="$WS"
npx nano-brain vsearch "rate limiting" --workspace="$WS"
```

### Pre-refactor impact
```
npx nano-brain code-impact DatabaseClient --workspace="$WS" --depth=2 --json
npx nano-brain detect-changes --workspace="$WS" --staged
```

### Persist a decision
```
npx nano-brain write "## Decision: …" \
  --workspace="$WS" \
  --tags=decision,architecture
```

### Triage many results
```
# Scan
npx nano-brain query "auth" --workspace="$WS" --json \
  | jq '.results[] | {id, title, score}'

# Expand
npx nano-brain get "<doc-id>" --workspace="$WS"
```

### Filter by tag (multi-tag = AND)
```
npx nano-brain query "..." --workspace="$WS" --tags=bug,auth
```

### Filter by collection (HTTP only)
```
curl -G http://localhost:3100/api/v1/documents \
  --data-urlencode "workspace=$WS" \
  --data-urlencode "collection=session-summary"
```
