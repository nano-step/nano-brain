//go:build integration

package watcher_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
	"github.com/nano-brain/nano-brain/internal/watcher"
	"github.com/rs/zerolog"
)

// TestReindexResolvesImportEdges_EndToEnd is the headline AC3+AC7 proof: it runs
// the REAL extract -> resolve -> UpsertGraphEdge path over the committed
// alias-import fixture (via ReextractEdgesForWorkspace, which walks every file
// unconditionally), then asserts reverse lookup on the RESOLVED relative target
// returns the importer (0 -> N), the RAW alias spec returns nothing, and a
// second reindex stays idempotent (no raw/resolved duplicates).
func TestReindexResolvesImportEdges_EndToEnd(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { db.Close() })

	q := sqlc.New(db)

	fixtureDir, err := filepath.Abs("../graph/testdata/alias-import")
	if err != nil {
		t.Fatalf("abs fixture dir: %v", err)
	}
	wsHash := fmt.Sprintf("%x", sha256.Sum256([]byte("import_e2e_"+uuid.New().String())))
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: wsHash, Name: "alias-import", Path: fixtureDir,
	}); err != nil {
		t.Fatalf("upsert workspace: %v", err)
	}

	// Build a watcher wired with the TS extractor, exactly like production.
	tsGE, err := graph.NewTypeScriptGraphExtractor()
	if err != nil {
		t.Fatalf("typescript extractor: %v", err)
	}
	registry := graph.NewRegistry(tsGE)

	w := watcher.New(db, q, zerolog.Nop(), config.Config{}).
		WithGraphRegistry(registry, q)
	if err := w.Watch("alias-import", fixtureDir, wsHash, "**/*.ts"); err != nil {
		t.Fatalf("watch fixture: %v", err)
	}

	const resolvedTarget = "utils/enums.ts"           // what ~/utils/enums must resolve to
	const importer = "composables/useThing.ts"        // the file doing the import
	const rawSpec = "~/utils/enums"                    // the unresolved alias form

	// --- First reindex: 0 -> N --------------------------------------------
	if n := w.ReextractEdgesForWorkspace(ctx, wsHash); n == 0 {
		t.Fatalf("ReextractEdgesForWorkspace processed 0 files")
	}

	incoming, err := q.GetIncomingEdges(ctx, sqlc.GetIncomingEdgesParams{
		WorkspaceHash: wsHash,
		TargetNode:    resolvedTarget,
		Column3:       "imports",
	})
	if err != nil {
		t.Fatalf("GetIncomingEdges(resolved): %v", err)
	}
	if len(incoming) != 1 {
		t.Fatalf("reverse lookup on %q returned %d edges, want 1 (the importer)", resolvedTarget, len(incoming))
	}
	if incoming[0].SourceNode != importer {
		t.Errorf("importer source_node = %q, want %q", incoming[0].SourceNode, importer)
	}
	if incoming[0].TargetNode != resolvedTarget {
		t.Errorf("target_node = %q, want resolved %q", incoming[0].TargetNode, resolvedTarget)
	}

	// The raw alias spec must NOT be a stored target anymore (it was resolved).
	rawHits, err := q.GetIncomingEdges(ctx, sqlc.GetIncomingEdgesParams{
		WorkspaceHash: wsHash,
		TargetNode:    rawSpec,
		Column3:       "imports",
	})
	if err != nil {
		t.Fatalf("GetIncomingEdges(raw): %v", err)
	}
	if len(rawHits) != 0 {
		t.Errorf("raw alias spec %q still has %d incoming edges, want 0 (should be resolved)", rawSpec, len(rawHits))
	}

	// --- Second reindex: idempotency (AC7) --------------------------------
	if n := w.ReextractEdgesForWorkspace(ctx, wsHash); n == 0 {
		t.Fatalf("second ReextractEdgesForWorkspace processed 0 files")
	}
	incoming2, err := q.GetIncomingEdges(ctx, sqlc.GetIncomingEdgesParams{
		WorkspaceHash: wsHash,
		TargetNode:    resolvedTarget,
		Column3:       "imports",
	})
	if err != nil {
		t.Fatalf("GetIncomingEdges after 2nd reindex: %v", err)
	}
	if len(incoming2) != 1 {
		t.Fatalf("after 2nd reindex: count=%d, want 1 (delete-by-source_file must prevent duplicates)", len(incoming2))
	}
}
