# Design: Harvester SQLite Support

## DB Schema (read-only)

```sql
session(id, project_id, slug, directory, title, time_created, time_updated)
message(id, session_id, time_created, time_updated, data JSON)
part(id, message_id, session_id, time_created, time_updated, data JSON)
```

- `message.data`: `{"role":"user"|"assistant", "agent":"...", "time":{"created":...}, ...}`
- `part.data`: `{"type":"text", "text":"actual content", ...}`

## Function: `harvestFromDb()`

```
harvestFromDb(dbPath, outputDir, stateFile, state) → { harvested[], stateChanged, counters }
```

1. Open `opencode.db` with `better-sqlite3` in read-only mode
2. Query all sessions: `SELECT id, slug, directory, title, time_created, time_updated FROM session`
3. For each session, apply skip logic using state (keyed by session ID):
   - Skip if `state[sessionId].skipped`
   - Skip if `state[sessionId].mtime >= session.time_updated` AND messageCount unchanged
4. For changed sessions, query messages: `SELECT id, time_created, data FROM message WHERE session_id = ? ORDER BY time_created`
5. Parse `message.data` JSON for role/agent
6. For each message, query parts: `SELECT data FROM part WHERE message_id = ? ORDER BY time_created`
7. Filter `part.data.type === 'text'` and concatenate text
8. Apply same incremental logic (append new messages if previous count exists)
9. Write markdown output in same format
10. Update state with `{mtime: session.time_updated, messageCount}`

## Entry Point Change

```
harvestSessions(options):
  dbPath = join(dirname(sessionDir), 'opencode.db')
  if (existsSync(dbPath)):
    sessionCount = quick count from DB
    if (sessionCount > 0):
      return harvestFromDb(dbPath, outputDir, state, ...)
  // fallback to existing JSON logic
  return harvestFromJson(...)
```

## State Key Format

- DB sessions: `ses_xxx` (session ID, no extension)
- JSON sessions: `ses_xxx.json` (filename, has extension)
- No collision possible between the two

## Performance

- Open DB once per harvest cycle, close after
- Batch queries by session (not one giant join)
- Read-only mode: `{ readonly: true }` flag on better-sqlite3
