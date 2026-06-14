//go:build integration

package mcp_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
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

func TestTimeFilter_MCP_ValidRelativeDuration(t *testing.T) {
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
	testutil.SeedDocumentWithTimestamps(t, ctx, db, wsHash, "doc-5d", "content for 5 days ago", nil, now.AddDate(0, 0, -5), now.AddDate(0, 0, -5))
	testutil.SeedDocumentWithTimestamps(t, ctx, db, wsHash, "doc-20d", "content for 20 days ago", nil, now.AddDate(0, 0, -20), now.AddDate(0, 0, -20))
	testutil.SeedDocumentWithTimestamps(t, ctx, db, wsHash, "doc-60d", "content for 60 days ago", nil, now.AddDate(0, 0, -60), now.AddDate(0, 0, -60))

	server := internalmcp.NewMCPServer("test")
	adapter := internalmcp.NewAdapter(q, db, nil, nil, nil, config.EmbeddingConfig{}, config.SearchConfig{}, config.FlowConfig{}, nil, zerolog.Nop())
	internalmcp.RegisterTools(server, adapter)

	ct, st := mcpsdk.NewInMemoryTransports()
	_, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "memory_search",
		Arguments: map[string]any{
			"workspace":     wsHash,
			"query":         "content",
			"updated_after": "30d",
			"max_results":   100,
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].(*mcpsdk.TextContent).Text)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].(*mcpsdk.TextContent).Text), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	totalFloat, ok := resp["total"].(float64)
	if !ok {
		t.Fatalf("total field not a number")
	}
	total := int(totalFloat)

	if total != 2 {
		t.Errorf("expected 2 results (5d and 20d), got %d", total)
	}
}

func TestTimeFilter_MCP_RFC3339CreatedAfter(t *testing.T) {
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
	testutil.SeedDocumentWithTimestamps(t, ctx, db, wsHash, "doc-old", "old content", nil, now.AddDate(0, 0, -60), now)
	testutil.SeedDocumentWithTimestamps(t, ctx, db, wsHash, "doc-recent", "recent content", nil, now.AddDate(0, 0, -10), now)

	server := internalmcp.NewMCPServer("test")
	adapter := internalmcp.NewAdapter(q, db, nil, nil, nil, config.EmbeddingConfig{}, config.SearchConfig{}, config.FlowConfig{}, nil, zerolog.Nop())
	internalmcp.RegisterTools(server, adapter)

	ct, st := mcpsdk.NewInMemoryTransports()
	_, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })

	thirtyDaysAgo := now.AddDate(0, 0, -30).Format(time.RFC3339)

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "memory_search",
		Arguments: map[string]any{
			"workspace":      wsHash,
			"query":          "content",
			"created_after":  thirtyDaysAgo,
			"max_results":    100,
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].(*mcpsdk.TextContent).Text)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].(*mcpsdk.TextContent).Text), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	totalFloat, ok := resp["total"].(float64)
	if !ok {
		t.Fatalf("total field not a number")
	}
	total := int(totalFloat)

	if total != 1 {
		t.Errorf("expected 1 result (only recent), got %d", total)
	}
}

func TestTimeFilter_MCP_AllFourFiltersCombined(t *testing.T) {
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
	docA := map[string]time.Time{
		"created_at": now.AddDate(0, 0, -50),
		"updated_at": now.AddDate(0, 0, -5),
	}
	docB := map[string]time.Time{
		"created_at": now.AddDate(0, 0, -100),
		"updated_at": now.AddDate(0, 0, -5),
	}

	testutil.SeedDocumentWithTimestamps(t, ctx, db, wsHash, "doc-a", "doc a text", nil, docA["created_at"], docA["updated_at"])
	testutil.SeedDocumentWithTimestamps(t, ctx, db, wsHash, "doc-b", "doc b text", nil, docB["created_at"], docB["updated_at"])

	server := internalmcp.NewMCPServer("test")
	adapter := internalmcp.NewAdapter(q, db, nil, nil, nil, config.EmbeddingConfig{}, config.SearchConfig{}, config.FlowConfig{}, nil, zerolog.Nop())
	internalmcp.RegisterTools(server, adapter)

	ct, st := mcpsdk.NewInMemoryTransports()
	_, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })

	createdAfter90d := now.AddDate(0, 0, -90).Format(time.RFC3339)
	createdBefore30d := now.AddDate(0, 0, -30).Format(time.RFC3339)

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "memory_search",
		Arguments: map[string]any{
			"workspace":       wsHash,
			"query":           "doc",
			"created_after":   createdAfter90d,
			"created_before":  createdBefore30d,
			"updated_after":   "30d",
			"updated_before":  now.Format(time.RFC3339),
			"max_results":     100,
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].(*mcpsdk.TextContent).Text)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].(*mcpsdk.TextContent).Text), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	totalFloat, ok := resp["total"].(float64)
	if !ok {
		t.Fatalf("total field not a number")
	}
	total := int(totalFloat)

	if total != 1 {
		t.Errorf("expected 1 result (only doc-a matches all filters), got %d", total)
	}
}

func TestTimeFilter_MCP_InvalidDurationReturnsError(t *testing.T) {
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

	server := internalmcp.NewMCPServer("test")
	adapter := internalmcp.NewAdapter(q, db, nil, nil, nil, config.EmbeddingConfig{}, config.SearchConfig{}, config.FlowConfig{}, nil, zerolog.Nop())
	internalmcp.RegisterTools(server, adapter)

	ct, st := mcpsdk.NewInMemoryTransports()
	_, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "memory_search",
		Arguments: map[string]any{
			"workspace":     wsHash,
			"query":         "test",
			"updated_after": "banana",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	if !result.IsError {
		t.Fatalf("expected error result for invalid duration")
	}

	errText := result.Content[0].(*mcpsdk.TextContent).Text

	if !strings.Contains(errText, "updated_after") || !strings.Contains(errText, "banana") {
		t.Errorf("expected error to mention 'updated_after' and 'banana', got: %s", errText)
	}
}

func TestTimeFilter_MCP_DateOnlyRejected(t *testing.T) {
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

	server := internalmcp.NewMCPServer("test")
	adapter := internalmcp.NewAdapter(q, db, nil, nil, nil, config.EmbeddingConfig{}, config.SearchConfig{}, config.FlowConfig{}, nil, zerolog.Nop())
	internalmcp.RegisterTools(server, adapter)

	ct, st := mcpsdk.NewInMemoryTransports()
	_, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "memory_search",
		Arguments: map[string]any{
			"workspace":     wsHash,
			"query":         "test",
			"updated_after": "2026-05-04",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	if !result.IsError {
		t.Fatalf("expected error result for date-only format")
	}

	errText := result.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(errText, "updated_after") {
		t.Errorf("expected error to mention 'updated_after', got: %s", errText)
	}
}

func TestTimeFilter_MCP_NegativeDurationRejected(t *testing.T) {
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

	server := internalmcp.NewMCPServer("test")
	adapter := internalmcp.NewAdapter(q, db, nil, nil, nil, config.EmbeddingConfig{}, config.SearchConfig{}, config.FlowConfig{}, nil, zerolog.Nop())
	internalmcp.RegisterTools(server, adapter)

	ct, st := mcpsdk.NewInMemoryTransports()
	_, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "memory_search",
		Arguments: map[string]any{
			"workspace":     wsHash,
			"query":         "test",
			"updated_after": "-30d",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	if !result.IsError {
		t.Fatalf("expected error result for negative duration")
	}

	errText := result.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(errText, "updated_after") {
		t.Errorf("expected error to mention 'updated_after', got: %s", errText)
	}
}

func TestTimeFilter_MCP_InvertedRangeReturnsEmpty(t *testing.T) {
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
	testutil.SeedDocumentWithTimestamps(t, ctx, db, wsHash, "doc", "test content", nil, now.AddDate(0, 0, -30), now.AddDate(0, 0, -30))

	server := internalmcp.NewMCPServer("test")
	adapter := internalmcp.NewAdapter(q, db, nil, nil, nil, config.EmbeddingConfig{}, config.SearchConfig{}, config.FlowConfig{}, nil, zerolog.Nop())
	internalmcp.RegisterTools(server, adapter)

	ct, st := mcpsdk.NewInMemoryTransports()
	_, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })

	after := now.AddDate(0, 0, -10).Format(time.RFC3339)
	before := now.AddDate(0, 0, -20).Format(time.RFC3339)

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "memory_search",
		Arguments: map[string]any{
			"workspace":      wsHash,
			"query":          "content",
			"updated_after":  after,
			"updated_before": before,
			"max_results":    100,
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].(*mcpsdk.TextContent).Text)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].(*mcpsdk.TextContent).Text), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	totalFloat, ok := resp["total"].(float64)
	if !ok {
		t.Fatalf("total field not a number")
	}
	total := int(totalFloat)

	if total != 0 {
		t.Errorf("expected 0 results for inverted range, got %d", total)
	}
}

func TestTimeFilter_MCP_NoMatchReturnsEmpty(t *testing.T) {
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
	testutil.SeedDocumentWithTimestamps(t, ctx, db, wsHash, "doc", "old content", nil, now.AddDate(0, -1, 0), now.AddDate(0, -1, 0))

	server := internalmcp.NewMCPServer("test")
	adapter := internalmcp.NewAdapter(q, db, nil, nil, nil, config.EmbeddingConfig{}, config.SearchConfig{}, config.FlowConfig{}, nil, zerolog.Nop())
	internalmcp.RegisterTools(server, adapter)

	ct, st := mcpsdk.NewInMemoryTransports()
	_, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "memory_search",
		Arguments: map[string]any{
			"workspace":     wsHash,
			"query":         "content",
			"updated_after": "7d",
			"max_results":   100,
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].(*mcpsdk.TextContent).Text)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].(*mcpsdk.TextContent).Text), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	totalFloat, ok := resp["total"].(float64)
	if !ok {
		t.Fatalf("total field not a number")
	}
	total := int(totalFloat)

	if total != 0 {
		t.Errorf("expected 0 results, got %d", total)
	}
}

func TestTimeFilter_MCP_VectorSearchWithTimeFilter(t *testing.T) {
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
	testutil.SeedDocumentWithTimestamps(t, ctx, db, wsHash, "doc-recent", "vector search test", nil, now.AddDate(0, 0, -5), now.AddDate(0, 0, -5))
	testutil.SeedDocumentWithTimestamps(t, ctx, db, wsHash, "doc-old", "vector search old", nil, now.AddDate(0, 0, -100), now.AddDate(0, 0, -100))

	server := internalmcp.NewMCPServer("test")
	adapter := internalmcp.NewAdapter(q, db, nil, nil, nil, config.EmbeddingConfig{}, config.SearchConfig{}, config.FlowConfig{}, nil, zerolog.Nop())
	internalmcp.RegisterTools(server, adapter)

	ct, st := mcpsdk.NewInMemoryTransports()
	_, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "memory_vsearch",
		Arguments: map[string]any{
			"workspace":     wsHash,
			"query":         "vector search",
			"updated_after": "30d",
			"max_results":   100,
		},
	})
	if err != nil {
		t.Logf("vsearch not available or embedding provider not configured: %v", err)
		t.Skip("vsearch requires embedding provider")
	}

	if result.IsError {
		if strings.Contains(result.Content[0].(*mcpsdk.TextContent).Text, "embedding provider") {
			t.Skip("embedding provider not configured")
		}
		t.Fatalf("unexpected error: %s", result.Content[0].(*mcpsdk.TextContent).Text)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].(*mcpsdk.TextContent).Text), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if total, ok := resp["total"].(float64); ok && total >= 0 {
		if total > 1 {
			t.Errorf("expected at most 1 result (only recent doc), got %d", int(total))
		}
	}
}

// TestTimeFilter_MCP_QueryPaginationWithTimeFilter regression-tests the bug where
// memory_query verified the cursor BEFORE parsing time filters, so the cursor hash
// computed on encode (with TimeRange set) did not match the one used on verify
// (with TimeRange nil). Fix: parse time filters first, then build hashInput.
// See docs/evidence/review-360.md F1.
func TestTimeFilter_MCP_QueryPaginationWithTimeFilter(t *testing.T) {
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
	for i := 0; i < 12; i++ {
		testutil.SeedDocumentWithTimestamps(
			t, ctx, db, wsHash,
			"page-doc-"+uuid.New().String()[:6],
			"golf keyword item searchable content "+uuid.New().String()[:4],
			nil,
			now.AddDate(0, 0, -i),
			now.AddDate(0, 0, -i),
		)
	}

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

	args := map[string]any{
		"workspace":     wsHash,
		"query":         "golf",
		"max_results":   5,
		"updated_after": "30d",
	}

	page1 := callMCPTool(t, ctx, session, "memory_query", args)
	if page1.IsError {
		t.Fatalf("page1 error: %s", page1.Content[0].(*mcpsdk.TextContent).Text)
	}
	resp1 := unmarshalQueryResp(t, page1)
	nextCursor, _ := resp1["next_cursor"].(string)
	if nextCursor == "" {
		t.Fatalf("page1: expected non-empty next_cursor (>5 docs in window), got empty. resp=%v", resp1)
	}

	args["cursor"] = nextCursor
	page2 := callMCPTool(t, ctx, session, "memory_query", args)
	if page2.IsError {
		text := page2.Content[0].(*mcpsdk.TextContent).Text
		if strings.Contains(text, "cursor query mismatch") {
			t.Fatalf("BUG REGRESSION: cursor query mismatch on page 2 with same time filter — F1 fix is broken. err=%s", text)
		}
		t.Fatalf("page2 unexpected error: %s", text)
	}
	resp2 := unmarshalQueryResp(t, page2)
	if results, ok := resp2["results"].([]interface{}); ok && len(results) == 0 {
		t.Errorf("page2: expected at least one result, got empty (cursor probably advanced past data)")
	}
}

func callMCPTool(t *testing.T, ctx context.Context, session *mcpsdk.ClientSession, name string, args map[string]any) *mcpsdk.CallToolResult {
	t.Helper()
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	return result
}

func unmarshalQueryResp(t *testing.T, result *mcpsdk.CallToolResult) map[string]interface{} {
	t.Helper()
	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].(*mcpsdk.TextContent).Text), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return resp
}
