# nano-brain Setup Guide for AI Agents

This guide is written for AI agents (Claude Code, OpenCode, Cursor, etc.) to walk a user through installing and configuring nano-brain. Follow each step in order. Check before installing — always ask the user before making changes to their system.

---

## Quick setup (one command)

nano-brain ships a single interactive wizard that walks a fresh machine from "binary installed" to "MCP tools working" — no manual Docker command, no separate `serve` step, no `init --root` afterthought. This is the primary path for a local machine with Docker available. If you're setting up a VPS / shared team server, a machine without Docker, or Windows, skip to [Manual setup / troubleshooting](#manual-setup--troubleshooting) below.

### 1. Prerequisites check

```bash
node --version   # 18+ required — nano-brain installs via npm
docker info 2>&1 | head -5   # recommended — the wizard can auto-provision PostgreSQL via Docker
ollama --version 2>&1 | head -1   # optional — only needed for local embeddings
```

- **Node.js 18+** is required to install nano-brain via npm.
- **Docker** is recommended: the wizard detects a reachable PostgreSQL, or offers to start one via Docker automatically. If there's no Docker, the wizard also accepts a remote PostgreSQL URL (see the Manual setup appendix for VPS-style setups).
- **Ollama** is optional: only needed if the user wants local embeddings. The wizard also accepts any Ollama-compatible URL or a Voyage AI API key, and embeddings can be skipped entirely for BM25-only (keyword) search.

### 2. Install

```bash
npm install -g @nano-step/nano-brain
```

### 3. Initialize

```bash
nano-brain init
```

Run with no arguments, in an interactive terminal. This single command:

1. Gates on an existing config — **keep** (default) resumes/skips straight to starting the server, **overwrite** re-runs setup. This makes `nano-brain init` safely re-runnable.
2. Detects a reachable PostgreSQL, or offers to provision one via Docker (or accepts a remote connection URL).
3. Optionally enables embeddings — local Ollama, any Ollama-compatible URL, Voyage AI, or skip for BM25-only search.
4. Writes the config file and runs `doctor` to verify the stack.
5. Starts the server.
6. Offers to register the current directory as a workspace.
7. Configures the MCP client(s) the user selects (Claude Code, OpenCode, Codex CLI). Claude Code and OpenCode are written **project-local** (`.mcp.json` / `opencode.json` in the project root), each bound to this project's `?workspace=`. Codex CLI asks a scope: **global** (`~/.codex/config.toml`, shared by every project) writes a bare URL with **no** `?workspace=` — so registering another project can't override this one's binding, and the agent passes a workspace per call; **project** (`<project>/.codex/config.toml`) writes a workspace-bound URL but requires trusting the directory in Codex.
8. Prints a summary — server URL, workspace name/hash — ending with: **restart your AI client** to pick up the new MCP configuration.

The happy path is a handful of prompts (at most six consequential questions: overwrite/keep, database, embeddings, start server, register, per-MCP-client) — not the ~20-question flow of older versions. For per-step manual control instead of the wizard, see the appendix below.

> **Windows note:** the wizard's automatic server auto-start is not available yet on Windows. `nano-brain init` will print a manual `nano-brain serve` instruction instead of starting the server itself — see [Step 7 — Start the server](#step-7--start-the-server) in the manual appendix.

### 4. Verify

After restarting your AI client, confirm the MCP tools respond:

```
memory_status → should return {"pg_status":"healthy", ...}
```

Then run a quick end-to-end check:

```bash
# Write a test memory
nano-brain write --workspace=<hash> "nano-brain setup complete on $(date)"

# Query it back
nano-brain query --workspace=<hash> "setup complete"
```

If results come back, setup is successful.

---

## Manual setup / troubleshooting

This per-step manual path stays valid and is the recommended route for a **VPS / shared-team server**, a machine **without Docker** (bring your own PostgreSQL), **Windows**, or when you want step-by-step control instead of the wizard.

Ask the user which setup path they want:

1. **Local** — single machine, Ollama for embeddings, Docker for PostgreSQL
2. **VPS / team** — nano-brain on a remote server, shared DB, team access
3. **Voyage AI** — use Voyage AI instead of Ollama (no local GPU needed)

The steps below cover **Path 1 (local)**. For VPS and Voyage AI, adapt the embedding and server binding steps accordingly.

---

> **Node.js is only needed for the npm install path (Step 5).** If you use the recommended `curl | bash` installer, skip this step.

### Step 1 — Check Node.js

```bash
node --version
```

- **Found (18+):** proceed
- **Found (< 18):** tell the user: "Node.js 18 or higher is required. Your version is X. Would you like me to upgrade it?"
- **Not found:** ask: "Node.js is required to install nano-brain via npm. Would you like me to install it?" → if yes, direct to https://nodejs.org or use a version manager (`nvm`, `fnm`)

---

### Step 2 — Check Docker

```bash
docker --version
docker info 2>&1 | head -5
```

- **Running:** proceed
- **Installed but not running:** ask: "Docker is installed but not running. Would you like me to start it?" → `open -a Docker` (macOS) or `sudo systemctl start docker` (Linux)
- **Not installed:** ask: "Docker is needed to run PostgreSQL locally. Would you like help installing it?" → direct to https://docs.docker.com/get-docker/

---

### Step 3 — Check Ollama

```bash
ollama --version
curl -s http://localhost:11434/api/tags 2>&1 | head -3
```

- **Running with models:** check if `nomic-embed-text` is available:
  ```bash
  ollama list | grep nomic-embed-text
  ```
  If not present, ask: "The `nomic-embed-text` embedding model is not pulled yet. Would you like me to pull it now? (~274 MB)" → `ollama pull nomic-embed-text`
- **Installed but not running:** ask: "Ollama is installed but not running. Would you like me to start it?" → `ollama serve &`
- **Not installed:** ask: "Ollama is needed for local embeddings. Would you like help installing it?" → direct to https://ollama.com/download

> **Alternative:** If the user has a Voyage AI API key, they can skip Ollama entirely. Set `VOYAGE_API_KEY` and use `provider: voyage` in config.

---

### Step 4 — Start PostgreSQL

Check if a PostgreSQL instance is already running:

```bash
docker ps | grep nanobrain-pg
pg_isready -h localhost -p 5432 2>/dev/null
```

- **Already running on port 5432:** ask: "A PostgreSQL instance is already running. Should I use it for nano-brain, or start a separate one on a different port?"
- **Not running:** ask: "No PostgreSQL instance found. Would you like me to start one using Docker?" → if yes:

```bash
docker run -d --name nanobrain-pg \
  --restart unless-stopped \
  -p 5432:5432 \
  -e POSTGRES_USER=nanobrain \
  -e POSTGRES_PASSWORD=nanobrain \
  -e POSTGRES_DB=nanobrain_dev \
  pgvector/pgvector:pg17
```

Wait for it to be ready:
```bash
until docker exec nanobrain-pg pg_isready -U nanobrain 2>/dev/null; do sleep 1; done
echo "PostgreSQL is ready"
```

---

### Step 5 — Install nano-brain

- **Already installed:** check version: `nano-brain version`
- **Not installed:** ask which method the user prefers:

**Recommended — one-line installer (no Node.js required):** downloads the prebuilt binary from GitHub Releases and verifies its SHA-256.

```bash
curl -fsSL https://raw.githubusercontent.com/nano-step/nano-brain/master/install.sh | bash
nano-brain version
```

Honors `NANO_BRAIN_VERSION=<tag>` to pin a release and `NANO_BRAIN_BIN_DIR=<dir>` to choose the install directory.

**Alternative — npm** (requires Node.js 18+, see Step 1):

```bash
npm install -g @nano-step/nano-brain
nano-brain version
```

---

### Step 6 — Run doctor

This verifies the full stack in one command:

```bash
nano-brain doctor
```

Expected output: all checks green. Common failures and fixes:

| Failure | Fix |
|---|---|
| `PostgreSQL: FAIL` | Check Docker container is running: `docker ps \| grep nanobrain-pg` |
| `pgvector extension: FAIL` | The `pgvector/pgvector:pg17` image includes pgvector — re-run migrations: `nano-brain db:migrate` |
| `Ollama: FAIL` | Ollama not running: `ollama serve &` |
| `Embedding model: FAIL` | Model not pulled: `ollama pull nomic-embed-text` |
| `Config: WARN` | Config file not found — nano-brain will use defaults, which is fine for local setup |

If any check fails, fix it before continuing.

---

### Step 7 — Start the server

```bash
# Check if already running
curl -s http://localhost:3100/health 2>/dev/null | grep -q "ok" && echo "already running"
```

- **Already running:** skip
- **Not running:** ask: "Would you like me to start nano-brain in the background?" → if yes:

```bash
nano-brain serve -d
```

Verify it started:
```bash
curl -s http://localhost:3100/health
# Expected: {"status":"ok", ...}
```

> **Windows:** `nano-brain init`'s automatic server auto-start is not available yet — run this step manually after `init` prints the instruction to do so.

---

### Step 8 — Register the workspace

Ask the user: "Which project directory would you like to register with nano-brain? (press Enter for current directory)"

```bash
# Register the project
nano-brain init --root=/path/to/project

# Confirm registration
nano-brain workspaces list
```

The output includes both the workspace **name** and **hash** — either can be used to scope queries to this project, including the `?workspace=` connection default in Step 9.

---

### Step 9 — Configure your MCP client

Ask the user which AI client they use:

#### Claude Code
Add to `~/.claude.json` under `mcpServers`:
```json
{
  "mcpServers": {
    "nano-brain": {
      "type": "http",
      "url": "http://localhost:3100/mcp"
    }
  }
}
```

#### OpenCode
Add to OpenCode config:
```json
{
  "mcp": {
    "nano-brain": {
      "type": "remote",
      "url": "http://localhost:3100/mcp",
      "enabled": true
    }
  }
}
```

#### Other MCP clients
Use `url: http://localhost:3100/mcp` with transport type `http` (MCP 2025-03-26 streamable HTTP) for Claude Code and generic streamable-HTTP clients. Note: OpenCode's own config schema names this transport `"type": "remote"` (not `"http"`).

#### Binding a default workspace (optional)

Append `?workspace=<name-or-hash>` to the MCP URL to bind a default workspace to the connection. Run `nano-brain workspaces list` (Step 8) to see the registered name and hash for this project, e.g.:
```json
{
  "url": "http://localhost:3100/mcp?workspace=nano-brain"
}
```
With this set, `memory_*` tool calls can omit the `workspace` argument — the connection default is used automatically. An explicit `workspace` argument on a tool call always overrides the connection default. The value must be a workspace name or full hash (not `"all"`) — a connection is meant to be pinned to one project. This is optional; the plain URL without a query string still works exactly as before.

After adding the config, restart your AI client and verify the tools are available:
```
memory_status → should return {"pg_status":"healthy", ...}
```

---

### Step 10 — Verify end-to-end

Run a quick test to confirm everything works:

```bash
# Write a test memory
nano-brain write --workspace=<hash> "nano-brain setup complete on $(date)"

# Query it back
nano-brain query --workspace=<hash> "setup complete"
```

If results come back, setup is successful.

The full REST API surface is discoverable at `GET /api/openapi.json` (OpenAPI 3.0 spec, regenerated via `make generate-openapi`) — useful if you're building non-MCP tooling against nano-brain.

---

## Troubleshooting

### Server won't start
```bash
nano-brain doctor --json    # machine-readable output
nano-brain logs -n 50       # last 50 log lines
```

### Embedding queue stuck
```bash
nano-brain status           # shows queue depth and provider status
```
If queue is growing but not shrinking, Ollama may be overloaded. Check: `curl http://localhost:11434/api/tags`

### MCP tools not showing in AI client
1. Confirm server is running: `curl http://localhost:3100/health`
2. Confirm MCP endpoint responds: `curl -X POST http://localhost:3100/mcp -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","method":"tools/list","id":1}'`
3. Restart your AI client after updating config

### Port conflict (3100 already in use)
```bash
# Use a different port
nano-brain serve -d --port=3200
```
Update your MCP config URL accordingly.

---

## VPS / team setup (Path 2)

If the user wants a shared server:

1. Complete Steps 1–8 on the server (set `--host=0.0.0.0` in Step 7)
2. Generate auth tokens: `nano-brain auth token` → one per team member or role
3. Enable auth in config:
   ```yaml
   server:
     host: 0.0.0.0
     port: 3100
     auth:
       enabled: true
       tokens:
         - "nbt_..."
   ```
4. Each developer adds the remote URL + token to their MCP client (see Step 9, replace `localhost` with server IP)
5. Each developer registers their local project: `NANO_BRAIN_SERVER=http://SERVER_IP:3100 nano-brain init --root=.`
