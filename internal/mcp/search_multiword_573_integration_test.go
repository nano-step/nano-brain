//go:build integration

package mcp_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nano-brain/nano-brain/internal/config"
	internalmcp "github.com/nano-brain/nano-brain/internal/mcp"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
	"github.com/rs/zerolog"
)

// Issue #573 (#542 F9): memory_search uses websearch_to_tsquery (ANDs terms), so
// a multi-word query where no chunk contains every term returns 0 — worse under
// chunk_type. The OR-fallback must rescue it. This test seeds a symbol chunk,
// populates its search_vector (UpsertChunk doesn't), and searches a multi-word
// phrase whose AND misses but OR hits.
func TestMemorySearch_MultiWordORFallback(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { db.Close() })
	q := sqlc.New(db)

	wsHash := fmt.Sprintf("%x", sha256.Sum256([]byte("f9_ws_"+uuid.New().String())))
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{Hash: wsHash, Name: "f9-ws", Path: "/tmp/f9-" + uuid.New().String()[:8]}); err != nil {
		t.Fatalf("upsert workspace: %v", err)
	}
	doc, err := q.UpsertDocument(ctx, sqlc.UpsertDocumentParams{
		WorkspaceHash: wsHash, ContentHash: "f9-d", Title: "depositController",
		Content: "deposit account balance controller", SourcePath: "svc.go?symbol=depositController", Collection: "code",
	})
	if err != nil {
		t.Fatalf("upsert document: %v", err)
	}
	if _, err := q.UpsertChunk(ctx, sqlc.UpsertChunkParams{
		DocumentID: doc.ID, WorkspaceHash: wsHash, ContentHash: "f9-c",
		Content: "deposit account balance controller", ChunkIndex: 0,
		ChunkType: "symbol", EmbeddingStrategy: "none",
	}); err != nil {
		t.Fatalf("upsert chunk: %v", err)
	}
	// search_vector is app-populated (UpsertChunk leaves it NULL); set it as the
	// indexer would so BM25 can match.
	if _, err := db.ExecContext(ctx, "UPDATE chunks SET search_vector = to_tsvector(get_tsvector_config(), content) WHERE workspace_hash=$1", wsHash); err != nil {
		t.Fatalf("set search_vector: %v", err)
	}

	server := internalmcp.NewMCPServer("test")
	adapter := internalmcp.NewAdapter(q, db, nil, nil, nil, config.EmbeddingConfig{}, config.SearchConfig{Limit: 20}, config.FlowConfig{}, nil, zerolog.Nop())
	internalmcp.RegisterTools(server, adapter)
	ct, st := mcpsdk.NewInMemoryTransports()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "t", Version: "v1"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	callTool := func(name string, args map[string]any) *mcpsdk.CallToolResult {
		r, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("CallTool(%s): %v", name, err)
		}
		return r
	}

	// Multi-word phrase whose AND misses (chunk lacks "zzzmissing"), chunk_type=symbol.
	resp := unmarshalGraphResp(t, callTool("memory_search", map[string]any{
		"workspace": wsHash, "query": "deposit zzzmissing balance", "chunk_type": "symbol",
	}))
	results, _ := resp["results"].([]any)
	if len(results) == 0 {
		t.Fatalf("multi-word search returned 0 — OR fallback did not rescue the AND miss: %+v", resp)
	}
	var sawDoc bool
	for _, r := range results {
		if r.(map[string]any)["title"] == "depositController" {
			sawDoc = true
		}
	}
	if !sawDoc {
		t.Errorf("expected depositController via OR fallback; got %+v", results)
	}
}
