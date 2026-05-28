## Context

nano-brain harvests AI coding sessions from two sources:
- **OpenCode**: SQLite DB (`~/.local/share/opencode/opencode.db`) — `message.data` JSON blobs with role, agent, tokens, system prompts, tool calls
- **Claude Code**: JSONL files (`~/.claude/transcripts/ses_*.jsonl`) — line-per-event with type=user/tool_use/tool_result

Current pipeline: harvest → render markdown → chunk → embed. Raw session content (including massive system prompts, tool stdout, code blocks) goes directly into vector DB.

Sessions range from 1K to 1M+ tokens. 6,833 OpenCode sessions exist, 5,922 have parent_id (subagent sessions). No "session done" signal — `time_archived` is always NULL.

Existing infrastructure: embedding providers (Ollama, VoyageAI), chunker (`chunk.Split`), document upsert with cascade cleanup (`UpsertDocumentBySourcePath`), embed queue. No LLM generation/completion client exists.

## Goals / Non-Goals

**Goals:**
- Produce structured session summaries (goal, decisions, files, problems, learnings) via LLM
- Handle sessions up to 1M tokens via map-reduce chunking
- Save summaries as physical `.md` files AND embed in vector DB
- Include cross-session links (parent/child/sibling, project grouping)
- LLM provider configurable in config.yaml, documented in setup guide
- Idempotent: re-summarize on content change, skip unchanged sessions

**Non-Goals:**
- Real-time/streaming summarization during active sessions
- Session "done" detection (use content_hash change detection instead)
- Multiple summary formats (only structured markdown)
- Pillar 4 self-learning integration (future work)
- Custom summarization prompts per user (single built-in prompt set)

## Decisions

### D1: OpenAI-compatible HTTP client over Go SDK

**Choice**: Raw `net/http` client targeting `/v1/chat/completions` endpoint with SSE streaming.

**Why**: No new Go dependencies (project constraint). ai-proxy already exposes OpenAI-compatible API. Existing `embed/ollama.go` and `embed/voyageai.go` use the same pattern — pure HTTP + JSON parsing. One client covers ai-proxy, OpenAI, Ollama (`/v1/` compat mode), and any OpenAI-compatible provider.

**Alternative rejected**: Go SDKs (openai-go, anthropic-sdk) — adds module dependencies, violates project constraint.

### D2: 3-stage hybrid pipeline (Strip → Map → Reduce)

**Choice**: 
1. **Strip** (Go, no LLM): Remove system prompts, tool output bodies, large code blocks, binary data. Replace with short placeholders (`[tool: grep, 45 results]`, `[code block: 234 lines, Go]`).
2. **Map** (parallel LLM): Chunk stripped content at ~4K chars, summarize each chunk independently with bounded concurrency (default 3 goroutines).
3. **Reduce** (single LLM): Merge all chunk summaries into final structured markdown.

**Why**: Strip reduces input 40-50% → fewer chunks → fewer LLM calls → lower cost. Map parallelizes for speed. Reduce produces coherent final output.

**Alternative rejected**: Pure map-reduce without strip — wastes tokens on noise. Refine (sequential) — too slow for 50+ chunk sessions.

### D3: Summary source_path scheme for idempotent upsert

**Choice**: `source_path = "summary://opencode/{session_id}"` or `"summary://claude/{filename}"`.

**Why**: Leverages existing `UpsertDocumentBySourcePath` with cascade delete. Same session re-summarized → same source_path → old chunks+embeddings auto-deleted → new ones created. Zero orphans, zero duplicates.

### D4: Summarize in harvest cycle, not separate scheduler

**Choice**: After `HarvestAll()` processes a session (newly created or content_hash changed), immediately queue it for summarization. Summarization runs in the same goroutine as harvest, bounded by LLM concurrency.

**Why**: Simple — no separate scheduler, no coordination. Harvest already runs every 5 minutes and detects changed sessions. Summarization adds latency to the harvest cycle but that's acceptable (sessions don't need real-time summaries).

**Risk**: Long sessions (1M tokens) may take 30-60s to summarize, blocking the harvest ticker. Mitigation: run summarization async with a buffered channel, same pattern as embed queue.

### D5: Session relationship mapping via DB queries

**Choice**: At summarization time, query `session` table for:
- Parent session: `WHERE id = {parent_id}` → get title
- Child sessions: `WHERE parent_id = {session_id}` → list subagent sessions
- Sibling sessions: `WHERE parent_id = {same_parent_id}` → other subagents in same task

Embed these as links in the summary markdown header.

**Why**: Relationships already exist in OpenCode DB (`parent_id`, `path`, `agent` columns). No new storage needed. Claude JSONL sessions don't have this metadata — skip linking for Claude sessions.

### D6: Config structure

```yaml
summarization:
  enabled: true
  provider_url: "https://ai-proxy.thnkandgrow.com/v1"
  api_key: ""        # or env: NANO_BRAIN_SUMMARIZE_API_KEY
  model: "nano-brain"
  max_tokens: 4096
  concurrency: 3
  output_dir: "~/.nano-brain/summaries"
```

**Why**: Mirrors existing `embedding` config pattern. `enabled` flag allows disabling without removing config. `output_dir` for physical `.md` files. `concurrency` bounds parallel map calls.

### D7: Strip logic — format-specific

**OpenCode sessions** (from rendered markdown):
- Strip `system` field from message data (contains entire skills list — can be 50K+ chars)
- Strip tool call result bodies → `[tool: {name}, {line_count} lines]`
- Strip code blocks >20 lines → `[code block: {lines} lines, {lang}]`
- Strip base64/binary patterns
- Keep: user message text, assistant reasoning text, file paths, error messages (first instance + count)

**Claude JSONL sessions** (from rendered markdown):
- Strip `tool_result.tool_output.output` → `[tool: {tool_name}, {line_count} lines]`  
- Strip `tool_use.tool_input.command` if >5 lines → `[command: {first_line}...]`
- Keep: user content, timestamps, tool names

### D8: File naming convention

Format: `{source}_{title_slug}_{date}.md`
- OpenCode: `opencode_{slugify(session.title)}_{YYYY-MM-DD}.md`
- Claude: `claude_{slugify(first_user_message[:60])}_{YYYY-MM-DD}.md`

`slugify`: lowercase, replace non-alphanum with `-`, collapse multiple `-`, trim to 80 chars.

## Risks / Trade-offs

**[LLM provider unavailable]** → Summarization silently skipped for this cycle; session harvested normally (chunks+embed raw content as fallback); retry next cycle. Log warning.

**[Session too large for reduce step]** → If merged chunk summaries exceed model context (~200K tokens), do hierarchical reduce: group summaries in batches of 10 → reduce each batch → final reduce. Increases LLM calls but handles arbitrarily large sessions.

**[Cost accumulation]** → 6,833 existing sessions × ~$0.10/summary = ~$680 for initial backfill. Mitigation: add `--since` flag to harvest CLI to limit backfill window. Ongoing cost: only new/changed sessions, ~5-20/day = $0.50-2/day.

**[Harvest cycle latency]** → Large session summarization (30-60s) blocks ticker. Mitigation: async summarization queue (buffered channel) — harvest enqueues, summarizer processes independently. Same pattern as embed queue.

**[Strip removes valuable context]** → Aggressive stripping might lose debugging context from tool outputs. Mitigation: strip only the body/output, always preserve tool name + first 200 chars. Error messages always kept.

**[Duplicate summaries on concurrent harvest]** → Two harvest cycles run overlapping. Mitigation: source_path upsert is idempotent — last writer wins, no duplicates.
