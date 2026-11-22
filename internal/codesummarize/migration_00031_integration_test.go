//go:build integration

package codesummarize

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
	"github.com/nano-brain/nano-brain/migrations"
	"github.com/pressly/goose/v3"
)

func TestMigration00031_PreservesExistingFailuresAndSeparatesNewOnes(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	if err := goose.DownToContext(ctx, db, ".", 30); err != nil {
		t.Fatal(err)
	}
	q := sqlc.New(db)
	workspace := uuid.NewString()
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: workspace, Name: "fixture", Path: "/tmp/fixture",
	}); err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	if _, err := db.ExecContext(ctx, `INSERT INTO code_summarization_failures
		(id, workspace_hash, symbol_name, source_file, content_hash, error_reason, error_type)
		VALUES ($1, $2, 'Shared', 'src/a.go', 'symbol-shared', 'HTTP 524', 'transient_exhausted')`,
		id, workspace); err != nil {
		t.Fatal(err)
	}
	if err := goose.UpContext(ctx, db, "."); err != nil {
		t.Fatal(err)
	}
	if err := q.UpsertCodeSummarizationFailure(ctx, sqlc.UpsertCodeSummarizationFailureParams{
		WorkspaceHash: workspace, ContentHash: "symbol-shared",
		SourceFile: "src/b.go", SymbolName: "Shared",
		ErrorType: "transient_exhausted", ErrorReason: "HTTP 524",
	}); err != nil {
		t.Fatal(err)
	}
	failures, err := q.GetUnresolvedFailures(ctx, workspace)
	if err != nil || len(failures) != 2 {
		t.Fatalf("migration lost or conflated failures: rows=%v err=%v", failures, err)
	}
	for _, failure := range failures {
		if failure.SourceFile == "src/a.go" && failure.ID != id {
			t.Fatalf("old failure ID changed: got %s, want %s", failure.ID, id)
		}
	}
}

func TestMigration00031_LongSourcePathCanRecordFailure(t *testing.T) {
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
	path := "/tmp/"
	for i := range 50 {
		path += fmt.Sprintf("%x/", sha256.Sum256([]byte{byte(i)}))
	}
	path += "file.go"
	if err := q.UpsertCodeSummarizationFailure(ctx, sqlc.UpsertCodeSummarizationFailureParams{
		WorkspaceHash: workspace, ContentHash: "symbol-long-path",
		SourceFile: path, SymbolName: "Shared",
		ErrorType: "transient_exhausted", ErrorReason: "HTTP 524",
	}); err != nil {
		t.Fatalf("valid long source path could not persist failure: %v", err)
	}
	for _, source := range []string{"src/a.go", `src/\x61.go`} {
		if err := q.UpsertCodeSummarizationFailure(ctx, sqlc.UpsertCodeSummarizationFailureParams{
			WorkspaceHash: workspace, ContentHash: "symbol-escape",
			SourceFile: source, SymbolName: "Shared",
			ErrorType: "transient_exhausted", ErrorReason: "HTTP 524",
		}); err != nil {
			t.Fatal(err)
		}
	}
	failures, err := q.GetUnresolvedFailures(ctx, workspace)
	if err != nil || len(failures) != 3 {
		t.Fatalf("distinct source paths collided in the identity index: rows=%v err=%v", failures, err)
	}
}
