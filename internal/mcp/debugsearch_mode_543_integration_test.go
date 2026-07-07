//go:build integration

package mcp_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nano-brain/nano-brain/internal/config"
	internalmcp "github.com/nano-brain/nano-brain/internal/mcp"
	"github.com/nano-brain/nano-brain/internal/search"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
	"github.com/rs/zerolog"
)

// TestMemoryQuery_DebugMode_ResultsCarrySourceLabel is the MCP-layer
// regression test for #543 (PR-D): memory_query mode="debugging" advertises
// "parallel code/session/config searches with source labels" but DebugSearch
// discarded which leg each result came from before this fix. Verify every
// result item now carries a non-empty "source" in {"code","session","config"}
// when mode="debugging", and that a normal (non-debug) query omits "source"
// entirely.
func TestMemoryQuery_DebugMode_ResultsCarrySourceLabel(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { db.Close() })

	q := sqlc.New(db)
	wsHash := fmt.Sprintf("%x", sha256.Sum256([]byte("test_ws_"+uuid.New().String())))
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: wsHash, Name: "test-ws", Path: "/tmp/" + wsHash,
	}); err != nil {
		t.Fatalf("upsert workspace: %v", err)
	}

	now := time.Now().UTC()
	// Config leg doc: tagged ["config","memory"] so it satisfies the config
	// leg's tag filter (d.tags && ["config","memory"], see DebugSearch).
	testutil.SeedDocumentWithTimestamps(t, ctx, db, wsHash, "config-doc",
		"zolgrivenparakeet rate limiting configuration value", []string{"config", "memory"},
		now, now)
	// Plain doc, no tags — reachable via the code/session legs only.
	testutil.SeedDocumentWithTimestamps(t, ctx, db, wsHash, "code-doc",
		"zolgrivenparakeet function implementation logic", nil,
		now, now)

	searchSvc := search.NewSearchService(q, &mockEmbedder{}, config.SearchConfig{
		RrfK: 60, RecencyWeight: 0.3, RecencyHalfLifeDays: 180, Limit: 20,
	}, zerolog.Nop())

	server := internalmcp.NewMCPServer("test")
	adapter := internalmcp.NewAdapter(q, db, &mockEmbedder{}, searchSvc, nil, config.EmbeddingConfig{}, config.SearchConfig{}, config.FlowConfig{}, nil, zerolog.Nop())
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

	debugResp := callMCPTool(t, ctx, session, "memory_query", map[string]any{
		"workspace":   wsHash,
		"query":       "zolgrivenparakeet",
		"max_results": 20,
		"mode":        "debugging",
	})
	if debugResp.IsError {
		t.Fatalf("debug query error: %s", debugResp.Content[0].(*mcpsdk.TextContent).Text)
	}
	debugParsed := unmarshalQueryResp(t, debugResp)
	debugResults, _ := debugParsed["results"].([]interface{})
	if len(debugResults) == 0 {
		t.Fatal("expected at least 1 result in debugging mode")
	}
	validSources := map[string]bool{"code": true, "session": true, "config": true}
	for i, raw := range debugResults {
		item, ok := raw.(map[string]interface{})
		if !ok {
			t.Fatalf("result %d: unexpected shape %T", i, raw)
		}
		source, _ := item["source"].(string)
		if source == "" {
			t.Errorf("result %d: expected non-empty source in debugging mode, item=%v", i, item)
			continue
		}
		if !validSources[source] {
			t.Errorf("result %d: unexpected source %q, want one of code/session/config", i, source)
		}
	}

	normalResp := callMCPTool(t, ctx, session, "memory_query", map[string]any{
		"workspace":   wsHash,
		"query":       "zolgrivenparakeet",
		"max_results": 20,
	})
	if normalResp.IsError {
		t.Fatalf("normal query error: %s", normalResp.Content[0].(*mcpsdk.TextContent).Text)
	}
	normalParsed := unmarshalQueryResp(t, normalResp)
	normalResults, _ := normalParsed["results"].([]interface{})
	if len(normalResults) == 0 {
		t.Fatal("expected at least 1 result in normal (non-debug) mode")
	}
	for i, raw := range normalResults {
		item, ok := raw.(map[string]interface{})
		if !ok {
			t.Fatalf("result %d: unexpected shape %T", i, raw)
		}
		if source, present := item["source"]; present && source != "" {
			t.Errorf("result %d: normal (non-debug) query must omit source, got %v", i, source)
		}
	}
}
