# nano-brain

Persistent memory for AI coding agents. Hybrid search (BM25 + semantic + LLM reranking) across past sessions, codebase, notes, and daily logs.

## Slash Commands

| Command | When to Use |
|---------|-------------|
| `/nano-brain-init` | First time in a workspace — indexes codebase and sets up memory |
| `/nano-brain-status` | Check health: document counts, embedding progress, server status |
| `/nano-brain-reindex` | After git pull, branch switch, or major code changes |

## MCP Tools Reference

### Search Tools

| Tool | Use When | Speed |
|------|----------|-------|
| `memory_search` | Exact keyword: error messages, function names, specific terms | Fast |
| `memory_vsearch` | Conceptual: "how does auth work", "payment flow" | Medium |
| `memory_query` | Best quality, complex questions (combines BM25 + vector + reranking) | Slower |

**Default: Use `memory_query`** — it gives best results for most questions.

### Retrieval Tools

| Tool | Use When |
|------|----------|
| `memory_get` | Retrieve specific document by path or docid |
| `memory_multi_get` | Batch retrieve by glob pattern |

### Management Tools

| Tool | Use When |
|------|----------|
| `memory_status` | Check index health, embedding progress |
| `memory_index_codebase` | Rescan source files (call with `root` param) |
| `memory_update` | Refresh all collection indexes |
| `memory_write` | Save insight to daily log (tagged with workspace) |

## When to Use Memory

**Before starting work:**
- Recall past decisions on similar features
- Find debugging insights from previous sessions
- Check existing patterns in codebase

**After completing work:**
- Save key architectural decisions
- Document non-obvious fixes
- Record domain knowledge for future sessions

## Collection Filtering

Add `collection` parameter to narrow search scope:

| Collection | Contains |
|------------|----------|
| `codebase` | Source files from workspace |
| `sessions` | Past AI coding sessions |
| `memory` | Daily logs (tagged with workspace context) |

Omit `collection` to search everything (recommended for most queries).

## Memory vs Native Tools

| Task | Use |
|------|-----|
| Semantic search, past context, cross-session recall | **nano-brain** |
| Exact code patterns, AST structure, precise matches | **grep, ast-grep, glob** |

They complement each other. Use both.

## Troubleshooting

| Issue | Solution |
|-------|----------|
| "tool not found" | Add nano-brain to MCP config in opencode.json |
| 0 codebase docs | Run `/nano-brain-init` |
| Embeddings stuck | Check Ollama running: `ollama serve` |
| Stale results | Run `/nano-brain-reindex` |
