package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nano-brain/nano-brain/internal/evalbench"
)

func TestBenchEval_HelpAndConfigWarning(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := executeBenchEval(context.Background(), []string{"--help"}, "", &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0 for --help, got %d (stderr=%s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Usage: nano-brain bench eval") || !strings.Contains(stdout.String(), "--transport") {
		t.Fatalf("unexpected --help output:\n%s", stdout.String())
	}

	// Verify missing explicit --config path resolves via resolveCLIConfigPath (#233).
	resetClientConfigWarnState()
	missingCfg := filepath.Join(t.TempDir(), "missing-config.yml")
	_, warn := resolveCLIConfigPath(missingCfg)
	if !strings.Contains(warn, "WARNING:") {
		t.Fatalf("expected WARNING for missing explicit config path, got %q", warn)
	}
}

func TestBenchEval_EndToEndMockServerCompareAndImmutability(t *testing.T) {
	var mutateAfterFirstSnapshot atomic.Bool
	var wsCalls atomic.Int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/workspaces":
			n := wsCalls.Add(1)
			docs := int64(25)
			if mutateAfterFirstSnapshot.Load() && n >= 2 {
				docs = 26
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"workspaces": []map[string]any{
					{
						"hash":        "PLACEHOLDER_HASH_GO",
						"name":        "go-app",
						"root_path":   "/nonexistent/mock/go-app",
						"doc_count":   docs,
						"chunk_count": 80,
					},
				},
			})

		case r.Method == http.MethodPost && r.URL.Path == "/mcp":
			raw, _ := io.ReadAll(r.Body)
			var rpc struct {
				ID     any    `json:"id"`
				Method string `json:"method"`
				Params struct {
					Name string `json:"name"`
				} `json:"params"`
			}
			_ = json.Unmarshal(raw, &rpc)
			if rpc.Method == "initialize" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18"}}`))
				return
			}
			inner, _ := json.Marshal(map[string]any{
				"results": []map[string]any{
					{"id": "1", "title": "HybridSearch", "source_path": "internal/search/service.go", "snippet": "func HybridSearch()", "collection": "code"},
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

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	dsPath := filepath.Join(tmpDir, "dataset.json")
	dsContent := `{
  "version": 1,
  "workspace": "go-app",
  "agent": {"enabled": false},
  "tasks": [
    {
      "id": "go-q1",
      "category": "search-qa",
      "question": "Find HybridSearch",
      "tools": ["query"],
      "input": {"query": "HybridSearch"},
      "expect_symbols": ["HybridSearch"],
      "expect_files": ["internal/search/service.go"]
    }
  ]
}`
	if err := os.WriteFile(dsPath, []byte(dsContent), 0644); err != nil {
		t.Fatalf("write dataset: %v", err)
	}

	profilesPath := filepath.Join(tmpDir, "profiles.json")
	profilesContent, _ := json.Marshal(map[string]any{
		"go": map[string]any{
			"workspace": "go-app",
			"dataset":   dsPath,
		},
	})
	_ = os.WriteFile(profilesPath, profilesContent, 0644)

	replayLog := filepath.Join(tmpDir, "calls-1.jsonl")
	_ = os.WriteFile(replayLog, []byte(
		`{"kind":"mcp_tool","tool":"memory_write","args":{"workspace":"go-app","content":"skip"},"status":"ok","latency_ms":5}`+"\n"+
			`{"kind":"mcp_tool","tool":"memory_query","args":{"workspace":"go-app","query":"HybridSearch"},"status":"ok","latency_ms":10,"response":"{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"results\\\":[{\\\"id\\\":\\\"1\\\",\\\"source_path\\\":\\\"internal/search/service.go\\\"}]}\"}]}"}`+"\n",
	), 0644)

	saveJSON := filepath.Join(tmpDir, "out", "report.json")
	saveMD := filepath.Join(tmpDir, "out", "report.md")

	var stdout, stderr bytes.Buffer
	code := executeBenchEval(context.Background(), []string{
		"--url", srv.URL,
		"--suite", "go,replay",
		"--profiles", profilesPath,
		"--replay-logs", replayLog,
		"--save", saveJSON,
		"--markdown", saveMD,
		"--json",
	}, "", &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stderr=%s)", code, stderr.String())
	}

	var parsed evalbench.EvalReport
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		t.Fatalf("unmarshal stdout JSON: %v", err)
	}
	if parsed.Mutated || parsed.OverallRecall != 1.0 || parsed.Replay == nil || parsed.Replay.SkippedMutations != 1 || parsed.Replay.TotalReplayed != 1 {
		t.Fatalf("unexpected report: %+v (replay=%+v)", parsed, parsed.Replay)
	}
	if _, err := os.Stat(saveJSON); err != nil {
		t.Fatalf("saved JSON missing: %v", err)
	}
	if _, err := os.Stat(saveMD); err != nil {
		t.Fatalf("saved MD missing: %v", err)
	}

	// Compare against a higher-recall synthetic baseline -> must exit 1!
	highBaselinePath := filepath.Join(tmpDir, "high_baseline.json")
	degraded := parsed
	degraded.OverallRecall = 1.0
	// Create a baseline expecting 1.0 while running a dataset task that misses -> exit 1!
	missDsPath := filepath.Join(tmpDir, "miss_dataset.json")
	_ = os.WriteFile(missDsPath, []byte(strings.ReplaceAll(dsContent, "HybridSearch", "NonExistentSymbolXYZ")), 0644)
	_ = os.WriteFile(highBaselinePath, stdout.Bytes(), 0644)

	stdout.Reset()
	stderr.Reset()
	cmpCode := executeBenchEval(context.Background(), []string{
		"--url", srv.URL,
		"--suite", "go",
		"--workspace", "go-app",
		"--dataset", missDsPath,
		"--compare", highBaselinePath,
	}, "", &stdout, &stderr)
	if cmpCode != 1 || !strings.Contains(stderr.String(), "REGRESSION") {
		t.Fatalf("expected exit 1 with REGRESSION on stderr, got code=%d stderr=%s", cmpCode, stderr.String())
	}

	// Simulate concurrent workspace count change: --strict-immutable (default true) exits 1,
	// whereas --strict-immutable=false exits 0 while setting Mutated=true!
	wsCalls.Store(0)
	mutateAfterFirstSnapshot.Store(true)
	stdout.Reset()
	stderr.Reset()
	strictCode := executeBenchEval(context.Background(), []string{
		"--url", srv.URL,
		"--suite", "go",
		"--profiles", profilesPath,
	}, "", &stdout, &stderr)
	if strictCode != 1 || !strings.Contains(stderr.String(), "IMMUTABILITY CHECK FAILED") {
		t.Fatalf("expected exit 1 on strict immutability violation, got code=%d stderr=%s", strictCode, stderr.String())
	}

	wsCalls.Store(0)
	stdout.Reset()
	stderr.Reset()
	nonStrictCode := executeBenchEval(context.Background(), []string{
		"--url", srv.URL,
		"--suite", "go",
		"--profiles", profilesPath,
		"--strict-immutable=false",
		"--json",
	}, "", &stdout, &stderr)
	if nonStrictCode != 0 {
		t.Fatalf("expected exit 0 with --strict-immutable=false, got code=%d stderr=%s", nonStrictCode, stderr.String())
	}
	var mutatedRep evalbench.EvalReport
	_ = json.Unmarshal(stdout.Bytes(), &mutatedRep)
	if !mutatedRep.Mutated {
		t.Fatal("expected Mutated=true in report even when --strict-immutable=false")
	}
}
