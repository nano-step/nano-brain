# handlers

HTTP handler functions for all REST API endpoints. One file per endpoint group.

## File Inventory

| Domain | Files |
|--------|-------|
| Search | `query.go` (hybrid BM25+vector+RRF), `search.go`/`bm25.go` (BM25), `symbols.go` |
| Documents | `document.go`, `embed.go`, `reindex.go`, `reload.go`, `summarize.go` |
| Workspace | `workspace.go` (init/list), `reset_workspace.go`, `collection.go`, `tags.go`, `wakeup.go` |
| Graph | `graph.go`, `impact.go`, `trace.go` |
| Infra | `health.go` (`/health` + `/api/status`), `harvest.go`, `context.go` |

## Handler Pattern

Constructor function returning `echo.HandlerFunc`. Dependencies passed as arguments at
registration time. Each file defines a small interface for its dependency (e.g.,
`HybridSearcher`, `WorkspaceQuerier`) to keep the handler testable without the full server.

```go
func Query(searcher HybridSearcher, logger zerolog.Logger) echo.HandlerFunc {
    return func(c echo.Context) error { /* ... */ }
}
// routes.go:
data.POST("/query", handlers.Query(s.searchService, s.logger))
```

## Context Helpers (`context.go`)

- `LoggerFromCtx(c echo.Context, fallback zerolog.Logger)` — returns per-request logger
  stored under key `"logger"` by middleware, or `fallback` if absent.
- `workspace` string — injected by `workspaceMiddleware()` in `../routes.go`, read via
  `c.Get("workspace").(string)`. Workspace-scoped handlers return 400 if it's empty.

## Request / Response Pattern

```go
var req MyRequest
if err := c.Bind(&req); err != nil {
    return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
}
return c.JSON(http.StatusOK, myResponse{...})
```

Always `echo.NewHTTPError(statusCode, "message")` for errors. Never `c.JSON(4xx, ...)`.

## Adding a New Endpoint

1. Create `handlers/<name>.go` — define interface, request/response structs, constructor func.
2. Register in `../routes.go` inside `registerRoutes`. Workspace-scoped routes go on the
   `data` group (has `workspaceMiddleware`); public routes on `s.echo` or `api`.
3. Add `handlers/<name>_test.go`.

## Testing Pattern

```go
package handlers_test  // external test package

type mockSearcher struct{ fn func(...) ([]Result, error) }
func (m *mockSearcher) Search(...) ([]Result, error) { return m.fn(...) }

e := echo.New()
req := httptest.NewRequest(http.MethodPost, "/api/v1/query", strings.NewReader(body))
req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
rec := httptest.NewRecorder()
c := e.NewContext(req, rec)
c.Set("workspace", "test-hash")  // inject workspace

err := handlers.Query(mock, zerolog.Nop())(c)
// assert err, rec.Code, parse rec.Body
```
