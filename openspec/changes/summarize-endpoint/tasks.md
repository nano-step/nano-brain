# Tasks: summarize-endpoint

## T1 — Add ListSessionDocumentsByWorkspace sqlc query

**File**: `internal/storage/queries/documents.sql`

Add after existing queries:

```sql
-- name: ListSessionDocumentsByWorkspace :many
SELECT id, workspace_hash, content_hash, title, source_path, collection, tags, content, created_at, updated_at
FROM documents
WHERE workspace_hash = @workspace_hash
  AND collection = 'sessions'
  AND (@tag_filter::text = '' OR @tag_filter::text = ANY(tags))
ORDER BY created_at DESC
LIMIT @lim;
```

Then run `sqlc generate` from project root to regenerate `internal/storage/sqlc/documents.sql.go`.

**Verify**: New `ListSessionDocumentsByWorkspace` function exists in generated file. Takes `ListSessionDocumentsByWorkspaceParams` with fields `WorkspaceHash`, `TagFilter`, `Lim`. Returns `[]ListSessionDocumentsByWorkspaceRow` with `Content` field present.

---

## T2 — Add SummarizeHandler

**File**: `internal/server/handlers/summarize.go` (new)

```go
package handlers

import (
    "net/http"
    "path"
    "strings"
    "time"

    "github.com/labstack/echo/v4"
    "github.com/nano-step/nano-brain/internal/harvest"
    "github.com/nano-step/nano-brain/internal/storage/sqlc"
)

const (
    summarizeDefaultLimit = 10
    summarizeMaxLimit     = 20
)

type SummarizeRequest struct {
    Workspace string `json:"workspace"`
    Source    string `json:"source"`
    Limit     int    `json:"limit"`
    Force     bool   `json:"force"`
}

type SummarizeResponse struct {
    Summarized int `json:"summarized"`
    Skipped    int `json:"skipped"`
    Errors     int `json:"errors"`
}

// SummarizeSummarizer is the interface the handler depends on.
// Matches harvest.SessionSummarizer.
type SummarizeSummarizer interface {
    SummarizeAndPersist(ctx context.Context, content string, meta harvest.SummaryMeta) error
}

// TriggerSummarize returns an Echo handler for POST /api/v1/summarize.
// getSummarizer is a closure that returns the current summarizer (nil if not configured).
// queries is used to fetch session documents.
func TriggerSummarize(
    getSummarizer func() SummarizeSummarizer,
    queries sqlcQuerier,
    logger zerolog.Logger,
) echo.HandlerFunc {
    return func(c echo.Context) error {
        var req SummarizeRequest
        if err := c.Bind(&req); err != nil {
            return echo.NewHTTPError(http.StatusBadRequest, err.Error())
        }
        workspace := c.Get("workspace").(string)

        s := getSummarizer()
        if s == nil {
            return echo.NewHTTPError(http.StatusServiceUnavailable, "summarization not configured")
        }

        lim := req.Limit
        if lim <= 0 {
            lim = summarizeDefaultLimit
        }
        if lim > summarizeMaxLimit {
            lim = summarizeMaxLimit
        }

        // Normalize source filter: "claude" → "claude_code"
        tagFilter := req.Source
        if tagFilter == "claude" {
            tagFilter = "claude_code"
        }

        ctx := c.Request().Context()
        docs, err := queries.ListSessionDocumentsByWorkspace(ctx, sqlc.ListSessionDocumentsByWorkspaceParams{
            WorkspaceHash: workspace,
            TagFilter:     tagFilter,
            Lim:           int32(lim),
        })
        if err != nil {
            return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("list sessions: %v", err))
        }

        reqLog := LoggerFromCtx(c, logger)
        var summarized, skipped, errors int

        for _, doc := range docs {
            sessionID := path.Base(doc.SourcePath)
            summaryPath := "session-summary://" + sourceFromTags(doc.Tags) + "/" + sessionID

            if !req.Force {
                existing, lookupErr := queries.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
                    WorkspaceHash: workspace,
                    SourcePath:    summaryPath,
                })
                if lookupErr == nil && existing.ID != uuid.Nil {
                    skipped++
                    continue
                }
            }

            source := sourceFromTags(doc.Tags)
            meta := harvest.SummaryMeta{
                Source:    source,
                SessionID: sessionID,
                Title:     strings.TrimPrefix(doc.Title, "Session: "),
                CreatedAt: doc.CreatedAt,
            }

            if err := s.SummarizeAndPersist(ctx, doc.Content, meta); err != nil {
                reqLog.Warn().Err(err).Str("session_id", sessionID).Msg("summarize_failed")
                errors++
                continue
            }
            summarized++
        }

        reqLog.Info().
            Str("workspace", workspace).
            Int("summarized", summarized).
            Int("skipped", skipped).
            Int("errors", errors).
            Msg("summarize triggered")

        return c.JSON(http.StatusOK, SummarizeResponse{
            Summarized: summarized,
            Skipped:    skipped,
            Errors:     errors,
        })
    }
}

// sourceFromTags returns "opencode" or "claude" from document tags.
func sourceFromTags(tags []string) string {
    for _, t := range tags {
        if t == "claude_code" {
            return "claude"
        }
        if t == "opencode" {
            return "opencode"
        }
    }
    return "opencode"
}
```

Note: `sqlcQuerier` interface must include both `ListSessionDocumentsByWorkspace` and `GetDocumentBySourcePath`. Check existing handler pattern for how queries interface is defined (e.g., `handlers/harvest.go`).

**Verify**: `go build ./internal/server/handlers/` compiles clean.

---

## T3 — Add summarizeMu + SetSummarizer to Server

**File**: `internal/server/server.go`

Follow the exact `harvestMu`/`harvestRunner` pattern:

1. Add fields to Server struct:
```go
summarizeMu   sync.RWMutex
summarizer    handlers.SummarizeSummarizer
```

2. Add methods:
```go
func (s *Server) SetSummarizer(sum handlers.SummarizeSummarizer) {
    s.summarizeMu.Lock()
    defer s.summarizeMu.Unlock()
    s.summarizer = sum
}

func (s *Server) getSummarizer() handlers.SummarizeSummarizer {
    s.summarizeMu.RLock()
    defer s.summarizeMu.RUnlock()
    return s.summarizer
}
```

**Verify**: `go build ./internal/server/` compiles clean.

---

## T4 — Register route in routes.go

**File**: `internal/server/routes.go`

In the `data` group (workspace-scoped), add:

```go
data.POST("/summarize", handlers.TriggerSummarize(s.getSummarizer, s.queries, s.logger))
```

Place near the `/embed` or `/reindex` route for logical grouping.

**Verify**: `go build ./internal/server/` compiles clean. `grep -n "summarize" internal/server/routes.go` shows the route.

---

## T5 — Wire summarizer in main.go

**File**: `cmd/nano-brain/main.go`

After the existing `harvestSummarizer` is constructed and assigned (where `s.SetHarvestRunner(runner)` is called), add:

```go
if harvestSummarizer != nil {
    srv.SetSummarizer(harvestSummarizer)
}
```

`harvestSummarizer` is already of type `harvest.SessionSummarizer`. The `handlers.SummarizeSummarizer` interface has the same single method — if they match structurally, no adapter needed. If not, create a thin wrapper.

**Verify**: `go build ./cmd/nano-brain/` compiles clean.

---

## T6 — Write unit tests

**File**: `internal/server/handlers/summarize_test.go` (new)

Follow `handlers/harvest_test.go` pattern. Required test cases:

| Test | Setup | Assert |
|------|-------|--------|
| `TestSummarize_Success` | Mock summarizer returns nil; mock queries return 3 docs (1 no existing summary, 1 has summary, 1 has summary + force=false) | `{"summarized":1,"skipped":2,"errors":0}`, status 200 |
| `TestSummarize_NilSummarizer` | getSummarizer returns nil | status 503, body contains "summarization not configured" |
| `TestSummarize_InvalidRequest` | Malformed JSON | status 400 |
| `TestSummarize_LimitCap` | Request with `limit:100` | verify query called with lim=20 |
| `TestSummarize_ForceFlag` | force=true, summary doc exists | getSummarizer called (pre-check skipped), not skipped |
| `TestSummarize_SourceFilter_Claude` | `source:"claude"` | query called with tag_filter="claude_code" |
| `TestSummarize_SummarizeError` | Mock summarizer returns error for 1 doc | `{"summarized":0,"skipped":0,"errors":1}`, status 200 |

**Verify**: `go test -race -short ./internal/server/handlers/` passes.

---

## T7 — Validation ladder

Run in order:

1. `go build ./...` — must exit 0
2. `go test -race -short ./...` — must pass (new tests + regression)
3. `go test -race -tags=integration ./...` — must pass
4. Smoke: start server, `curl -X POST http://localhost:3100/api/v1/summarize -H 'Content-Type: application/json' -d '{"workspace":"<hash>","limit":5}'`
   - If summarization disabled → expect `{"message":"summarization not configured"}` with 503
   - If enabled → expect `{"summarized":N,"skipped":M,"errors":0}` with 200

---

## Completion Criteria

- [ ] T1: `ListSessionDocumentsByWorkspace` in generated sqlc with `Content` field
- [ ] T2: `internal/server/handlers/summarize.go` compiles
- [ ] T3: `SetSummarizer`/`getSummarizer` on Server struct
- [ ] T4: Route registered under `/api/v1/summarize`
- [ ] T5: Summarizer wired in main.go
- [ ] T6: All 7 unit tests pass
- [ ] T7: Full validation ladder green
- [ ] README updated: add `POST /api/v1/summarize` to REST API table
