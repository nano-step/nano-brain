---
slug: sessions-not-harvested
status: resolved
trigger: "Claude/OpenCode sessions are not being harvested into nano-brain; agents querying memory get no session context"
created: 2026-06-29
updated: 2026-06-29
---

# Debug: Claude/OpenCode sessions not harvested

## Symptoms

- **Expected:** Coding sessions (Claude Code / OpenCode) for a workspace are harvested into nano-brain's `sessions` collection so agents can recall prior session context via memory_query/memory_search.
- **Actual:** The `sessions` collection is empty. Agents querying memory get no session context.
- **Errors:** None surfaced to the user — silent failure (0 documents).
- **Timeline:** Unknown if ever worked; needs investigation (regression vs never-wired).
- **Reproduction:** `memory_wake_up(workspace="zengamingx")` shows `sessions: {document_count: 0, last_updated: ""}` while `code: 12702` and `memory: 5` are populated.

## Reproduced Evidence

- Workspace: `zengamingx` → hash `d1915ee19311546a064576fc5df565da7ab20fe1c4a81c97e3ba6e9059d977b7`, root `/Users/tamlh/workspaces/NUSTechnology/Projects/zengamingx`, registered=true.
- `memory_status`: pg healthy, provider ollama, queue_pending=0 (not a backlog issue).
- Collections for zengamingx: `code`=12702 ✓, `memory`=5 ✓, **`sessions`=0 ✗** (last_updated empty → never written).
- So: code indexing + manual memory work; only **session harvesting** is broken/absent for this workspace.

## Investigation Leads (for debugger)

1. **Workspace mapping**: Claude Code session JSONL files live under `~/.claude/projects/<encoded-cwd>/`. The encoded cwd is the *repo subdir* actually opened (e.g. `.../zengamingx/tradeit-backend`), not the `zengamingx` parent. Does the harvester map a session's cwd to the parent workspace hash, or to a different (subdir) hash? If subdir, sessions land under a different workspace than the one the agent queries.
2. **Harvester trigger**: Is the session harvester run at all (on init / watcher / schedule)? Is it enabled in config? Check the harvest entrypoint and whether it discovers Claude session files.
3. **Discovery/path**: Does the harvester find Claude session files for these repos? Check the session source dir resolution and any gitignore/path filters.
4. **Write path**: Does harvested content reach the `sessions` collection (vs being dropped/misrouted)?

## Current Focus

hypothesis: CONFIRMED — initClaudeCodeHarvester hashed cfg.SessionDir (the Claude session dir path, e.g. ~/.claude/projects/-Users-...) to compute a workspace hash, then looked that hash up in the DB. But the registered workspace hash is SHA256 of the actual code root (e.g. /Users/tamlh/workspaces/.../zengamingx). These two paths always produce different hashes, so the lookup always returned ErrNoRows and the harvester silently returned (nil, nil) — never starting.
test: Verified hash mismatch: SHA256("~/.claude/projects/-Users-tamlh-...-zengamingx") = 695802... vs registered d1915e...
expecting: Fix applied — awaiting test results and human verification.
next_action: Await full test suite results, then request human verification.

reasoning_checkpoint:
  hypothesis: "initClaudeCodeHarvester hashed cfg.SessionDir (the Claude session dir) instead of
    the workspace root path, causing hash mismatch against registered workspaces → silently disabled"
  confirming_evidence:
    - "SHA256(~/.claude/projects/-Users-...-zengamingx) = 695802cd... ≠ registered d1915ee..."
    - "initClaudeCodeHarvester returned (nil,nil) on every startup due to ErrNoRows"
    - "detectClaudeCodeStorageDir() exists but was never called in main.go (no auto-detection)"
    - "Claude encodes cwd as replace('/','-') — encodes workspace path to Claude dir name unambiguously"
    - "~/.claude/projects/-Users-...-zengamingx/ exists with 104 JSONL files ready to harvest"
  falsification_test: "If fix is correct, after restart sessions > 0 for zengamingx workspace"
  fix_rationale: "New initClaudeCodeHarvesters queries registered workspaces, encodes each path to
    Claude dir name, checks for existence, creates one harvester per match with correct workspace hash"
  blind_spots: "Workspace paths with hyphens in dir components encode identically to path separators —
    but since we go workspace→Claude (not Claude→workspace), encoding is unambiguous"

## Evidence

- timestamp: 2026-06-29 — reproduced sessions=0 for zengamingx via memory_wake_up (code/memory populated, sessions empty).
- timestamp: 2026-06-29 — SHA256("/Users/tamlh/workspaces/NUSTechnology/Projects/zengamingx") = d1915ee... (registered). SHA256("~/.claude/projects/-Users-...-zengamingx") = 695802c... (never registered). Hash mismatch confirmed.
- timestamp: 2026-06-29 — detectClaudeCodeStorageDir() exists in detect.go and correctly returns ~/.claude/projects but was never called in main.go (auto-detection missing).
- timestamp: 2026-06-29 — Claude encoding: replace('/', '-') on workspace path → Claude dir name. Encoding verified: /Users/tamlh/workspaces/NUSTechnology/Projects/zengamingx → -Users-tamlh-workspaces-NUSTechnology-Projects-zengamingx (dir exists with 104 JSONL files).
- timestamp: 2026-06-29 — Fix applied: initClaudeCodeHarvesters (plural) now queries registered workspaces, encodes each path, checks for Claude session dir, creates one harvester per match.
- timestamp: 2026-06-29 — All tests pass (go test -race -short ./..., exit 0).

## Eliminated

- hypothesis: Queue backlog preventing harvest
  evidence: queue_pending=0, server healthy
  timestamp: 2026-06-29
- hypothesis: Wrong workspace hash in DB
  evidence: d1915ee... matches /Users/tamlh/workspaces/NUSTechnology/Projects/zengamingx — correct
  timestamp: 2026-06-29

## Resolution

root_cause: initClaudeCodeHarvester computed SHA256(cfg.SessionDir) where cfg.SessionDir was the Claude session directory path (~/.claude/projects/<encoded>), not the registered workspace root path. This always produced a hash that didn't match any registered workspace, so the harvester silently returned (nil, nil) on every startup. Additionally, detectClaudeCodeStorageDir() was never called in main.go so cfg.SessionDir was always empty (harvester disabled by default), compounding the issue.
fix: Rewrote initClaudeCodeHarvester as initClaudeCodeHarvesters (plural): it now accepts the parent ~/.claude/projects/ directory, queries all registered workspaces from DB, encodes each workspace path to the Claude directory name format (replace '/' with '-'), checks for existence of the encoded dir under the projects parent, and creates one ClaudeCodeHarvester per match using the correct workspace hash. Also added auto-detection call for cfg.Harvester.ClaudeCode.SessionDir in main.go (mirrors the existing OpenCode auto-detection pattern).
verification: go test -race -short ./... passes (all 28 test packages, exit 0). Build clean (CGO_ENABLED=0 go build ./cmd/nano-brain/).
files_changed:
  - cmd/nano-brain/claudecode_init.go
  - cmd/nano-brain/claudecode_init_test.go
  - cmd/nano-brain/main.go
