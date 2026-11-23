package evalbench_test

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nano-brain/nano-brain/internal/evalbench"
)

func TestRunReplay_FiltersMutationsComputesOverlapAndRedactsPrivateData(t *testing.T) {
	var toolCalls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp" || r.Method != http.MethodPost {
			t.Errorf("unexpected replay request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		var env struct {
			ID     any    `json:"id"`
			Method string `json:"method"`
			Params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		_ = json.Unmarshal(raw, &env)
		if env.Method == "initialize" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18"}}`))
			return
		}
		if env.Method != "tools/call" {
			w.WriteHeader(http.StatusAccepted)
			return
		}

		toolCalls.Add(1)
		switch env.Params.Name {
		case "memory_query":
			inner, _ := json.Marshal(map[string]any{
				"results": []map[string]any{
					{"id": "1", "source_path": "src/pricing.ts", "title": "PriceModel", "snippet": "class PriceModel", "collection": "code"},
					{"id": "2", "source_path": "src/cart.ts", "title": "useCart", "snippet": "function useCart", "collection": "code"},
					{"id": "3", "source_path": "src/new_file.ts", "title": "NewHelper", "snippet": "helper", "collection": "code"},
				},
			})
			w.Header().Set("Content-Type", "text/event-stream")
			respJSON, _ := json.Marshal(map[string]any{
				"jsonrpc": "2.0",
				"id":      env.ID,
				"result": map[string]any{
					"content": []map[string]any{{"type": "text", "text": string(inner)}},
				},
			})
			_, _ = w.Write([]byte("event: message\ndata: " + string(respJSON) + "\n\n"))

		case "memory_get":
			// Simulate empty result on live replay to verify EmptyRate delta.
			w.Header().Set("Content-Type", "application/json")
			respJSON, _ := json.Marshal(map[string]any{
				"jsonrpc": "2.0",
				"id":      env.ID,
				"result": map[string]any{
					"content": []map[string]any{{"type": "text", "text": `{}`}},
				},
			})
			_, _ = w.Write(respJSON)

		default:
			t.Errorf("unexpected tool replayed: %s", env.Params.Name)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "calls-test.jsonl")

	recQueryInner, _ := json.Marshal(map[string]any{
		"results": []map[string]any{
			{"id": "1", "source_path": "src/pricing.ts", "title": "PriceModel"},
			{"id": "2", "source_path": "src/cart.ts", "title": "useCart"},
			{"id": "4", "source_path": "src/old_file.ts", "title": "OldHelper"},
		},
	})
	recQueryResp, _ := json.Marshal(map[string]any{
		"content": []map[string]any{{"type": "text", "text": string(recQueryInner)}},
	})

	lines := []map[string]any{
		// Handshake & REST OAuth noise -> SkippedNonTool
		{"kind": "mcp", "rpc_method": "initialize", "status": "ok", "latency_ms": 2},
		{"kind": "rest", "path": "/.well-known/oauth-authorization-server", "status": "ok", "latency_ms": 1},
		// Mutating MCP tools -> SkippedMutations (MUST NEVER be replayed!)
		{"kind": "mcp_tool", "tool": "memory_write", "args": map[string]any{"workspace": "typescript-app", "content": "SECRET_PRIVATE_WRITE"}, "status": "ok", "latency_ms": 15},
		{"kind": "mcp_tool", "tool": "memory_delete", "args": map[string]any{"workspace": "typescript-app", "path": "doc-1"}, "status": "ok", "latency_ms": 8},
		{"kind": "mcp_tool", "tool": "memory_update", "args": map[string]any{"workspace": "typescript-app", "path": "doc-2"}, "status": "ok", "latency_ms": 10},
		// Read-only memory_query (object args)
		{
			"kind":         "mcp_tool",
			"tool":         "memory_query",
			"args":         map[string]any{"workspace": "typescript-app", "query": "SENSITIVE_PRIVATE_QUERY_TOKEN"},
			"status":       "ok",
			"result_count": 3,
			"latency_ms":   42,
			"response":     string(recQueryResp),
		},
		// Read-only memory_get (stringified JSON args)
		{
			"kind":       "mcp_tool",
			"tool":       "memory_get",
			"args":       `{"workspace":"typescript-app","path":"SENSITIVE_PRIVATE_FILE_PATH.ts"}`,
			"status":     "ok",
			"latency_ms": 12,
			"response":   `{"content":[{"type":"text","text":"{\"id\":\"99\",\"source_path\":\"src/doc.ts\",\"content\":\"body\"}"}]}`,
		},
	}

	var rawLog strings.Builder
	for _, l := range lines {
		b, _ := json.Marshal(l)
		rawLog.Write(b)
		rawLog.WriteByte('\n')
	}
	if err := os.WriteFile(logFile, []byte(rawLog.String()), 0644); err != nil {
		t.Fatalf("write synthetic jsonl: %v", err)
	}

	snap := evalbench.WorkspaceSnapshot{
		CapturedAt: time.Now(),
		Workspaces: map[string]evalbench.WorkspaceInfo{
			"PLACEHOLDER_HASH_TS": {
				Hash:     "PLACEHOLDER_HASH_TS",
				Name:     "typescript-app",
				RootPath: "/data/workspaces/typescript-app",
			},
		},
	}

	evalClient := evalbench.NewEvalClient(srv.URL, evalbench.NewReadOnlyHTTPClient(5*time.Second, nil))
	summary, hashes, err := evalbench.RunReplay(context.Background(), evalClient, evalbench.ReplayOptions{
		LogPaths:       []string{filepath.Join(tmpDir, "calls-*.jsonl")},
		Limit:          200,
		VerbosePrivate: false,
	}, snap)
	if err != nil {
		t.Fatalf("RunReplay error: %v", err)
	}

	if got := toolCalls.Load(); got != 2 {
		t.Fatalf("expected exactly 2 read-only tool calls replayed, got %d", got)
	}
	if summary.TotalEntries != 7 || summary.TotalReplayed != 2 || summary.SkippedMutations != 3 || summary.SkippedNonTool != 2 {
		t.Fatalf("unexpected replay counts: %+v", summary)
	}
	if len(hashes) != 1 || hashes[0] != "PLACEHOLDER_HASH_TS" {
		t.Fatalf("expected resolved workspace hash PLACEHOLDER_HASH_TS, got %v", hashes)
	}
	if math.Abs(summary.RecordedEmptyRate-0.0) > 0.001 || math.Abs(summary.LiveEmptyRate-0.5) > 0.001 {
		t.Fatalf("expected RecordedEmptyRate=0.0 and LiveEmptyRate=0.5, got rec=%.3f live=%.3f", summary.RecordedEmptyRate, summary.LiveEmptyRate)
	}
	// Jaccard overlap on memory_query: recorded={src/pricing.ts, src/cart.ts, src/old_file.ts},
	// live={src/pricing.ts, src/cart.ts, src/new_file.ts} => |inter|=2 / |union|=4 = 0.500.
	if summary.Top5OverlapSamples != 1 || math.Abs(summary.Top5OverlapMean-0.5) > 0.001 {
		t.Fatalf("expected Top5OverlapMean=0.500 (1 sample), got %.3f (%d)", summary.Top5OverlapMean, summary.Top5OverlapSamples)
	}

	// Verify privacy-by-default: neither JSON nor Markdown contains private query/path/workspace strings!
	report := evalbench.BuildEvalReport(srv.URL, evalbench.TransportMCP, true, nil, &summary, evalbench.SnapshotDiff{})
	reportJSON, _ := json.Marshal(report)
	reportMD := evalbench.FormatMarkdown(report)
	for _, secret := range []string{
		"SENSITIVE_PRIVATE_QUERY_TOKEN",
		"SENSITIVE_PRIVATE_FILE_PATH",
		"SECRET_PRIVATE_WRITE",
		"typescript-app",
		"PLACEHOLDER_HASH_TS",
	} {
		if strings.Contains(string(reportJSON), secret) {
			t.Fatalf("default JSON report leaked %q:\n%s", secret, string(reportJSON))
		}
		if strings.Contains(reportMD, secret) {
			t.Fatalf("default Markdown report leaked %q:\n%s", secret, reportMD)
		}
	}
	if len(summary.ByWorkspace) != 1 || summary.ByWorkspace[0].Slot != "ws-1" {
		t.Fatalf("expected anonymized workspace slot ws-1, got %+v", summary.ByWorkspace)
	}

	// Verify Limit=1 and VerbosePrivate=true populate Details for the 1 replayed call.
	verboseSummary, _, err := evalbench.RunReplay(context.Background(), evalClient, evalbench.ReplayOptions{
		LogPaths:       []string{tmpDir},
		Limit:          1,
		VerbosePrivate: true,
	}, snap)
	if err != nil {
		t.Fatalf("RunReplay verbose: %v", err)
	}
	if verboseSummary.TotalReplayed != 1 || len(verboseSummary.Details) != 1 {
		t.Fatalf("expected 1 detail with Limit=1 & VerbosePrivate=true, got %+v", verboseSummary)
	}
}
