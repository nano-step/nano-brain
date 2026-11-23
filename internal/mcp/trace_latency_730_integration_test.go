//go:build integration

package mcp_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nano-brain/nano-brain/internal/config"
	internalmcp "github.com/nano-brain/nano-brain/internal/mcp"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
	"github.com/rs/zerolog"
)

// TestTraceLatency730Evidence is an opt-in, verbose measurement, not a latency
// assertion. All fixture data and index operations are confined to this test's
// schema in nanobrain_test; no real indexed workspace is used.
func TestTraceLatency730Evidence(t *testing.T) {
	dsn, err := url.Parse(testutil.TestDSN())
	if err != nil || dsn.Path != "/nanobrain_test" {
		t.Fatal("evidence test requires a nanobrain_test DSN")
	}
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	var databaseName string
	if err := db.QueryRowContext(ctx, "SELECT current_database()").Scan(&databaseName); err != nil {
		t.Fatal(err)
	}
	if databaseName != "nanobrain_test" {
		t.Fatalf("evidence test requires nanobrain_test, got %q", databaseName)
	}
	q := sqlc.New(db)
	ws := fmt.Sprintf("%x", sha256.Sum256([]byte("synthetic-"+uuid.NewString())))
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: ws, Name: "synthetic", Path: "/tmp/synthetic-trace",
	}); err != nil {
		t.Fatal(err)
	}

	// Bulk-insert unrelated symbols to make a missing title scan observable.
	_, err = db.ExecContext(ctx, `
		INSERT INTO documents (workspace_hash, content_hash, blob_id, title, source_path, collection, metadata)
		SELECT $1, 'fixture', intern_content_blob('fixture'),
		       'unrelated_' || n, 'unrelated/file_' || n || '.go?symbol=unrelated_' || n,
		       'code', '{"source_type":"symbol","kind":"function"}'::jsonb
		FROM generate_series(1, 14000) AS n`, ws)
	if err != nil {
		t.Fatal(err)
	}
	upsertSymbolDoc(t, ctx, q, ws, "fixture/present.go", "present", "function", "func present() {}", "1", "1")
	for i := 0; i < 40; i++ {
		source := fmt.Sprintf("fixture/branch_%02d.go::Branch", i)
		for _, e := range []struct{ from, to, file string }{
			{"fixture/entry.go::Entry", source, "fixture/entry.go"},
			{source, "present", fmt.Sprintf("fixture/branch_%02d.go", i)},
			{source, "missing", fmt.Sprintf("fixture/branch_%02d.go", i)},
		} {
			if err := q.UpsertGraphEdge(ctx, sqlc.UpsertGraphEdgeParams{
				WorkspaceHash: ws, SourceNode: e.from, TargetNode: e.to,
				SourceFile: e.file, EdgeType: "calls", Metadata: []byte("{}"),
			}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := db.ExecContext(ctx, "ANALYZE documents"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "ANALYZE graph_edges"); err != nil {
		t.Fatal(err)
	}

	lookup := `SELECT source_path, metadata FROM documents
		WHERE workspace_hash = $1 AND metadata->>'source_type' = 'symbol' AND title = $2::text
		ORDER BY CASE metadata->>'kind'
			WHEN 'function' THEN 0
			WHEN 'struct' THEN 1 WHEN 'type' THEN 1 WHEN 'interface' THEN 1
			WHEN 'const' THEN 2 WHEN 'var' THEN 2 WHEN 'method' THEN 3
			ELSE 4 END, source_path`
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for _, title := range []string{"present", "missing"} {
		traceExplain730(t, ctx, conn, "indexed "+title, lookup, ws, title)
	}
	// DROP INDEX is transactional: the exact same data, connection, SQL and
	// planner settings are used; rollback restores the migrated index.
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DROP INDEX idx_documents_workspace_symbol_title"); err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"present", "missing"} {
		traceExplain730(t, ctx, tx, "unindexed "+title, lookup, ws, title)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}

	// Bare-node fallback still uses split_part(source_node), independently of
	// the documents title index. Include this residual in the evidence.
	traceExplain730(t, ctx, conn, "bare graph fallback", `
		SELECT * FROM graph_edge_views
		WHERE workspace_hash = $1 AND source_node <> '<unresolved>'
		  AND target_node <> '<unresolved>'
		  AND (source_node = $2 OR
		       (NOT ($2 ~ '^(?:(?:[^/]+/)*[^/]+\.(?:js|jsx|mjs|cjs|ts|tsx|mts|cts))::')
		        AND split_part(source_node, '::', 2) = $2))
		  AND ($3::text = '' OR edge_type = $3)
		ORDER BY edge_type, target_node`, ws, "Branch", "calls")

	server := internalmcp.NewMCPServer("trace-evidence")
	adapter := internalmcp.NewAdapter(q, db, nil, nil, nil, config.EmbeddingConfig{}, config.SearchConfig{}, config.FlowConfig{}, nil, zerolog.Nop())
	internalmcp.RegisterTools(server, adapter)
	ct, st := mcpsdk.NewInMemoryTransports()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "trace-evidence", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	args := map[string]any{
		"workspace": ws, "node": "fixture/entry.go::Entry",
		"max_depth": float64(2), "paths": "relative", "include_external": true,
	}
	durations := make([]time.Duration, 0, 20)
	for i := 0; i < 25; i++ {
		start := time.Now()
		result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "memory_trace", Arguments: args})
		elapsed := time.Since(start)
		if err != nil {
			t.Fatal(err)
		}
		response := unmarshalGraphResp(t, result)
		if response["count"] != float64(42) {
			t.Fatalf("trace count = %v, want 42 (40 branches, one present, one missing)", response["count"])
		}
		if i >= 5 {
			durations = append(durations, elapsed)
		}
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	t.Logf("MCP in-memory transport, 5 warmup + 20 measured: p50=%s p95=%s min=%s max=%s",
		durations[9], durations[18], durations[0], durations[len(durations)-1])

	httpServer := httptest.NewServer(internalmcp.NewStreamableHTTPHandler(server))
	defer httpServer.Close()
	requestJSON := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"memory_trace","arguments":{"workspace":%q,"node":"fixture/entry.go::Entry","max_depth":2,"paths":"relative","include_external":true}}}`, ws)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, httpServer.URL+"/mcp", strings.NewReader(requestJSON))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	reply, err := (&http.Client{Timeout: 30 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer reply.Body.Close()
	if reply.StatusCode != http.StatusOK {
		t.Fatalf("HTTP MCP trace: %s", reply.Status)
	}
	var envelope struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	raw, err := io.ReadAll(reply.Body)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(reply.Header.Get("Content-Type"), "text/event-stream") {
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(line, "data: ") {
				raw = []byte(strings.TrimPrefix(line, "data: "))
				break
			}
		}
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Result.Content) != 1 {
		t.Fatalf("HTTP MCP trace returned %d content blocks", len(envelope.Result.Content))
	}
	var traced struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal([]byte(envelope.Result.Content[0].Text), &traced); err != nil {
		t.Fatal(err)
	}
	if traced.Count != 42 {
		t.Fatalf("HTTP MCP trace count = %d, want 42", traced.Count)
	}
	t.Logf("%s %s; content-type=%s; memory_trace count=%d (synthetic workspace)",
		reply.Proto, reply.Status, reply.Header.Get("Content-Type"), traced.Count)
}

type traceExplainer730 interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func traceExplain730(t *testing.T, ctx context.Context, db traceExplainer730, label, query string, args ...any) {
	t.Helper()
	rows, err := db.QueryContext(ctx, "EXPLAIN (ANALYZE, BUFFERS) "+query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	t.Logf("%s:\n%s", label, strings.Join(lines, "\n"))
}
