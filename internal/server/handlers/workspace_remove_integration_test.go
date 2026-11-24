//go:build integration

package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
	"github.com/rs/zerolog"
)

// TestRemoveWorkspaceTxCleanupFailureRollsBack covers issue #714: inside the
// remove/reset transaction a failed summarization-cleanup statement aborts the
// tx, so it must roll back and return 500 instead of warn-and-continue into a
// guaranteed DeleteWorkspace failure.
func TestRemoveWorkspaceTxCleanupFailureRollsBack(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	q := sqlc.New(db)
	ctx := context.Background()

	const wsHash = "ws-tx-rollback"
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: wsHash, Name: "tx-rollback", Path: "/tmp/tx-rollback",
	}); err != nil {
		t.Fatalf("UpsertWorkspace: %v", err)
	}
	if _, err := q.UpsertDocumentBySourcePath(ctx, sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: wsHash, ContentHash: "h1", Title: "doc", Content: "body",
		SourcePath: "/tmp/tx-rollback/a.txt", Collection: "code",
	}); err != nil {
		t.Fatalf("seed doc: %v", err)
	}

	// Force DeleteCodeSummarizationUsageByWorkspace to fail inside the tx.
	// Renaming a column (rather than DROP TABLE) keeps the failure scoped to
	// this test's schema: search_path falls through to the shared public
	// schema's copy if the schema-local table disappears entirely.
	if _, err := db.ExecContext(ctx, `ALTER TABLE code_summarization_usage RENAME COLUMN workspace_hash TO workspace_hash_gone`); err != nil {
		t.Fatalf("rename column: %v", err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/"+wsHash, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("hash")
	c.SetParamValues(wsHash)

	h := handlers.RemoveWorkspace(q, db, nil, zerolog.Nop())
	err := h(c)
	if err == nil {
		t.Fatal("expected 500 when summarization cleanup fails inside tx")
	}
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %v", err)
	}
	// Assert the fix-specific message: pre-fix warn-and-continue code would
	// surface a later tx failure ("failed to delete workspace") instead.
	if he, ok := err.(*echo.HTTPError); !ok || !strings.Contains(fmt.Sprint(he.Message), "cleanup code summarization") {
		t.Fatalf("expected cleanup-failure 500, got %v", err)
	}

	// The tx must have rolled back: workspace row AND its document survive.
	if _, err := q.GetWorkspaceByHash(ctx, wsHash); err != nil {
		t.Fatalf("workspace row lost despite rollback: %v", err)
	}
	if _, err := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath: "/tmp/tx-rollback/a.txt", WorkspaceHash: wsHash,
	}); err != nil {
		t.Fatalf("document lost despite rollback: %v", err)
	}
}

// TestResetWorkspaceTxCleanupFailureRollsBack is the reset_workspace.go twin:
// documents must survive when the tx aborts on the cleanup failure.
func TestResetWorkspaceTxCleanupFailureRollsBack(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	q := sqlc.New(db)
	ctx := context.Background()

	const wsHash = "ws-reset-rollback"
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: wsHash, Name: "reset-rollback", Path: "/tmp/reset-rollback",
	}); err != nil {
		t.Fatalf("UpsertWorkspace: %v", err)
	}
	if _, err := q.UpsertDocumentBySourcePath(ctx, sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: wsHash, ContentHash: "h1", Title: "doc", Content: "body",
		SourcePath: "/tmp/reset-rollback/a.txt", Collection: "code",
	}); err != nil {
		t.Fatalf("seed doc: %v", err)
	}

	// Force DeleteCodeSummarizationFailuresByWorkspace to fail inside the tx
	// (usage cleanup succeeds first, proving per-statement rollback). Same
	// search_path caveat as above — rename, don't drop.
	if _, err := db.ExecContext(ctx, `ALTER TABLE code_summarization_failures RENAME COLUMN workspace_hash TO workspace_hash_gone`); err != nil {
		t.Fatalf("rename column: %v", err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reset-workspace", strings.NewReader(`{"workspace":"`+wsHash+`"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := handlers.ResetWorkspace(q, db, zerolog.Nop())
	err := h(c)
	if err == nil {
		t.Fatal("expected 500 when summarization cleanup fails inside tx")
	}
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %v", err)
	}
	// Assert the fix-specific message: pre-fix warn-and-continue code would
	// surface a later tx failure ("failed to commit transaction") instead.
	if he, ok := err.(*echo.HTTPError); !ok || !strings.Contains(fmt.Sprint(he.Message), "cleanup code summarization") {
		t.Fatalf("expected cleanup-failure 500, got %v", err)
	}

	// Rollback proof: the document DeleteDocumentsByWorkspace removed inside
	// the tx must still be there.
	if _, err := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath: "/tmp/reset-rollback/a.txt", WorkspaceHash: wsHash,
	}); err != nil {
		t.Fatalf("document lost despite rollback: %v", err)
	}
}
