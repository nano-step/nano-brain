package evalbench_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nano-brain/nano-brain/internal/evalbench"
)

func TestReadOnlyTransport_BlocksAllMutatingOperations(t *testing.T) {
	var upstreamHits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := evalbench.NewReadOnlyHTTPClient(5*time.Second, nil)

	mcpBody := func(tool string) string {
		b, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "tools/call",
			"params": map[string]any{
				"name":      tool,
				"arguments": map[string]any{"workspace": "PLACEHOLDER_HASH"},
			},
		})
		return string(b)
	}

	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "mcp memory_write", method: http.MethodPost, path: "/mcp", body: mcpBody("memory_write")},
		{name: "mcp memory_delete", method: http.MethodPost, path: "/mcp", body: mcpBody("memory_delete")},
		{name: "mcp memory_update", method: http.MethodPost, path: "/mcp", body: mcpBody("memory_update")},
		{name: "mcp unknown tool", method: http.MethodPost, path: "/mcp", body: mcpBody("memory_wipe_all")},
		{name: "mcp batch containing memory_write", method: http.MethodPost, path: "/mcp", body: `[` + mcpBody("memory_query") + `,` + mcpBody("memory_write") + `]`},
		{name: "mcp unknown method", method: http.MethodPost, path: "/mcp", body: `{"jsonrpc":"2.0","id":1,"method":"resources/write"}`},
		{name: "rest write", method: http.MethodPost, path: "/api/v1/write", body: `{}`},
		{name: "rest reindex", method: http.MethodPost, path: "/api/v1/reindex", body: `{}`},
		{name: "rest update", method: http.MethodPost, path: "/api/v1/update", body: `{}`},
		{name: "rest init", method: http.MethodPost, path: "/api/v1/init", body: `{}`},
		{name: "rest reset-workspace", method: http.MethodPost, path: "/api/v1/reset-workspace", body: `{}`},
		{name: "rest config post", method: http.MethodPost, path: "/api/v1/config", body: `{}`},
		{name: "rest config patch", method: http.MethodPatch, path: "/api/v1/config", body: `{}`},
		{name: "rest harvest", method: http.MethodPost, path: "/api/harvest", body: ``},
		{name: "rest reload-config", method: http.MethodPost, path: "/api/reload-config", body: ``},
		{name: "http put collection", method: http.MethodPut, path: "/api/v1/collections/code", body: `{}`},
		{name: "http patch documents", method: http.MethodPatch, path: "/api/v1/documents/123", body: `{}`},
		{name: "http delete workspace", method: http.MethodDelete, path: "/api/v1/workspaces/PLACEHOLDER_HASH", body: ``},
		{name: "http delete document", method: http.MethodDelete, path: "/api/v1/documents/123", body: ``},
		{name: "http delete mcp", method: http.MethodDelete, path: "/mcp", body: ``},
		{name: "path traversal", method: http.MethodGet, path: "/api/v1/documents/../../config", body: ``},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var rdr io.Reader
			if tc.body != "" {
				rdr = strings.NewReader(tc.body)
			}
			req, err := http.NewRequestWithContext(context.Background(), tc.method, srv.URL+tc.path, rdr)
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := client.Do(req)
			if resp != nil {
				resp.Body.Close()
			}
			if !errors.Is(err, evalbench.ErrForbiddenMutation) {
				t.Fatalf("expected ErrForbiddenMutation for %s %s, got: %v", tc.method, tc.path, err)
			}
		})
	}

	// Also prove EvalClient.CallMCPTool blocks mutating tools with 0 upstream hits.
	evalClient := evalbench.NewEvalClient(srv.URL, client)
	for _, forbidden := range []string{"memory_write", "memory_delete", "memory_update"} {
		_, err := evalClient.CallMCPTool(context.Background(), forbidden, map[string]any{"workspace": "PLACEHOLDER_HASH"})
		if !errors.Is(err, evalbench.ErrForbiddenMutation) {
			t.Fatalf("CallMCPTool(%q) expected ErrForbiddenMutation, got %v", forbidden, err)
		}
	}

	if got := upstreamHits.Load(); got != 0 {
		t.Fatalf("expected 0 requests to reach upstream server, got %d", got)
	}
}

func TestReadOnlyTransport_AllowsAllReadOnlyOperations(t *testing.T) {
	var upstreamHits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits.Add(1)
		if r.Method == http.MethodPost && r.URL.Path == "/mcp" {
			body, _ := io.ReadAll(r.Body)
			if len(bytes.TrimSpace(body)) == 0 {
				t.Errorf("expected preserved body on POST /mcp")
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{}"}]}}`))
	}))
	defer srv.Close()

	client := evalbench.NewReadOnlyHTTPClient(5*time.Second, nil)

	getPaths := []string{
		"/health",
		"/api/status",
		"/api/v1/workspaces",
		"/api/v1/stats?workspace=PLACEHOLDER_HASH",
		"/api/v1/symbols?workspace=PLACEHOLDER_HASH&query=Foo",
		"/api/v1/tags?workspace=PLACEHOLDER_HASH",
		"/api/v1/wake-up?workspace=PLACEHOLDER_HASH",
		"/api/v1/documents?workspace=PLACEHOLDER_HASH",
		"/api/v1/documents/doc-123",
	}
	for _, p := range getPaths {
		resp, err := client.Get(srv.URL + p)
		if err != nil {
			t.Fatalf("GET %s failed unexpectedly: %v", p, err)
		}
		resp.Body.Close()
	}

	postPaths := []string{
		"/api/v1/query",
		"/api/v1/search",
		"/api/v1/vsearch",
		"/api/v1/get",
		"/api/v1/multi-get",
		"/api/v1/graph",
		"/api/v1/graph/query",
		"/api/v1/graph/impact",
		"/api/v1/graph/trace",
		"/api/v1/graph/flow",
		"/api/v1/graph/flowchart",
		"/api/v1/workspaces/resolve",
		"/api/v1/wake-up",
	}
	for _, p := range postPaths {
		resp, err := client.Post(srv.URL+p, "application/json", strings.NewReader(`{"workspace":"PLACEHOLDER_HASH"}`))
		if err != nil {
			t.Fatalf("POST %s failed unexpectedly: %v", p, err)
		}
		resp.Body.Close()
	}

	readOnlyTools := []string{
		"memory_query",
		"memory_search",
		"memory_vsearch",
		"memory_symbols",
		"memory_get",
		"memory_multi_get",
		"memory_graph",
		"memory_impact",
		"memory_trace",
		"memory_flow",
		"memory_flowchart",
		"memory_tags",
		"memory_status",
		"memory_wake_up",
		"memory_workspaces_list",
		"memory_workspaces_resolve",
		"memory_ticket",
	}
	for _, tool := range readOnlyTools {
		payload, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "tools/call",
			"params": map[string]any{
				"name":      tool,
				"arguments": map[string]any{"workspace": "PLACEHOLDER_HASH"},
			},
		})
		resp, err := client.Post(srv.URL+"/mcp", "application/json", bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("MCP tools/call %s failed unexpectedly: %v", tool, err)
		}
		resp.Body.Close()
	}

	expectedHits := int64(len(getPaths) + len(postPaths) + len(readOnlyTools))
	if got := upstreamHits.Load(); got != expectedHits {
		t.Fatalf("expected %d allowed hits, got %d", expectedHits, got)
	}
}

func TestSnapshotGuard_CaptureAndDiff(t *testing.T) {
	var callNum atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callNum.Add(1)
		docCount := int64(100)
		bgCount := int64(50)
		if n == 2 {
			bgCount = 51 // non-target workspace changed concurrently
		}
		if n == 3 {
			docCount = 101 // target workspace mutated
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"workspaces": []map[string]any{
				{
					"hash":        "PLACEHOLDER_HASH_TARGET",
					"name":        "vue-app",
					"root_path":   "/data/workspaces/vue-app",
					"doc_count":   docCount,
					"chunk_count": 400,
				},
				{
					"hash":        "PLACEHOLDER_HASH_OTHER",
					"name":        "rails-app",
					"root_path":   "/data/workspaces/rails-app",
					"doc_count":   bgCount,
					"chunk_count": 200,
				},
			},
		})
	}))
	defer srv.Close()

	client := evalbench.NewReadOnlyHTTPClient(5*time.Second, nil)
	ctx := context.Background()

	s1, err := evalbench.CaptureWorkspaceSnapshot(ctx, client, srv.URL)
	if err != nil {
		t.Fatalf("CaptureWorkspaceSnapshot 1: %v", err)
	}
	s2, err := evalbench.CaptureWorkspaceSnapshot(ctx, client, srv.URL)
	if err != nil {
		t.Fatalf("CaptureWorkspaceSnapshot 2: %v", err)
	}
	diffUnchanged := evalbench.DiffSnapshots(s1, s2, []string{"PLACEHOLDER_HASH_TARGET"})
	if diffUnchanged.Mutated {
		t.Fatalf("expected Mutated=false when only non-target workspace changed, got %+v", diffUnchanged)
	}
	if diffUnchanged.BackgroundChanges != 1 {
		t.Fatalf("expected BackgroundChanges=1, got %d", diffUnchanged.BackgroundChanges)
	}

	s3, err := evalbench.CaptureWorkspaceSnapshot(ctx, client, srv.URL)
	if err != nil {
		t.Fatalf("CaptureWorkspaceSnapshot 3: %v", err)
	}
	diffMutated := evalbench.DiffSnapshots(s2, s3, []string{"PLACEHOLDER_HASH_TARGET"})
	if !diffMutated.Mutated || len(diffMutated.Mutations) != 1 {
		t.Fatalf("expected Mutated=true with 1 mutation, got %+v", diffMutated)
	}
	if diffMutated.Mutations[0].Slot != "ws-1" {
		t.Fatalf("expected anonymized slot ws-1, got %q", diffMutated.Mutations[0].Slot)
	}
}
