//go:build integration

package codesummarize

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
	"github.com/sqlc-dev/pqtype"
)

func seedPendingSymbol(t *testing.T, ctx context.Context, q *sqlc.Queries, workspace, name string) {
	t.Helper()
	doc, err := q.UpsertDocumentBySourcePath(ctx, sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: workspace,
		ContentHash:   "file-" + name,
		Content:       "func " + name + "() {}",
		SourcePath:    "src/" + name + ".go",
		Collection:    "code",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = q.UpsertChunk(ctx, sqlc.UpsertChunkParams{
		DocumentID:    doc.ID,
		WorkspaceHash: workspace,
		ContentHash:   "symbol-" + name,
		Content:       "func " + name + "() {}",
		ChunkType:     "symbol",
		SymbolName:    sql.NullString{String: name, Valid: true},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetUnsummarizedSymbols_CooldownAndNeverAttemptedFirst(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	q := sqlc.New(db)
	workspace := uuid.NewString()
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{Hash: workspace, Name: "fixture", Path: "/tmp/fixture"}); err != nil {
		t.Fatal(err)
	}
	seedPendingSymbol(t, ctx, q, workspace, "Fails")
	if err := q.UpsertCodeSummarizationFailure(ctx, sqlc.UpsertCodeSummarizationFailureParams{
		WorkspaceHash: workspace, SymbolName: "Fails", SourceFile: "src/Fails.go",
		ContentHash: "symbol-Fails", ErrorReason: "HTTP 524", ErrorType: "transient_exhausted",
	}); err != nil {
		t.Fatal(err)
	}

	fetch := func() []sqlc.GetUnsummarizedSymbolsRow {
		t.Helper()
		rows, err := q.GetUnsummarizedSymbols(ctx, sqlc.GetUnsummarizedSymbolsParams{WorkspaceHash: workspace, Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	if got := fetch(); len(got) != 0 {
		t.Fatalf("cooling-down failure was reselected: %v", got)
	}
	seedPendingSymbol(t, ctx, q, workspace, "New")
	if _, err := db.ExecContext(ctx, `UPDATE code_summarization_failures
		SET last_attempt_at = NOW() - interval '2 hours' WHERE workspace_hash = $1`, workspace); err != nil {
		t.Fatal(err)
	}
	if got := fetch(); len(got) != 2 || got[0].ContentHash != "symbol-New" || got[1].ContentHash != "symbol-Fails" {
		t.Fatalf("expected new symbol before eligible retry, got %v", got)
	}
	if _, err := db.ExecContext(ctx, `UPDATE code_summarization_failures
		SET attempts = 8 WHERE workspace_hash = $1`, workspace); err != nil {
		t.Fatal(err)
	}
	if got := fetch(); len(got) != 1 || got[0].ContentHash != "symbol-New" {
		t.Fatalf("capped failure should be parked, got %v", got)
	}
	if _, err := db.ExecContext(ctx, `UPDATE code_summarization_failures
		SET resolved_at = NOW() WHERE workspace_hash = $1`, workspace); err != nil {
		t.Fatal(err)
	}
	if got := fetch(); len(got) != 2 {
		t.Fatalf("resolved failure should be eligible again, got %v", got)
	}
}

func TestGetGraphContextStaleSymbols_RespectsFailureCooldown(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	q := sqlc.New(db)
	workspace := uuid.NewString()
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: workspace, Name: "fixture", Path: "/tmp/fixture",
	}); err != nil {
		t.Fatal(err)
	}
	seedPendingSymbol(t, ctx, q, workspace, "Stale")
	if _, err := q.UpsertDocument(ctx, sqlc.UpsertDocumentParams{
		WorkspaceHash: workspace, ContentHash: "summary-Stale",
		Content: "Old summary", SourcePath: "src/Stale.go?summary=true",
		Collection: "code", Tags: []string{"symbol-summary"},
		Metadata: pqtype.NullRawMessage{
			RawMessage: []byte(`{"source_content_hash":"symbol-Stale"}`), Valid: true,
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := q.UpsertCodeSummarizationFailure(ctx, sqlc.UpsertCodeSummarizationFailureParams{
		WorkspaceHash: workspace, SymbolName: "Stale", SourceFile: "src/Stale.go",
		ContentHash: "symbol-Stale", ErrorReason: "HTTP 524", ErrorType: "transient_exhausted",
	}); err != nil {
		t.Fatal(err)
	}

	fetch := func() []sqlc.GetSymbolChunksByGraphContextStaleRow {
		t.Helper()
		rows, err := q.GetSymbolChunksByGraphContextStale(ctx, sqlc.GetSymbolChunksByGraphContextStaleParams{
			WorkspaceHash: workspace, Limit: 10,
		})
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	if rows := fetch(); len(rows) != 0 {
		t.Fatalf("stale summary failure reselected before cooldown: %+v", rows)
	}
	forced, err := q.GetUnsummarizedSymbols(ctx, sqlc.GetUnsummarizedSymbolsParams{
		WorkspaceHash: workspace, Limit: 10, ForceHashes: []string{"symbol-Stale"},
	})
	if err != nil || len(forced) != 1 || forced[0].ContentHash != "symbol-Stale" {
		t.Fatalf("explicit retry must bypass cooldown and the old summary: %+v, err=%v", forced, err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE code_summarization_failures
		SET last_attempt_at = NOW() - interval '2 hours' WHERE workspace_hash = $1`, workspace); err != nil {
		t.Fatal(err)
	}
	if rows := fetch(); len(rows) != 1 || rows[0].ContentHash != "symbol-Stale" {
		t.Fatalf("expired stale failure should retry: %+v", rows)
	}
}
