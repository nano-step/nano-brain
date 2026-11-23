package evalbench_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nano-brain/nano-brain/internal/evalbench"
)

func TestRunSuite_MCPAndREST_MetricsRankingAndTokenSavings(t *testing.T) {
	mockRepo := t.TempDir()
	cartDir := filepath.Join(mockRepo, "app", "composables", "cart")
	if err := os.MkdirAll(cartDir, 0755); err != nil {
		t.Fatalf("mkdir cartDir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(mockRepo, ".git"), 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	// Ignored huge file inside .git must never be counted in naive baseline.
	_ = os.WriteFile(filepath.Join(mockRepo, ".git", "pack.js"), []byte(strings.Repeat("addTradeLog\n", 5000)), 0644)

	var cartFileBody strings.Builder
	cartFileBody.WriteString("export function useCart() {\n  return { addTradeLog, createCartLog };\n}\n")
	cartFileBody.WriteString("export function addTradeLog(payload) {\n  return createCartLog(payload);\n}\n")
	for i := 0; i < 60; i++ {
		cartFileBody.WriteString(fmt.Sprintf("// padding context line %d for realistic file size measurement\n", i))
	}
	if err := os.WriteFile(filepath.Join(cartDir, "useCart.js"), []byte(cartFileBody.String()), 0644); err != nil {
		t.Fatalf("write useCart.js: %v", err)
	}

	dsDir := t.TempDir()
	templatePath := filepath.Join(dsDir, "dataset.template.json")
	templateJSON := `{
  "version": 1,
  "workspace": "vue-app",
  "agent": {
    "enabled": true,
    "tools": ["symbols_identifiers"],
    "max_symbol_queries": 4
  },
  "tasks": [
    {
      "id": "vue-cart-01",
      "category": "search-qa",
      "question": "Where is createCartLog used in cart composable?",
      "tools": ["query"],
      "input": { "query": "cart composable" },
      "expect_symbols": ["addTradeLog", "createCartLog"],
      "expect_files": ["__PROJECT__/composables/cart/useCart.js"]
    },
    {
      "id": "vue-trace-01",
      "category": "trace",
      "question": "What does addTradeLog call?",
      "tools": ["trace"],
      "input": { "node": "__PROJECT__/composables/cart/useCart.js::addTradeLog", "max_depth": 3 },
      "expect_symbols": ["createCartLog"]
    }
  ]
}`
	if err := os.WriteFile(templatePath, []byte(templateJSON), 0644); err != nil {
		t.Fatalf("write dataset template: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.Header.Get("X-Requested-With") != "nano-brain-ui" {
			t.Errorf("missing X-Requested-With header on POST %s", r.URL.Path)
		}

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/workspaces":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"workspaces": []map[string]any{
					{
						"hash":        "PLACEHOLDER_HASH_VUE",
						"name":        "vue-app",
						"root_path":   mockRepo,
						"doc_count":   12,
						"chunk_count": 34,
					},
				},
			})

		case r.Method == http.MethodPost && r.URL.Path == "/mcp":
			if !strings.Contains(r.Header.Get("Accept"), "text/event-stream") || !strings.Contains(r.Header.Get("Accept"), "application/json") {
				t.Errorf("MCP POST missing dual Accept header: %q", r.Header.Get("Accept"))
			}
			raw, _ := io.ReadAll(r.Body)
			var rpc struct {
				ID     any    `json:"id"`
				Method string `json:"method"`
				Params struct {
					Name      string         `json:"name"`
					Arguments map[string]any `json:"arguments"`
				} `json:"params"`
			}
			_ = json.Unmarshal(raw, &rpc)

			if rpc.Method == "initialize" {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Mcp-Session-Id", "sess-test-1")
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18"}}`))
				return
			}
			if rpc.Method == "notifications/initialized" {
				w.WriteHeader(http.StatusAccepted)
				return
			}

			switch rpc.Params.Name {
			case "memory_query":
				// Return SSE framing! Surfaces addTradeLog + file, misses createCartLog so FixedRecall = 2/3,
				// then agent symbols_identifiers surfaces createCartLog -> AgentRecall = 3/3!
				inner, _ := json.Marshal(map[string]any{
					"results": []map[string]any{
						{"id": "1", "title": "useCart", "source_path": "app/composables/cart/useCart.js", "snippet": "function addTradeLog()", "collection": "code"},
						{"id": "2", "title": "Decision", "source_path": "memory://cart", "snippet": "note", "collection": "memory"},
						{"id": "3", "title": "Session", "source_path": "summary://ses_1", "snippet": "hist", "collection": "sessions"},
					},
				})
				env, _ := json.Marshal(map[string]any{
					"jsonrpc": "2.0",
					"id":      rpc.ID,
					"result": map[string]any{
						"content": []map[string]any{{"type": "text", "text": string(inner)}},
					},
				})
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = w.Write([]byte("event: message\ndata: " + string(env) + "\n\n"))

			case "memory_symbols":
				// Standard JSON framing!
				inner, _ := json.Marshal(map[string]any{
					"count": 1,
					"symbols": []map[string]any{
						{"name": "createCartLog", "kind": "function", "source_path": "app/composables/cart/useCart.js"},
					},
				})
				env, _ := json.Marshal(map[string]any{
					"jsonrpc": "2.0",
					"id":      rpc.ID,
					"result": map[string]any{
						"content": []map[string]any{{"type": "text", "text": string(inner)}},
					},
				})
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(env)

			case "memory_trace":
				// First item unrelated (rank 1), second item matches createCartLog (rank 2 -> MRR = 0.500, Recall@5 = 1.000).
				node, _ := rpc.Params.Arguments["node"].(string)
				if strings.Contains(node, "__PROJECT__") {
					t.Errorf("unsubstituted __PROJECT__ reached server: %s", node)
				}
				inner, _ := json.Marshal(map[string]any{
					"entry": node,
					"chain": []map[string]any{
						{"node": "app/composables/cart/useCart.js::validatePayload", "name": "validatePayload"},
						{"node": "app/composables/cart/useCart.js::createCartLog", "name": "createCartLog"},
					},
				})
				env, _ := json.Marshal(map[string]any{
					"jsonrpc": "2.0",
					"id":      rpc.ID,
					"result": map[string]any{
						"content": []map[string]any{{"type": "text", "text": string(inner)}},
					},
				})
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(env)
			}

		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/query":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{
					{"id": "1", "title": "useCart", "source_path": "app/composables/cart/useCart.js", "snippet": "function addTradeLog()", "collection": "code"},
					{"id": "2", "title": "Decision", "source_path": "memory://cart", "snippet": "note", "collection": "memory"},
					{"id": "3", "title": "Session", "source_path": "summary://ses_1", "snippet": "hist", "collection": "sessions"},
				},
			})

		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/symbols":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"count": 1,
				"symbols": []map[string]any{
					{"name": "createCartLog", "kind": "function", "source_path": "app/composables/cart/useCart.js"},
				},
			})

		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/graph/trace":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"entry": "app/composables/cart/useCart.js::addTradeLog",
				"chain": []map[string]any{
					{"node": "app/composables/cart/useCart.js::validatePayload"},
					{"node": "app/composables/cart/useCart.js::createCartLog"},
				},
			})

		default:
			t.Errorf("unexpected HTTP request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	roHTTP := evalbench.NewReadOnlyHTTPClient(5*time.Second, nil)
	ctx := context.Background()
	snap, err := evalbench.CaptureWorkspaceSnapshot(ctx, roHTTP, srv.URL)
	if err != nil {
		t.Fatalf("CaptureWorkspaceSnapshot: %v", err)
	}

	for _, transport := range []string{evalbench.TransportMCP, evalbench.TransportREST} {
		t.Run("transport_"+transport, func(t *testing.T) {
			client := evalbench.NewEvalClient(srv.URL, roHTTP)
			sr, resolvedHash, runErr := evalbench.RunSuite(ctx, client, evalbench.SuiteRunConfig{
				SuiteName:     "vue",
				DatasetPath:   templatePath,
				Transport:     transport,
				WorkspaceSlot: "suite-ws-1",
				Vars:          map[string]string{"PROJECT": "app"},
			}, snap)
			if runErr != nil {
				t.Fatalf("RunSuite(%s): %v", transport, runErr)
			}
			if sr.Status != "ok" || resolvedHash != "PLACEHOLDER_HASH_VUE" {
				t.Fatalf("expected status=ok and hash=PLACEHOLDER_HASH_VUE, got status=%s (%s) hash=%s", sr.Status, sr.SkipReason, resolvedHash)
			}

			// Task 1: FixedRecall = 2/3 = 0.667, AgentRecall = 1.000, MRR = 1.000 (rank 1 hit), RecallAt5 = 1.000
			// Task 2: FixedRecall = 1.000, AgentRecall = 1.000, MRR = 0.500 (rank 2 hit), RecallAt5 = 1.000
			if math.Abs(sr.Tasks[0].FixedRecall-0.667) > 0.002 || math.Abs(sr.Tasks[0].AgentRecall-1.0) > 0.001 {
				t.Fatalf("task 0 unexpected recall: fixed=%.3f agent=%.3f", sr.Tasks[0].FixedRecall, sr.Tasks[0].AgentRecall)
			}
			if math.Abs(sr.Tasks[1].MRR-0.5) > 0.001 || math.Abs(sr.Tasks[1].RecallAt5-1.0) > 0.001 {
				t.Fatalf("task 1 unexpected ranking: mrr=%.3f r@5=%.3f", sr.Tasks[1].MRR, sr.Tasks[1].RecallAt5)
			}
			if math.Abs(sr.OverallRecall-1.0) > 0.001 || math.Abs(sr.OverallMRR-0.75) > 0.001 {
				t.Fatalf("overall unexpected: recall=%.3f mrr=%.3f", sr.OverallRecall, sr.OverallMRR)
			}

			// Verify collection mix (code: 1, memory: 1, sessions: 1).
			if sr.CollectionDistribution["code"] != 1 || sr.CollectionDistribution["memory"] != 1 || sr.CollectionDistribution["sessions"] != 1 {
				t.Fatalf("unexpected collection mix: %+v", sr.CollectionDistribution)
			}

			// Verify per-tool latency summary.
			if len(sr.ToolLatencies) != 3 {
				t.Fatalf("expected 3 tool latency entries, got %+v", sr.ToolLatencies)
			}

			// Verify #529 Token-Cost Proxy: ground-truth useCart.js resolved, .git ignored, positive token savings!
			if sr.TokenEfficiency.TasksWithBaseline != 2 || sr.TokenEfficiency.NaiveTokens <= sr.TokenEfficiency.AgentTokens {
				t.Fatalf("expected naive tokens > agent tokens across both tasks, got %+v", sr.TokenEfficiency)
			}
			if sr.TokenEfficiency.TokenSavingsFixedPct <= 50.0 || sr.TokenEfficiency.TokenSavingsAgentPct <= 50.0 {
				t.Fatalf("expected >50%% token savings vs full file read + grep, got %+v", sr.TokenEfficiency)
			}

			// Verify no temporary dataset.json was written next to dataset.template.json.
			if _, statErr := os.Stat(filepath.Join(dsDir, "dataset.json")); !os.IsNotExist(statErr) {
				t.Fatal("RunSuite must not write dataset.json to disk")
			}
		})
	}

	// Verify unregistered workspace skips cleanly without calling init.
	client := evalbench.NewEvalClient(srv.URL, roHTTP)
	skippedRes, skippedHash, err := evalbench.RunSuite(ctx, client, evalbench.SuiteRunConfig{
		SuiteName:     "rails",
		DatasetPath:   templatePath,
		Transport:     evalbench.TransportMCP,
		WorkspaceSpec: "rails-app-unregistered",
		Vars:          map[string]string{"PROJECT": "app"},
	}, snap)
	if err != nil {
		t.Fatalf("unregistered suite returned error: %v", err)
	}
	if skippedRes.Status != "skipped" || skippedHash != "" {
		t.Fatalf("expected skipped suite, got %+v (hash=%q)", skippedRes, skippedHash)
	}
}

func TestEvalClient_ResolveWorkspaceByPathAndAdditionalTools(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/mcp":
			raw, _ := io.ReadAll(r.Body)
			var rpc struct {
				ID     any    `json:"id"`
				Method string `json:"method"`
				Params struct {
					Name      string         `json:"name"`
					Arguments map[string]any `json:"arguments"`
				} `json:"params"`
			}
			_ = json.Unmarshal(raw, &rpc)
			if rpc.Method == "initialize" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18"}}`))
				return
			}
			var inner string
			switch rpc.Params.Name {
			case "memory_workspaces_resolve":
				b, _ := json.Marshal(map[string]any{
					"workspace_hash": "UNREG_CHILD",
					"registered":     false,
					"covered_by": map[string]any{
						"workspace_hash": "PLACEHOLDER_HASH_PARENT",
						"workspace_name": "typescript-app",
						"root_path":      "/data/workspaces/typescript-app",
					},
				})
				inner = string(b)
			case "memory_flow":
				inner = `{"found":true,"chain":[{"id":"src/api.ts::handlePost","name":"handlePost"}],"externals":[{"id":"validate","name":"validate"}]}`
			case "memory_impact":
				inner = `{"node":"src/api.ts::handlePost","impacted":[{"node":"src/router.ts::mountRoutes"}]}`
			case "memory_graph":
				inner = `{"direction":"out","edges":[{"source":"src/api.ts::handlePost","target":"src/db.ts::saveOrder"}]}`
			case "memory_get":
				inner = `{"id":"doc-1","title":"api.ts","source_path":"src/api.ts","content":"export function handlePost() {}","collection":"code"}`
			case "memory_flowchart":
				inner = `{"found":true,"entry":"src/api.ts::handlePost"}`
			case "memory_wake_up":
				inner = `{"summary":"Active","recent_memories":[{"id":"m1","title":"Note","snippet":"recent note"}]}`
			}
			env, _ := json.Marshal(map[string]any{
				"jsonrpc": "2.0",
				"id":      rpc.ID,
				"result": map[string]any{
					"content": []map[string]any{{"type": "text", "text": inner}},
				},
			})
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(env)

		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/workspaces/resolve":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"workspace_hash":"PLACEHOLDER_HASH_PARENT","name":"typescript-app","root_path":"/data/workspaces/typescript-app","registered":true}`))
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	snap := evalbench.WorkspaceSnapshot{
		Workspaces: map[string]evalbench.WorkspaceInfo{
			"PLACEHOLDER_HASH_PARENT": {
				Hash:     "PLACEHOLDER_HASH_PARENT",
				Name:     "typescript-app",
				RootPath: "/data/workspaces/typescript-app",
			},
		},
	}

	client := evalbench.NewEvalClient(srv.URL, evalbench.NewReadOnlyHTTPClient(5*time.Second, nil))

	wsMCP, ok, err := client.ResolveWorkspaceReadOnly(ctx, evalbench.TransportMCP, "/data/workspaces/typescript-app/packages/api", snap)
	if err != nil || !ok || wsMCP.Hash != "PLACEHOLDER_HASH_PARENT" {
		t.Fatalf("expected MCP covered_by resolution to PLACEHOLDER_HASH_PARENT, got ok=%v ws=%+v err=%v", ok, wsMCP, err)
	}

	wsREST, ok, err := client.ResolveWorkspaceReadOnly(ctx, evalbench.TransportREST, "/data/workspaces/typescript-app", snap)
	if err != nil || !ok || wsREST.Hash != "PLACEHOLDER_HASH_PARENT" {
		t.Fatalf("expected REST path resolution to PLACEHOLDER_HASH_PARENT, got ok=%v ws=%+v err=%v", ok, wsREST, err)
	}

	for _, tool := range []string{"flow", "impact", "graph", "get", "flowchart", "wake_up"} {
		cr, callErr := client.CallTaskTool(ctx, evalbench.TransportMCP, "PLACEHOLDER_HASH_PARENT", tool, map[string]any{
			"entry": "POST /orders",
			"node":  "src/api.ts::handlePost",
			"path":  "src/api.ts",
		})
		if callErr != nil || cr.IsError || cr.IsEmpty || len(cr.OrderedItems) == 0 {
			t.Fatalf("tool %s failed or empty: err=%v cr=%+v", tool, callErr, cr)
		}
	}
}
