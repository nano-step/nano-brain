//go:build integration

package mcp_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"math"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	pgvector_go "github.com/pgvector/pgvector-go"
	"github.com/rs/zerolog"
	"github.com/sqlc-dev/pqtype"

	"github.com/nano-brain/nano-brain/internal/config"
	internalmcp "github.com/nano-brain/nano-brain/internal/mcp"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
)

// Regression coverage for #545: memory_vsearch under-retrieval on
// multi-concept queries. Root cause: with group_by="document" the handler
// fetched only offset+max_results+1 CHUNKS (no similarity threshold) before
// collapsing chunks -> documents via deduplicateByDocument. When the top
// chunks cluster into a handful of documents, dedup yields far fewer than
// max_results distinct documents. The fix over-fetches chunks before dedup
// (vsearchDedupOverFetchFactor/Cap in internal/mcp/tools.go) so dedup has
// enough candidates to draw from.

const vsearchOverfetchVecDim = 768

// vsearchFixedEmbedder always returns a caller-supplied vector, so tests can
// pin the query embedding and construct chunk embeddings with a known cosine
// similarity to it, independent of any real embedding provider.
type vsearchFixedEmbedder struct{ vec []float32 }

func (f *vsearchFixedEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return f.vec, nil
}
func (f *vsearchFixedEmbedder) Dimension() int { return vsearchOverfetchVecDim }

// vsearchVec builds a unit vector [alpha, beta, 0, ..., 0] such that its
// cosine similarity against the fixed query vector [1, 0, ..., 0] is exactly
// alpha (beta fills out the remaining norm so the vector stays unit length;
// its exact placement doesn't affect the dot product with the query).
func vsearchVec(alpha float32) []float32 {
	v := make([]float32, vsearchOverfetchVecDim)
	v[0] = alpha
	beta := 1 - float64(alpha)*float64(alpha)
	if beta < 0 {
		beta = 0
	}
	v[1] = float32(math.Sqrt(beta))
	return v
}

// setupVSearchMCP wires an MCP server/client pair over an isolated
// nanobrain_test schema with a fixed-vector embedder so memory_vsearch
// exercises its real DB query path deterministically.
func setupVSearchMCP(t *testing.T) (context.Context, *sqlc.Queries, string, func(string, map[string]any) *mcpsdk.CallToolResult) {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { db.Close() })

	queries := sqlc.New(db)
	wsHash := fmt.Sprintf("%x", sha256.Sum256([]byte("test_ws_vsearch_overfetch_"+uuid.New().String())))
	if _, err := queries.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: wsHash, Name: "test-ws-vsearch-overfetch", Path: "/tmp/" + wsHash,
	}); err != nil {
		t.Fatalf("upsert workspace: %v", err)
	}

	embedder := &vsearchFixedEmbedder{vec: vsearchVec(1.0)} // query vector = [1,0,...,0]
	server := internalmcp.NewMCPServer("test")
	adapter := internalmcp.NewAdapter(queries, db, embedder, nil, nil, config.EmbeddingConfig{}, config.SearchConfig{}, config.FlowConfig{}, nil, zerolog.Nop())
	internalmcp.RegisterTools(server, adapter)

	ct, st := mcpsdk.NewInMemoryTransports()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })

	callTool := func(name string, args map[string]any) *mcpsdk.CallToolResult {
		t.Helper()
		result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("CallTool(%s): %v", name, err)
		}
		return result
	}
	return ctx, queries, wsHash, callTool
}

// seedVSearchDoc inserts one document plus one chunk (with a pinned
// embedding) per entry in alphas.
func seedVSearchDoc(t *testing.T, ctx context.Context, q *sqlc.Queries, wsHash, title string, alphas []float32) {
	t.Helper()
	content := "content body for " + title
	doc, err := q.UpsertDocument(ctx, sqlc.UpsertDocumentParams{
		WorkspaceHash: wsHash,
		ContentHash:   contentHash(content + wsHash + title),
		Title:         title,
		Content:       content,
		SourcePath:    "/tmp/" + title + ".md",
		Collection:    "memory",
		Tags:          []string{},
		Metadata:      pqtype.NullRawMessage{},
	})
	if err != nil {
		t.Fatalf("UpsertDocument(%s): %v", title, err)
	}

	for i, alpha := range alphas {
		chunkContent := fmt.Sprintf("%s chunk %d", title, i)
		chunkID, err := q.UpsertChunk(ctx, sqlc.UpsertChunkParams{
			DocumentID:        doc.ID,
			WorkspaceHash:     wsHash,
			ContentHash:       contentHash(chunkContent + wsHash + title + fmt.Sprintf("%d", i)),
			Content:           chunkContent,
			ChunkIndex:        int32(i),
			StartLine:         sql.NullInt32{Int32: 1, Valid: true},
			EndLine:           sql.NullInt32{Int32: 10, Valid: true},
			Metadata:          pqtype.NullRawMessage{},
			ChunkType:         "raw",
			EmbeddingStrategy: "raw_code",
		})
		if err != nil {
			t.Fatalf("UpsertChunk(%s, %d): %v", title, i, err)
		}
		if _, err := q.InsertEmbedding(ctx, sqlc.InsertEmbeddingParams{
			ChunkID:       chunkID,
			WorkspaceHash: wsHash,
			Provider:      "test",
			Model:         "test-model",
			Embedding:     pgvector_go.NewVector(vsearchVec(alpha)),
		}); err != nil {
			t.Fatalf("InsertEmbedding(%s, %d): %v", title, i, err)
		}
	}
}

// TestMemoryVSearch_OverFetch_FewDocumentsCollapse reproduces #545: two "hot"
// documents contribute 5 near-identical, highly-similar chunks each (10
// chunks total, alpha 0.99/0.98) while 10 "cold" documents contribute a
// single, less-similar chunk each (alpha 0.50 down to 0.41). Pre-fix, the
// handler fetched only offset+max_results+1=8 chunks before deduplicating by
// document; since the top 8 chunks by score are entirely hot-document
// chunks, dedup collapsed the result to 2 documents even though max_results
// was 7 and 12 distinct documents exist. Post-fix, the over-fetch (x5,
// capped at 200) pulls in all 20 chunks, so dedup has every document
// available and the page fills to the requested max_results.
func TestMemoryVSearch_OverFetch_FewDocumentsCollapse(t *testing.T) {
	ctx, q, wsHash, callTool := setupVSearchMCP(t)

	seedVSearchDoc(t, ctx, q, wsHash, "hot-doc-0", []float32{0.99, 0.99, 0.99, 0.99, 0.99})
	seedVSearchDoc(t, ctx, q, wsHash, "hot-doc-1", []float32{0.98, 0.98, 0.98, 0.98, 0.98})
	coldAlphas := []float32{0.50, 0.49, 0.48, 0.47, 0.46, 0.45, 0.44, 0.43, 0.42, 0.41}
	for i, alpha := range coldAlphas {
		seedVSearchDoc(t, ctx, q, wsHash, fmt.Sprintf("cold-doc-%d", i), []float32{alpha})
	}

	result := callTool("memory_vsearch", map[string]any{
		"workspace": wsHash, "query": "probe concept", "max_results": 7,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].(*mcpsdk.TextContent).Text)
	}

	resp, _ := parseSearchResponse(t, result)
	if len(resp.Results) != 7 {
		t.Fatalf("got %d results, want 7 (over-fetch should fill the page, not collapse to 2)", len(resp.Results))
	}
	if resp.Total != 12 {
		t.Fatalf("total = %d, want 12 distinct documents (2 hot + 10 cold)", resp.Total)
	}

	items := parseResultItems(t, resp)
	seenDocs := make(map[string]bool, len(items))
	for _, item := range items {
		if seenDocs[item.DocumentID] {
			t.Errorf("duplicate document_id %s in a single group_by=document page", item.DocumentID)
		}
		seenDocs[item.DocumentID] = true
	}
	if len(seenDocs) != 7 {
		t.Fatalf("page contains %d distinct documents, want 7 (not collapsed to 2 hot documents)", len(seenDocs))
	}
}

// TestMemoryVSearch_ManyDocumentsAlreadyDiverse checks the non-collapsing
// case still behaves correctly after the fix: 10 documents each contribute
// exactly one chunk at a distinct similarity, so no over-fetch is needed to
// reach max_results distinct documents.
func TestMemoryVSearch_ManyDocumentsAlreadyDiverse(t *testing.T) {
	ctx, q, wsHash, callTool := setupVSearchMCP(t)

	alphas := []float32{0.99, 0.90, 0.80, 0.70, 0.60, 0.50, 0.40, 0.30, 0.20, 0.10}
	for i, alpha := range alphas {
		seedVSearchDoc(t, ctx, q, wsHash, fmt.Sprintf("diverse-doc-%d", i), []float32{alpha})
	}

	result := callTool("memory_vsearch", map[string]any{
		"workspace": wsHash, "query": "probe concept", "max_results": 7,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].(*mcpsdk.TextContent).Text)
	}

	resp, _ := parseSearchResponse(t, result)
	if len(resp.Results) != 7 {
		t.Fatalf("got %d results, want 7", len(resp.Results))
	}
	if resp.Total != 10 {
		t.Fatalf("total = %d, want 10 distinct documents", resp.Total)
	}

	items := parseResultItems(t, resp)
	seenDocs := make(map[string]bool, len(items))
	for _, item := range items {
		seenDocs[item.DocumentID] = true
	}
	if len(seenDocs) != 7 {
		t.Fatalf("page contains %d distinct documents, want 7", len(seenDocs))
	}
}

// TestMemoryVSearch_GroupByNone_FetchLimitUnaffected asserts the fix is
// scoped to group_by="document": when grouping is off, each chunk is its
// own result and the fetch limit must stay at offset+max_results+1 (no
// over-fetch), matching pre-fix behavior exactly.
func TestMemoryVSearch_GroupByNone_FetchLimitUnaffected(t *testing.T) {
	ctx, q, wsHash, callTool := setupVSearchMCP(t)

	seedVSearchDoc(t, ctx, q, wsHash, "hot-doc-0", []float32{0.99, 0.99, 0.99, 0.99, 0.99})
	seedVSearchDoc(t, ctx, q, wsHash, "hot-doc-1", []float32{0.98, 0.98, 0.98, 0.98, 0.98})
	coldAlphas := []float32{0.50, 0.49, 0.48, 0.47, 0.46, 0.45, 0.44, 0.43, 0.42, 0.41}
	for i, alpha := range coldAlphas {
		seedVSearchDoc(t, ctx, q, wsHash, fmt.Sprintf("cold-doc-%d", i), []float32{alpha})
	}

	result := callTool("memory_vsearch", map[string]any{
		"workspace": wsHash, "query": "probe concept", "max_results": 7, "group_by": "none",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].(*mcpsdk.TextContent).Text)
	}

	resp, _ := parseSearchResponse(t, result)
	if len(resp.Results) != 7 {
		t.Fatalf("got %d results, want 7", len(resp.Results))
	}
	// offset(0)+max_results(7)+1 = 8 chunk-level rows fetched, unaffected by
	// the group_by=document over-fetch factor.
	if resp.Total != 8 {
		t.Fatalf("total = %d, want 8 (fetch limit must stay at offset+max_results+1 when ungrouped)", resp.Total)
	}
}
