//go:build integration

package codesummarize

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
	"github.com/sqlc-dev/pqtype"
)

// docRow is a minimal projection used by the assertions below.
type docRow struct {
	sourcePath string
	content    string
}

func listSummaryDocsForSymbol(t *testing.T, ctx context.Context, db *sql.DB, workspaceHash, file, symbolName, kind string) []docRow {
	t.Helper()

	rows, err := db.QueryContext(ctx,
		`SELECT source_path, content FROM documents WHERE workspace_hash = $1 ORDER BY source_path`,
		workspaceHash)
	if err != nil {
		t.Fatalf("query documents: %v", err)
	}
	defer rows.Close()

	prefix := fmt.Sprintf("%s?symbol=%s&kind=%s", file, symbolName, kind)
	var out []docRow
	for rows.Next() {
		var r docRow
		if err := rows.Scan(&r.sourcePath, &r.content); err != nil {
			t.Fatalf("scan document row: %v", err)
		}
		if strings.HasPrefix(r.sourcePath, prefix) {
			out = append(out, r)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate document rows: %v", err)
	}
	return out
}

func chunkCountForDocument(t *testing.T, ctx context.Context, db *sql.DB, documentID uuid.UUID) int {
	t.Helper()

	var n int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM chunks WHERE document_id = $1`, documentID).Scan(&n)
	if err != nil {
		t.Fatalf("count chunks for document %s: %v", documentID, err)
	}
	return n
}

func documentIDBySourcePath(t *testing.T, ctx context.Context, db *sql.DB, workspaceHash, sourcePath string) uuid.UUID {
	t.Helper()

	var id uuid.UUID
	err := db.QueryRowContext(ctx,
		`SELECT id FROM documents WHERE workspace_hash = $1 AND source_path = $2`,
		workspaceHash, sourcePath).Scan(&id)
	if err != nil {
		t.Fatalf("lookup document id for %s: %v", sourcePath, err)
	}
	return id
}

// TestUpsertSummaryDocument_552_AC1 covers #552 AC-1: two summarizations of the
// same symbol with DIFFERENT content produce exactly ONE summary document
// (source_path has no "&hash="), with the latest content persisted.
func TestUpsertSummaryDocument_552_AC1(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	q := sqlc.New(db)
	ctx := context.Background()

	workspaceHash := uuid.New().String()[:8]
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: workspaceHash,
		Name: "test-workspace",
		Path: "/tmp/test-552",
	}); err != nil {
		t.Fatalf("UpsertWorkspace: %v", err)
	}

	svc := NewService(config.CodeSummarizationConfig{Model: "test-model"}, nil, nil, q, nil, testutil.NopLogger())

	symbol := &SymbolForSummary{
		Name:        "ProcessData",
		Kind:        "func",
		File:        "internal/processor.go",
		ContentHash: "hash-v1",
	}

	if err := svc.upsertSummaryDocument(ctx, workspaceHash, symbol, "First summary version"); err != nil {
		t.Fatalf("upsertSummaryDocument (1st call): %v", err)
	}

	symbol.ContentHash = "hash-v2"
	if err := svc.upsertSummaryDocument(ctx, workspaceHash, symbol, "Second summary version"); err != nil {
		t.Fatalf("upsertSummaryDocument (2nd call): %v", err)
	}

	docs := listSummaryDocsForSymbol(t, ctx, db, workspaceHash, symbol.File, symbol.Name, symbol.Kind)
	if len(docs) != 1 {
		t.Fatalf("expected exactly 1 summary doc for symbol, got %d: %+v", len(docs), docs)
	}

	got := docs[0]
	if strings.Contains(got.sourcePath, "&hash=") {
		t.Errorf("expected source_path to have no &hash= segment, got %q", got.sourcePath)
	}
	wantPath := fmt.Sprintf("%s?symbol=%s&kind=%s&summary=true", symbol.File, symbol.Name, symbol.Kind)
	if got.sourcePath != wantPath {
		t.Errorf("source_path = %q, want %q", got.sourcePath, wantPath)
	}
	if got.content != "Second summary version" {
		t.Errorf("content = %q, want latest content %q", got.content, "Second summary version")
	}

	// #552 BLOCKER regression: dedup must hold at the CHUNK level too. Two
	// different-content upserts must leave exactly ONE chunk under the doc —
	// otherwise stale chunks accumulate and still pollute chunk-level search.
	var chunkCount int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM chunks WHERE workspace_hash = $1 AND document_id = (
		    SELECT id FROM documents WHERE workspace_hash = $1 AND source_path = $2)`,
		workspaceHash, wantPath).Scan(&chunkCount); err != nil {
		t.Fatalf("count chunks for deduped doc: %v", err)
	}
	if chunkCount != 1 {
		t.Errorf("expected exactly 1 chunk after 2 different-content upserts, got %d", chunkCount)
	}
}

// TestUpsertSummaryDocument_552_AC2 covers #552 AC-2: after the fix, an upsert
// removes any legacy "&hash=...&summary=true" docs for that symbol (and their
// chunks via FK cascade), while the canonical no-hash doc survives.
func TestUpsertSummaryDocument_552_AC2(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	q := sqlc.New(db)
	ctx := context.Background()

	workspaceHash := uuid.New().String()[:8]
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: workspaceHash,
		Name: "test-workspace",
		Path: "/tmp/test-552-legacy",
	}); err != nil {
		t.Fatalf("UpsertWorkspace: %v", err)
	}

	symbol := &SymbolForSummary{
		Name:        "HybridSearch",
		Kind:        "func",
		File:        "internal/search/hybrid.go",
		ContentHash: "hash-current",
	}

	// Seed a legacy hash-path summary doc + chunk directly, simulating pre-D1 data.
	legacyPath := fmt.Sprintf("%s?symbol=%s&kind=%s&hash=deadbeef&summary=true", symbol.File, symbol.Name, symbol.Kind)
	legacyContent := "Legacy stale summary"
	legacyContentHash := computeContentHash(legacyContent)
	metadataJSON, err := json.Marshal(map[string]interface{}{
		"symbol_name": symbol.Name,
		"symbol_kind": symbol.Kind,
		"source_file": symbol.File,
	})
	if err != nil {
		t.Fatalf("marshal legacy metadata: %v", err)
	}

	legacyDocRow, err := q.UpsertDocument(ctx, sqlc.UpsertDocumentParams{
		WorkspaceHash: workspaceHash,
		ContentHash:   legacyContentHash,
		Title:         fmt.Sprintf("Summary: %s (%s)", symbol.Name, symbol.Kind),
		Content:       legacyContent,
		SourcePath:    legacyPath,
		Collection:    "code",
		Tags:          []string{"symbol-summary"},
		Metadata:      pqtype.NullRawMessage{RawMessage: metadataJSON, Valid: true},
	})
	if err != nil {
		t.Fatalf("seed legacy UpsertDocument: %v", err)
	}
	legacyDocID := legacyDocRow.ID

	legacyChunkID, err := q.UpsertChunk(ctx, sqlc.UpsertChunkParams{
		DocumentID:        legacyDocID,
		WorkspaceHash:     workspaceHash,
		ContentHash:       computeContentHash(legacyContent + "-chunk"),
		Content:           legacyContent,
		ChunkIndex:        0,
		SymbolName:        sql.NullString{String: symbol.Name, Valid: true},
		SymbolKind:        sql.NullString{String: symbol.Kind, Valid: true},
		ChunkType:         "text",
		EmbeddingStrategy: "default",
	})
	if err != nil {
		t.Fatalf("seed legacy UpsertChunk: %v", err)
	}

	if n := chunkCountForDocument(t, ctx, db, legacyDocID); n != 1 {
		t.Fatalf("precondition: expected 1 chunk for legacy doc, got %d", n)
	}

	svc := NewService(config.CodeSummarizationConfig{Model: "test-model"}, nil, nil, q, nil, testutil.NopLogger())

	if err := svc.upsertSummaryDocument(ctx, workspaceHash, symbol, "Fresh summary after fix"); err != nil {
		t.Fatalf("upsertSummaryDocument: %v", err)
	}

	// Legacy doc must be gone.
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents WHERE id = $1`, legacyDocID).Scan(&count); err != nil {
		t.Fatalf("check legacy doc gone: %v", err)
	}
	if count != 0 {
		t.Errorf("expected legacy doc %s to be deleted, but it still exists", legacyDocID)
	}

	// Legacy chunk must be gone too (FK cascade).
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM chunks WHERE id = $1`, legacyChunkID).Scan(&count); err != nil {
		t.Fatalf("check legacy chunk gone: %v", err)
	}
	if count != 0 {
		t.Errorf("expected legacy chunk %s to be deleted via cascade, but it still exists", legacyChunkID)
	}

	// Only the canonical no-hash doc should remain for this symbol.
	docs := listSummaryDocsForSymbol(t, ctx, db, workspaceHash, symbol.File, symbol.Name, symbol.Kind)
	if len(docs) != 1 {
		t.Fatalf("expected exactly 1 surviving summary doc, got %d: %+v", len(docs), docs)
	}
	if strings.Contains(docs[0].sourcePath, "&hash=") {
		t.Errorf("surviving doc source_path should not contain &hash=, got %q", docs[0].sourcePath)
	}
	if docs[0].content != "Fresh summary after fix" {
		t.Errorf("surviving doc content = %q, want %q", docs[0].content, "Fresh summary after fix")
	}

	canonicalDocID := documentIDBySourcePath(t, ctx, db, workspaceHash, docs[0].sourcePath)
	if n := chunkCountForDocument(t, ctx, db, canonicalDocID); n != 1 {
		t.Errorf("expected canonical doc to have exactly 1 chunk, got %d", n)
	}
}
