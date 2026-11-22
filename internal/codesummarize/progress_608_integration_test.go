//go:build integration

package codesummarize

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
	"github.com/sqlc-dev/pqtype"
)

func TestSplitAndSend_PartialResponseCoolsMissingSymbols(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	q := sqlc.New(db)
	workspace := uuid.NewString()
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{Hash: workspace, Name: "fixture", Path: "/tmp/fixture"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"A", "B"} {
		seedPendingSymbol(t, ctx, q, workspace, name)
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{
				"content": `{"summaries":[{"name":"A","file":"src/A.go","summary":"Processes A"}]}`,
			}}},
		})
	}))
	defer provider.Close()
	cfg := config.CodeSummarizationConfig{ProviderURL: provider.URL, Model: "fixture"}
	s := NewService(cfg, NewLLMProvider(cfg, testutil.NopLogger()), NewBudgetTracker(q), q, nil, testutil.NopLogger())
	batch := []SymbolForSummary{
		{Name: "A", File: "src/A.go", Kind: "function", Code: "func A() {}", ContentHash: "symbol-A"},
		{Name: "B", File: "src/B.go", Kind: "function", Code: "func B() {}", ContentHash: "symbol-B"},
	}
	processed, failed := s.splitAndSend(ctx, workspace, batch, nil)
	if processed != 1 || failed != 1 {
		t.Fatalf("partial batch: processed=%d errors=%d, want 1/1", processed, failed)
	}
	failures, err := q.GetUnresolvedFailures(ctx, workspace)
	if err != nil {
		t.Fatal(err)
	}
	if len(failures) != 1 || failures[0].SymbolName != "B" || failures[0].ErrorType != "partial_omitted" {
		t.Fatalf("missing symbol not parked: %+v", failures)
	}
	rows, err := q.GetUnsummarizedSymbols(ctx, sqlc.GetUnsummarizedSymbolsParams{WorkspaceHash: workspace, Limit: 10})
	if err != nil || len(rows) != 0 {
		t.Fatalf("missing symbol reselected immediately: rows=%v err=%v", rows, err)
	}
}

func TestWorkerSweep_OneBatchPerWorkspaceAndRotates(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	q := sqlc.New(db)
	for _, prefix := range []string{"A", "B"} {
		workspace := "ws-" + prefix
		if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
			Hash: workspace, Name: prefix, Path: "/tmp/" + prefix,
		}); err != nil {
			t.Fatal(err)
		}
		for i := range 4 {
			seedPendingSymbol(t, ctx, q, workspace, prefix+string(rune('0'+i)))
		}
	}
	var mu sync.Mutex
	var calls []string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		prompt := req.Messages[1].Content
		workspace := "B"
		if strings.Contains(prompt, "src/A") {
			workspace = "A"
		}
		mu.Lock()
		calls = append(calls, workspace)
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"content": `{"summaries":[]}`}}},
		})
	}))
	defer provider.Close()
	cfg := config.CodeSummarizationConfig{
		ProviderURL: provider.URL, Model: "fixture", BatchSize: 2, MaxSummariesPerCycle: 4,
	}
	s := NewService(cfg, NewLLMProvider(cfg, testutil.NopLogger()), NewBudgetTracker(q), q, nil, testutil.NopLogger()).WithWorkspaceLister(q)
	for sweep := range 2 {
		if backoff := s.processAllWorkspaces(ctx, 0, 0); backoff != 0 {
			t.Fatalf("unexpected backoff: %v", backoff)
		}
		mu.Lock()
		if len(calls) != 2*(sweep+1) || calls[2*sweep] == calls[2*sweep+1] {
			t.Fatalf("sweep %d did not yield a batch to both workspaces: %v", sweep, calls)
		}
		if sweep == 1 && calls[0] == calls[2] {
			t.Fatalf("start workspace did not rotate: %v", calls)
		}
		mu.Unlock()
	}
}

func TestRetryContentHashes_BypassesCooldownWithoutSelectingOtherSymbols(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	q := sqlc.New(db)
	workspace := uuid.NewString()
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{Hash: workspace, Name: "fixture", Path: "/tmp/fixture"}); err != nil {
		t.Fatal(err)
	}
	seedPendingSymbol(t, ctx, q, workspace, "Failed")
	seedPendingSymbol(t, ctx, q, workspace, "New")
	if err := q.UpsertCodeSummarizationFailure(ctx, sqlc.UpsertCodeSummarizationFailureParams{
		WorkspaceHash: workspace, SymbolName: "Failed", SourceFile: "src/Failed.go",
		ContentHash: "symbol-Failed", ErrorReason: "HTTP 524", ErrorType: "transient_exhausted",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE code_summarization_failures SET attempts = 8 WHERE workspace_hash = $1`, workspace); err != nil {
		t.Fatal(err)
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if !strings.Contains(req.Messages[1].Content, "Failed") || strings.Contains(req.Messages[1].Content, "New") {
			t.Errorf("retry included an unselected symbol: %q", req.Messages[1].Content)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{
				"content": `{"summaries":[{"name":"Failed","file":"src/Failed.go","summary":"Processes failed input"}]}`,
			}}},
		})
	}))
	defer provider.Close()
	cfg := config.CodeSummarizationConfig{ProviderURL: provider.URL, Model: "fixture"}
	s := NewService(cfg, NewLLMProvider(cfg, testutil.NopLogger()), NewBudgetTracker(q), q, nil, testutil.NopLogger())
	processed, _, failed, err := s.RetryContentHashes(ctx, workspace, []string{"symbol-Failed"})
	if err != nil || processed != 1 || failed != 0 {
		t.Fatalf("manual retry: processed=%d failed=%d err=%v", processed, failed, err)
	}
	failures, err := q.GetUnresolvedFailures(ctx, workspace)
	if err != nil || len(failures) != 0 {
		t.Fatalf("successful retry should resolve failure: %v, err=%v", failures, err)
	}
}

func TestRetryContentHashes_ProcessesBeyondCycleLimit(t *testing.T) {
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
	var hashes []string
	for _, name := range []string{"A", "B", "C"} {
		seedPendingSymbol(t, ctx, q, workspace, name)
		hashes = append(hashes, "symbol-"+name)
		if err := q.UpsertCodeSummarizationFailure(ctx, sqlc.UpsertCodeSummarizationFailureParams{
			WorkspaceHash: workspace, SymbolName: name, SourceFile: "src/" + name + ".go",
			ContentHash: "symbol-" + name, ErrorReason: "HTTP 524", ErrorType: "transient_exhausted",
		}); err != nil {
			t.Fatal(err)
		}
	}
	calls := 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{
				"content": `{"summaries":[
					{"name":"A","file":"src/A.go","summary":"Summary A"},
					{"name":"B","file":"src/B.go","summary":"Summary B"},
					{"name":"C","file":"src/C.go","summary":"Summary C"}]}`,
			}}},
		})
	}))
	defer provider.Close()
	cfg := config.CodeSummarizationConfig{
		ProviderURL: provider.URL, Model: "fixture", MaxSummariesPerCycle: 2,
	}
	s := NewService(cfg, NewLLMProvider(cfg, testutil.NopLogger()), NewBudgetTracker(q), q, nil, testutil.NopLogger())
	processed, _, failed, err := s.RetryContentHashes(ctx, workspace, hashes)
	if err != nil || processed != 3 || failed != 0 || calls != 2 {
		t.Fatalf("retry beyond cycle limit: processed=%d failed=%d calls=%d err=%v", processed, failed, calls, err)
	}
	remaining, err := q.GetUnresolvedFailures(ctx, workspace)
	if err != nil || len(remaining) != 0 {
		t.Fatalf("retry left failures: %v, err=%v", remaining, err)
	}
}

func TestWorkerSweep_GraphStaleProgressesWithNewBacklog(t *testing.T) {
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
	for _, name := range []string{"NewA", "NewB", "Stale"} {
		seedPendingSymbol(t, ctx, q, workspace, name)
	}
	if _, err := q.UpsertDocument(ctx, sqlc.UpsertDocumentParams{
		WorkspaceHash: workspace, ContentHash: "summary-Stale", Content: "Old summary",
		SourcePath: "src/Stale.go?summary=true", Collection: "code",
		Tags: []string{"symbol-summary"},
		Metadata: pqtype.NullRawMessage{
			RawMessage: []byte(`{"source_content_hash":"symbol-Stale"}`), Valid: true,
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := q.UpsertGraphEdge(ctx, sqlc.UpsertGraphEdgeParams{
		WorkspaceHash: workspace, SourceNode: "src/Caller.go::Caller",
		TargetNode: "src/Stale.go::Stale", EdgeType: "calls",
		SourceFile: "src/Caller.go", Metadata: []byte("{}"),
	}); err != nil {
		t.Fatal(err)
	}
	staleRows, err := q.GetSymbolChunksByGraphContextStale(ctx, sqlc.GetSymbolChunksByGraphContextStaleParams{
		WorkspaceHash: workspace, Limit: 1,
	})
	if err != nil || len(staleRows) != 1 {
		t.Fatalf("stale summary fixture: rows=%v err=%v", staleRows, err)
	}
	contexts, err := FetchGraphContext(ctx, q, workspace, []string{"src/Stale.go::Stale"})
	if err != nil || ComputeGraphContextHash(contexts["src/Stale.go::Stale"]) == "" {
		t.Fatalf("stale summary fixture has no graph change: context=%v err=%v", contexts, err)
	}
	var mu sync.Mutex
	var prompts []string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode provider request: %v", err)
			return
		}
		mu.Lock()
		prompts = append(prompts, req.Messages[1].Content)
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{
				"content": `{"summaries":[{"name":"Stale","file":"src/Stale.go","summary":"Updated graph summary"}]}`,
			}}},
		})
	}))
	defer provider.Close()
	cfg := config.CodeSummarizationConfig{
		ProviderURL: provider.URL, Model: "fixture", BatchSize: 1, MaxSummariesPerCycle: 1,
	}
	s := NewService(cfg, NewLLMProvider(cfg, testutil.NopLogger()), NewBudgetTracker(q), q, nil, testutil.NopLogger()).WithWorkspaceLister(q)
	for range 2 {
		if backoff := s.processAllWorkspaces(ctx, 0, 0); backoff != 0 {
			t.Fatalf("unexpected backoff: %v", backoff)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(prompts) != 2 || !strings.Contains(prompts[1], "Stale") {
		t.Fatalf("graph-stale queue starved behind fresh symbols: %v", prompts)
	}
}
