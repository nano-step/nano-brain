//go:build integration

package mcp_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

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

func setupSearchMCP(t *testing.T) (context.Context, *sql.DB, *sqlc.Queries, string, func(string, map[string]any) *mcpsdk.CallToolResult) {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { db.Close() })

	queries := sqlc.New(db)
	wsHash := fmt.Sprintf("%x", sha256.Sum256([]byte("test_ws_"+uuid.New().String())))
	if _, err := queries.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: wsHash, Name: "test-ws", Path: "/tmp/" + wsHash,
	}); err != nil {
		t.Fatalf("upsert workspace: %v", err)
	}

	server := internalmcp.NewMCPServer("test")
	adapter := internalmcp.NewAdapter(queries, db, nil, nil, nil, config.EmbeddingConfig{}, config.SearchConfig{}, config.FlowConfig{}, nil, zerolog.Nop())
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
	return ctx, db, queries, wsHash, callTool
}

func contentHash(s string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(s)))
}

func insertDocWithChunk(t *testing.T, ctx context.Context, db *sql.DB, wsHash, title, content string) uuid.UUID {
	t.Helper()
	docID := uuid.New()
	chunkID := uuid.New()
	_, err := db.ExecContext(ctx,
		`INSERT INTO documents (id, workspace_hash, content_hash, title, source_path, collection)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		docID, wsHash, contentHash(title+content), title, "/tmp/"+title+".md", "memory")
	if err != nil {
		t.Fatalf("insert doc %q: %v", title, err)
	}
	_, err = db.ExecContext(ctx,
		`INSERT INTO chunks (id, document_id, workspace_hash, content_hash, content, chunk_index)
		 VALUES ($1, $2, $3, $4, $5, 0)`,
		chunkID, docID, wsHash, contentHash(content+chunkID.String()), content)
	if err != nil {
		t.Fatalf("insert chunk for %q: %v", title, err)
	}
	return chunkID
}

type searchResponse struct {
	Results    []json.RawMessage `json:"results"`
	Total      int               `json:"total"`
	QueryMs    int64             `json:"query_ms"`
	NextCursor string            `json:"next_cursor,omitempty"`
}

type searchResultItem struct {
	ID            string  `json:"id"`
	DocumentID    string  `json:"document_id"`
	WorkspaceHash string  `json:"workspace_hash"`
	Title         string  `json:"title"`
	Snippet       string  `json:"snippet"`
	Content       string  `json:"content,omitempty"`
	Score         float64 `json:"score"`
	Tags          []string `json:"tags"`
	Collection    string  `json:"collection"`
	SourcePath    string  `json:"source_path"`
}

func parseSearchResponse(t *testing.T, result *mcpsdk.CallToolResult) (searchResponse, string) {
	t.Helper()
	text := result.Content[0].(*mcpsdk.TextContent).Text
	var resp searchResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal response: %v\nraw: %s", err, text)
	}
	return resp, text
}

func parseResultItems(t *testing.T, resp searchResponse) []searchResultItem {
	t.Helper()
	items := make([]searchResultItem, len(resp.Results))
	for i, raw := range resp.Results {
		if err := json.Unmarshal(raw, &items[i]); err != nil {
			t.Fatalf("unmarshal result[%d]: %v", i, err)
		}
	}
	return items
}

func TestMemorySearch_DefaultExcludesContent(t *testing.T) {
	ctx, db, _, wsHash, callTool := setupSearchMCP(t)
	for i := 0; i < 3; i++ {
		insertDocWithChunk(t, ctx, db, wsHash,
			fmt.Sprintf("snippet-test-%d", i),
			fmt.Sprintf("searchable keyword bravo document number %d with some extra text", i))
	}

	result := callTool("memory_search", map[string]any{"workspace": wsHash, "query": "bravo"})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].(*mcpsdk.TextContent).Text)
	}

	resp, rawText := parseSearchResponse(t, result)
	if len(resp.Results) == 0 {
		t.Fatal("expected results, got 0")
	}

	items := parseResultItems(t, resp)
	for i, item := range items {
		if item.Snippet == "" {
			t.Errorf("result[%d]: snippet is empty", i)
		}
		if utf8.RuneCountInString(item.Snippet) > 500 {
			t.Errorf("result[%d]: snippet length %d > 500 runes", i, utf8.RuneCountInString(item.Snippet))
		}
	}

	for i, raw := range resp.Results {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("result[%d] unmarshal to map: %v", i, err)
		}
		if _, ok := m["content"]; ok {
			t.Errorf("result[%d]: content field should be absent in default response, raw: %s", i, rawText)
		}
	}
}

func TestMemorySearch_IncludeContentTrue(t *testing.T) {
	ctx, db, _, wsHash, callTool := setupSearchMCP(t)
	originalContent := "searchable keyword charlie full content to verify inclusion in response payload"
	insertDocWithChunk(t, ctx, db, wsHash, "include-content-doc", originalContent)

	result := callTool("memory_search", map[string]any{
		"workspace": wsHash, "query": "charlie", "include_content": true,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].(*mcpsdk.TextContent).Text)
	}

	resp, _ := parseSearchResponse(t, result)
	if len(resp.Results) == 0 {
		t.Fatal("expected results, got 0")
	}

	items := parseResultItems(t, resp)
	for i, item := range items {
		if item.Snippet == "" {
			t.Errorf("result[%d]: snippet is empty", i)
		}
		if item.Content == "" {
			t.Errorf("result[%d]: content should be present with include_content=true", i)
		}
		if item.Content != originalContent {
			t.Errorf("result[%d]: content mismatch\ngot:  %q\nwant: %q", i, item.Content, originalContent)
		}
	}
}

func TestMemorySearch_IncludeContentFalseMatchesDefault(t *testing.T) {
	ctx, db, _, wsHash, callTool := setupSearchMCP(t)
	insertDocWithChunk(t, ctx, db, wsHash, "false-default-doc",
		"searchable keyword delta comparing explicit false vs omitted parameter")

	resultDefault := callTool("memory_search", map[string]any{
		"workspace": wsHash, "query": "delta",
	})
	resultFalse := callTool("memory_search", map[string]any{
		"workspace": wsHash, "query": "delta", "include_content": false,
	})

	respDefault, _ := parseSearchResponse(t, resultDefault)
	respFalse, _ := parseSearchResponse(t, resultFalse)

	if len(respDefault.Results) != len(respFalse.Results) {
		t.Fatalf("result count mismatch: default=%d, false=%d", len(respDefault.Results), len(respFalse.Results))
	}

	itemsDefault := parseResultItems(t, respDefault)
	itemsFalse := parseResultItems(t, respFalse)
	for i := range itemsDefault {
		if itemsDefault[i].Snippet != itemsFalse[i].Snippet {
			t.Errorf("result[%d] snippet mismatch: default=%q false=%q", i, itemsDefault[i].Snippet, itemsFalse[i].Snippet)
		}
		if itemsDefault[i].Content != "" || itemsFalse[i].Content != "" {
			t.Errorf("result[%d]: content should be absent in both; default=%q false=%q", i, itemsDefault[i].Content, itemsFalse[i].Content)
		}
	}
}

func TestMemorySearch_SnippetUTF8Boundary(t *testing.T) {
	ctx, db, _, wsHash, callTool := setupSearchMCP(t)

	prefix := "echo " + strings.Repeat("x", 485) // 490 ASCII runes before multi-byte zone
	suffix := strings.Repeat("世", 20)            // 3-byte runes spanning positions 490..509
	content := prefix + suffix
	insertDocWithChunk(t, ctx, db, wsHash, "utf8-boundary-doc", content)

	result := callTool("memory_search", map[string]any{"workspace": wsHash, "query": "echo"})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].(*mcpsdk.TextContent).Text)
	}

	resp, _ := parseSearchResponse(t, result)
	if len(resp.Results) == 0 {
		t.Fatal("expected results, got 0")
	}

	items := parseResultItems(t, resp)
	snippet := items[0].Snippet

	if !utf8.ValidString(snippet) {
		t.Error("snippet contains invalid UTF-8")
	}
	runeCount := utf8.RuneCountInString(snippet)
	if runeCount > 500 {
		t.Errorf("snippet rune count %d > 500", runeCount)
	}
}

func TestMemorySearch_PayloadSizeBudget(t *testing.T) {
	ctx, db, _, wsHash, callTool := setupSearchMCP(t)
	for i := 0; i < 10; i++ {
		insertDocWithChunk(t, ctx, db, wsHash,
			fmt.Sprintf("budget-doc-%d", i),
			fmt.Sprintf("foxtrot keyword %s", strings.Repeat("a", 1500+i)))
	}

	result := callTool("memory_search", map[string]any{
		"workspace": wsHash, "query": "foxtrot", "max_results": 10,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].(*mcpsdk.TextContent).Text)
	}

	responseText := result.Content[0].(*mcpsdk.TextContent).Text
	if len(responseText) > 20000 {
		t.Errorf("response payload %d bytes > 20000 byte budget", len(responseText))
	}
}

func TestMemorySearch_PaginationRoundTrip(t *testing.T) {
	ctx, db, _, wsHash, callTool := setupSearchMCP(t)
	for i := 0; i < 12; i++ {
		insertDocWithChunk(t, ctx, db, wsHash,
			fmt.Sprintf("page-doc-%02d", i),
			fmt.Sprintf("golf keyword item number %d searchable", i))
	}

	page1 := callTool("memory_search", map[string]any{
		"workspace": wsHash, "query": "golf", "max_results": 5,
	})
	if page1.IsError {
		t.Fatalf("page1 error: %s", page1.Content[0].(*mcpsdk.TextContent).Text)
	}
	resp1, _ := parseSearchResponse(t, page1)
	if len(resp1.Results) != 5 {
		t.Fatalf("page1: got %d results, want 5", len(resp1.Results))
	}
	if resp1.NextCursor == "" {
		t.Fatal("page1: expected next_cursor, got empty")
	}

	page2 := callTool("memory_search", map[string]any{
		"workspace": wsHash, "query": "golf", "max_results": 5, "cursor": resp1.NextCursor,
	})
	if page2.IsError {
		t.Fatalf("page2 error: %s", page2.Content[0].(*mcpsdk.TextContent).Text)
	}
	resp2, _ := parseSearchResponse(t, page2)
	if len(resp2.Results) != 5 {
		t.Fatalf("page2: got %d results, want 5", len(resp2.Results))
	}
	if resp2.NextCursor == "" {
		t.Fatal("page2: expected next_cursor, got empty")
	}

	page3 := callTool("memory_search", map[string]any{
		"workspace": wsHash, "query": "golf", "max_results": 5, "cursor": resp2.NextCursor,
	})
	if page3.IsError {
		t.Fatalf("page3 error: %s", page3.Content[0].(*mcpsdk.TextContent).Text)
	}
	resp3, _ := parseSearchResponse(t, page3)
	if len(resp3.Results) != 2 {
		t.Fatalf("page3: got %d results, want 2", len(resp3.Results))
	}
	if resp3.NextCursor != "" {
		t.Errorf("page3: next_cursor should be absent, got %q", resp3.NextCursor)
	}

	seen := make(map[string]bool)
	allItems := append(parseResultItems(t, resp1), parseResultItems(t, resp2)...)
	allItems = append(allItems, parseResultItems(t, resp3)...)
	for _, item := range allItems {
		if seen[item.ID] {
			t.Errorf("duplicate result ID across pages: %s", item.ID)
		}
		seen[item.ID] = true
	}
	if len(seen) != 12 {
		t.Errorf("total unique results = %d, want 12", len(seen))
	}
}

func TestMemorySearch_FirstPageWithoutCursorReturnsResults(t *testing.T) {
	ctx, db, _, wsHash, callTool := setupSearchMCP(t)
	for i := 0; i < 5; i++ {
		insertDocWithChunk(t, ctx, db, wsHash,
			fmt.Sprintf("exact-doc-%d", i),
			fmt.Sprintf("hotel keyword result %d", i))
	}

	result := callTool("memory_search", map[string]any{
		"workspace": wsHash, "query": "hotel", "max_results": 5,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].(*mcpsdk.TextContent).Text)
	}

	resp, _ := parseSearchResponse(t, result)
	if len(resp.Results) != 5 {
		t.Fatalf("got %d results, want 5", len(resp.Results))
	}
	if resp.NextCursor != "" {
		t.Errorf("next_cursor should be absent when exact match count, got %q", resp.NextCursor)
	}
}

func TestMemorySearch_CursorQueryMismatch(t *testing.T) {
	_, _, _, wsHash, callTool := setupSearchMCP(t)

	cursor := search.EncodeCursor(5, search.QueryHash(search.QueryHashInput{Query: "alpha"}))
	result := callTool("memory_search", map[string]any{
		"workspace": wsHash, "query": "beta", "cursor": cursor,
	})

	if !result.IsError {
		t.Fatal("expected error for cursor query mismatch, got success")
	}
	errText := result.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(errText, "cursor query mismatch") {
		t.Errorf("error should contain 'cursor query mismatch', got: %s", errText)
	}
}

func TestMemorySearch_InvalidCursor(t *testing.T) {
	_, _, _, wsHash, callTool := setupSearchMCP(t)

	result := callTool("memory_search", map[string]any{
		"workspace": wsHash, "query": "anything", "cursor": "not-valid-base64!@#",
	})

	if !result.IsError {
		t.Fatal("expected error for invalid cursor, got success")
	}
	errText := result.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(errText, "invalid cursor") {
		t.Errorf("error should contain 'invalid cursor', got: %s", errText)
	}
}

func TestMemorySearch_ResponseIncludesTotalAndQueryMs(t *testing.T) {
	ctx, db, _, wsHash, callTool := setupSearchMCP(t)
	insertDocWithChunk(t, ctx, db, wsHash, "meta-doc", "india keyword metadata verification test")

	result := callTool("memory_search", map[string]any{
		"workspace": wsHash, "query": "india",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].(*mcpsdk.TextContent).Text)
	}

	resp, rawText := parseSearchResponse(t, result)
	if resp.Total < len(resp.Results) {
		t.Errorf("total (%d) < len(results) (%d)", resp.Total, len(resp.Results))
	}
	if resp.QueryMs < 0 {
		t.Errorf("query_ms should be >= 0, got %d", resp.QueryMs)
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(rawText), &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	if _, ok := raw["total"]; !ok {
		t.Error("response missing 'total' field at top level")
	}
	if _, ok := raw["query_ms"]; !ok {
		t.Error("response missing 'query_ms' field at top level")
	}
}

func TestMemorySearch_EmptyResultSet(t *testing.T) {
	_, _, _, wsHash, callTool := setupSearchMCP(t)

	result := callTool("memory_search", map[string]any{
		"workspace": wsHash, "query": "nonexistentterm_xyz_qpr",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].(*mcpsdk.TextContent).Text)
	}

	resp, rawText := parseSearchResponse(t, result)
	if len(resp.Results) != 0 {
		t.Errorf("expected empty results, got %d", len(resp.Results))
	}
	if resp.Total != 0 {
		t.Errorf("total should be 0, got %d", resp.Total)
	}
	if resp.QueryMs < 0 {
		t.Errorf("query_ms should be >= 0, got %d", resp.QueryMs)
	}
	if resp.NextCursor != "" {
		t.Errorf("next_cursor should be absent for empty results, got %q", resp.NextCursor)
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(rawText), &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	results, ok := raw["results"]
	if !ok {
		t.Fatal("response missing 'results' field")
	}
	arr, ok := results.([]any)
	if !ok || arr == nil {
		t.Errorf("results should be empty array (not null); got %T: %v", results, results)
	}
}
