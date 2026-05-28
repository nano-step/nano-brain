# API Spec: POST /api/v1/summarize

## Request

```
POST /api/v1/summarize
Content-Type: application/json
```

### Body

| Field | Type | Required | Default | Constraints |
|-------|------|----------|---------|-------------|
| `workspace` | string | yes | — | Extracted by workspace middleware from JSON body |
| `source` | string | no | `""` (all) | `"opencode"` or `"claude"` (maps to tag `"claude_code"`) |
| `limit` | int | no | 10 | Server clamps to range [1, 20] |
| `force` | bool | no | false | Skip pre-existence check; always calls LLM |

### Example

```json
{
  "workspace": "abc123",
  "source": "opencode",
  "limit": 10,
  "force": false
}
```

## Response

### 200 OK

```json
{
  "summarized": 5,
  "skipped": 4,
  "errors": 1
}
```

| Field | Description |
|-------|-------------|
| `summarized` | Sessions where `SummarizeAndPersist` succeeded |
| `skipped` | Sessions skipped (summary already exists and `force=false`) |
| `errors` | Sessions where `SummarizeAndPersist` returned an error |

Note: 200 is returned even when `errors > 0` (matches `/api/harvest` precedent). Callers should inspect the `errors` field.

### 400 Bad Request

JSON binding failure (malformed body, invalid field types).

### 503 Service Unavailable

```json
{"message": "summarization not configured"}
```

Returned when `summarization.enabled = false` in config (no summarizer wired to server).

## Behavior

1. Workspace middleware extracts `workspace` from JSON body. Missing/invalid workspace → 400 from middleware.
2. Handler checks summarizer is non-nil. If nil → 503.
3. Clamps `limit` to [1, 20]. Zero or negative → use default 10.
4. Queries `ListSessionDocumentsByWorkspace` — only `collection='sessions'`, filtered by source tag if provided, ordered by `created_at DESC`, limited.
5. For each session document, unless `force=true`: checks if summary already exists via `GetDocumentBySourcePath`. If exists → `skipped++`, continue.
6. Calls `SummarizeAndPersist(ctx, doc.Content, meta)`. Error → `errors++` + log warn, continue. Success → `summarized++`.
7. Returns counts. No partial-failure status codes.

## Idempotency

- `force=false` (default): Idempotent. Re-running returns the same `skipped` count with no LLM calls.
- `force=true`: Calls LLM for every candidate session. The Persister deduplicates by output hash — if LLM produces identical text, no DB write occurs. But LLM is called regardless.

## Concurrency

A server-side mutex (`summarizeMu`) serializes concurrent `/api/v1/summarize` requests. A second request while one is in progress will block until the first completes.
