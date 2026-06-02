<!-- OPENCODE-MEMORY:START -->
<!-- Managed block - do not edit manually. Updated by: npx @nano-step/nano-brain init -->

## Memory System (nano-brain)

This project uses **nano-brain** for persistent context across sessions. Server runs on the host at port 3100; agents inside containers reach it at `host.docker.internal:3100`.

### Bootstrap (once per shell session)

```bash
eval "$(npx @nano-step/nano-brain workspaces current --export)"
```

This exports `NANO_BRAIN_WORKSPACE` so subsequent CLI calls do not need `--workspace=...`. If the workspace is not yet registered:

```bash
npx @nano-step/nano-brain workspaces current --check 2>/dev/null \
  || npx @nano-step/nano-brain init --root="$PWD"
```

### Quick Reference (CLI)

| I want to... | Command |
|---|---|
| Recall past work | `npx @nano-step/nano-brain query "topic"` |
| Find exact term/identifier | `npx @nano-step/nano-brain search "FunctionName"` |
| Semantic search | `npx @nano-step/nano-brain vsearch "concept"` |
| Save a decision | `npx @nano-step/nano-brain write --tags=decision --title="..." --content="..."` |
| Workspace briefing | `npx @nano-step/nano-brain wake-up` |
| Cross-workspace search | `npx @nano-step/nano-brain query --scope=all "topic"` |
| Tag-filtered search | `npx @nano-step/nano-brain query --tags=decision "topic"` |
| Server health | `npx @nano-step/nano-brain status` |

### HTTP API (for non-CLI agents)

Base URL inside containers: `http://host.docker.internal:3100`. On the host: `http://localhost:3100`.

All `POST /api/v1/*` workspace-scoped endpoints require a `workspace` field in the JSON body. Get the hash via:

```bash
curl -fsS -X POST $BASE/api/v1/workspaces/resolve \
  -H 'Content-Type: application/json' \
  -d "{\"path\":\"$PWD\"}" | jq -r .workspace_hash
```

Endpoint contract: `POST /api/v1/workspaces/resolve` body `{"path":"<abs>"}` → `{"workspace_hash","root_path","name","registered"}`. Read-only — never auto-registers; use `POST /api/v1/init` for that.

Example query:

```bash
curl -fsS -X POST $BASE/api/v1/query \
  -H 'Content-Type: application/json' \
  -d "{\"workspace\":\"$NANO_BRAIN_WORKSPACE\",\"query\":\"topic\",\"max_results\":10}"
```

### Session Workflow

- **Start of session:** `npx @nano-step/nano-brain query "what have we done about <task>"` before exploring the codebase.
- **End of session:** Persist key decisions and learnings:

```bash
npx @nano-step/nano-brain write --tags=summary,decision \
  --title="Session: <topic>" \
  --content="## Summary\n- Decision: ...\n- Why: ...\n- Files: ..."
```

### When to Search Memory vs Codebase

- **"Have we done this before?"** → `npx @nano-step/nano-brain query "..."` (past sessions)
- **"Where is this in the code right now?"** → grep / ast-grep
- **"How does this concept work here?"** → both
- **"What calls this function / what breaks if I change Y?"** → `nano-brain` code intelligence (`POST /api/v1/graph/query`, `/api/v1/graph/impact`, `/api/v1/graph/trace`)

See `skills/nano-brain/SKILL.md` for the full reference (phases, error recovery, all endpoints).

<!-- OPENCODE-MEMORY:END -->
