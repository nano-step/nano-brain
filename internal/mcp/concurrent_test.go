package mcp_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	internalmcp "github.com/nano-brain/nano-brain/internal/mcp"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/search"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

// --- Fake database/sql driver (stdlib only) ---
// Returns errors on all operations so sqlc-generated code fails gracefully
// instead of panicking on nil pointers.

var errFakeDB = errors.New("fakedb: not a real database")

func init() {
	sql.Register("fakedb", &fakeDriver{})
}

type fakeDriver struct{}

func (d *fakeDriver) Open(_ string) (driver.Conn, error) { return &fakeConn{}, nil }

type fakeConn struct{}

func (c *fakeConn) Prepare(_ string) (driver.Stmt, error) { return nil, errFakeDB }
func (c *fakeConn) Close() error                          { return nil }
func (c *fakeConn) Begin() (driver.Tx, error)             { return nil, errFakeDB }

// --- Mock search.Querier (returns empty results, no DB needed) ---

type mockSearchQuerier struct{}

func (m *mockSearchQuerier) BM25Search(_ context.Context, _ sqlc.BM25SearchParams) ([]sqlc.BM25SearchRow, error) {
	return nil, nil
}
func (m *mockSearchQuerier) BM25SearchAll(_ context.Context, _ sqlc.BM25SearchAllParams) ([]sqlc.BM25SearchAllRow, error) {
	return nil, nil
}
func (m *mockSearchQuerier) BM25SearchWithTags(_ context.Context, _ sqlc.BM25SearchWithTagsParams) ([]sqlc.BM25SearchWithTagsRow, error) {
	return nil, nil
}
func (m *mockSearchQuerier) BM25SearchAllWithTags(_ context.Context, _ sqlc.BM25SearchAllWithTagsParams) ([]sqlc.BM25SearchAllWithTagsRow, error) {
	return nil, nil
}
func (m *mockSearchQuerier) VectorSearch(_ context.Context, _ sqlc.VectorSearchParams) ([]sqlc.VectorSearchRow, error) {
	return nil, nil
}
func (m *mockSearchQuerier) VectorSearchAll(_ context.Context, _ sqlc.VectorSearchAllParams) ([]sqlc.VectorSearchAllRow, error) {
	return nil, nil
}
func (m *mockSearchQuerier) VectorSearchWithTags(_ context.Context, _ sqlc.VectorSearchWithTagsParams) ([]sqlc.VectorSearchWithTagsRow, error) {
	return nil, nil
}
func (m *mockSearchQuerier) VectorSearchAllWithTags(_ context.Context, _ sqlc.VectorSearchAllWithTagsParams) ([]sqlc.VectorSearchAllWithTagsRow, error) {
	return nil, nil
}

// --- Mock embed.Embedder (returns zero vector) ---

type mockEmbedder struct{}

func (m *mockEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return make([]float32, 384), nil
}
func (m *mockEmbedder) Dimension() int { return 384 }

// --- Mock mcp.PoolChecker ---

type mockPoolChecker struct{}

func (m *mockPoolChecker) Ping(_ context.Context) error { return nil }

// --- Mock mcp.EmbedQueueInfo ---

type mockEmbedQueueInfo struct{}

func (m *mockEmbedQueueInfo) Depth() int        { return 0 }
func (m *mockEmbedQueueInfo) Capacity() int     { return 100 }
func (m *mockEmbedQueueInfo) Status() string     { return "idle" }
func (m *mockEmbedQueueInfo) PendingCount() int64 { return 0 }

// setupMockedTestClient creates an MCP client+server with real mocked
// services so tool handlers reach their actual logic paths instead of
// bailing at nil checks.
func setupMockedTestClient(t *testing.T) (*mcpsdk.ClientSession, context.Context) {
	t.Helper()

	db, err := sql.Open("fakedb", "")
	if err != nil {
		t.Fatalf("open fakedb: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	queries := sqlc.New(db)
	embedder := &mockEmbedder{}
	searchSvc := search.NewSearchService(&mockSearchQuerier{}, embedder, config.SearchConfig{
		RrfK:                60,
		RecencyWeight:       0.1,
		RecencyHalfLifeDays: 7,
		Limit:               10,
	}, zerolog.Nop())

	server := internalmcp.NewMCPServer("test")
	adapter := internalmcp.NewAdapter(
		queries, db, embedder, searchSvc, &mockEmbedQueueInfo{},
		config.EmbeddingConfig{Provider: "mock"},
		config.SearchConfig{},
		config.FlowConfig{},
		&mockPoolChecker{},
		zerolog.Nop(),
	)
	internalmcp.RegisterTools(server, adapter)

	ctx := context.Background()
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
	return session, ctx
}

func TestToolRegistration_ListToolsUnderRaceDetector(t *testing.T) {
	server := internalmcp.NewMCPServer("test")
	adapter := internalmcp.NewAdapter(nil, nil, nil, nil, nil, config.EmbeddingConfig{}, config.SearchConfig{}, config.FlowConfig{}, nil, zerolog.Nop())
	internalmcp.RegisterTools(server, adapter)

	ctx := context.Background()
	ct, st := mcpsdk.NewInMemoryTransports()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer session.Close()

	result, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(result.Tools) != 15 {
		t.Errorf("expected 15 tools, got %d", len(result.Tools))
	}
}

// TestConcurrentToolCalls_NoRace fires 20 goroutines that each invoke
// different tool handlers concurrently via mocked services. Handlers
// reach their actual logic paths (search, embed, DB) — proving no
// data race under `go test -race`.
func TestConcurrentToolCalls_NoRace(t *testing.T) {
	session, ctx := setupMockedTestClient(t)

	// All 9 handlers exercised with valid workspaces.
	// memory_query → full HybridSearch (BM25 + vector in parallel via errgroup)
	// memory_search → BM25Search via fakedb (returns SQL error gracefully)
	// memory_vsearch → Embed then VectorSearch via fakedb
	// memory_write → chunk.Split + BeginTx via fakedb (fails at tx, not nil)
	// memory_tags, memory_wake_up → query via fakedb
	// memory_status → reads pool/queue/config fields
	calls := []mcpsdk.CallToolParams{
		{Name: "memory_status", Arguments: map[string]any{}},
		{Name: "memory_update", Arguments: map[string]any{"workspace": "ws-1"}},
		{Name: "memory_get", Arguments: map[string]any{"workspace": "ws-1", "path": "doc-1"}},
		{Name: "memory_write", Arguments: map[string]any{"workspace": "ws-1", "content": "concurrent write test"}},
		{Name: "memory_query", Arguments: map[string]any{"workspace": "ws-1", "query": "concurrent search"}},
		{Name: "memory_search", Arguments: map[string]any{"workspace": "ws-1", "query": "bm25 test"}},
		{Name: "memory_vsearch", Arguments: map[string]any{"workspace": "ws-1", "query": "vector test"}},
		{Name: "memory_tags", Arguments: map[string]any{"workspace": "ws-1"}},
		{Name: "memory_wake_up", Arguments: map[string]any{"workspace": "ws-1"}},
	}

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			call := calls[idx%len(calls)]
			result, err := session.CallTool(ctx, &call)
			if err != nil {
				t.Errorf("goroutine %d: CallTool(%s) transport error: %v", idx, call.Name, err)
				return
			}
			_ = result
		}(i)
	}
	wg.Wait()
}

// TestConcurrentMixedToolCalls_NoRace mixes read-like and write-like tool
// calls across 50 goroutines with multiple workspace hashes, stress-testing
// concurrent handler execution under the race detector.
func TestConcurrentMixedToolCalls_NoRace(t *testing.T) {
	session, ctx := setupMockedTestClient(t)

	type toolCall struct {
		name string
		args map[string]any
	}

	palette := []toolCall{
		{"memory_status", map[string]any{}},
		{"memory_update", map[string]any{"workspace": "ws-a"}},
		{"memory_update", map[string]any{"workspace": "ws-b"}},
		{"memory_get", map[string]any{"workspace": "ws-a", "path": "id-1"}},
		{"memory_get", map[string]any{"workspace": "ws-b", "path": "id-2"}},
		{"memory_write", map[string]any{"workspace": "ws-a", "content": "doc alpha"}},
		{"memory_write", map[string]any{"workspace": "ws-b", "content": "doc beta"}},
		{"memory_query", map[string]any{"workspace": "ws-a", "query": "search alpha"}},
		{"memory_query", map[string]any{"workspace": "all", "query": "cross-workspace"}},
		{"memory_search", map[string]any{"workspace": "ws-a", "query": "bm25 alpha"}},
		{"memory_search", map[string]any{"workspace": "all", "query": "bm25 all"}},
		{"memory_vsearch", map[string]any{"workspace": "ws-b", "query": "vector beta"}},
		{"memory_tags", map[string]any{"workspace": "ws-a"}},
		{"memory_wake_up", map[string]any{"workspace": "ws-b", "limit": float64(5)}},
	}

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			tc := palette[idx%len(palette)]
			result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
				Name:      tc.name,
				Arguments: tc.args,
			})
			if err != nil {
				t.Errorf("goroutine %d: CallTool(%s) transport error: %v", idx, tc.name, err)
				return
			}
			_ = result
		}(i)
	}
	wg.Wait()
}
