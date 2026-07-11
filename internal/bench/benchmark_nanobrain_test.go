//go:build integration

package bench

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/nano-brain/nano-brain/internal/search"
)

type httpSearcher struct {
	baseURL    string
	workspace  string
	httpClient *http.Client
}

func (h *httpSearcher) HybridSearch(ctx context.Context, query string, workspace string, maxResults int, tags []string, timeRange *search.TimeRangeFilter, chunkType string, hypothetical string) ([]search.Result, error) {
	body := map[string]any{
		"query":       query,
		"workspace":   workspace,
		"max_results": maxResults * 3,
	}
	payload, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", h.baseURL+"/api/v1/query", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Results []search.Result `json:"results"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	filtered := make([]search.Result, 0, len(result.Results))
	for _, r := range result.Results {
		if r.Collection == "code" {
			filtered = append(filtered, r)
		}
	}
	if len(filtered) > maxResults {
		filtered = filtered[:maxResults]
	}
	return filtered, nil
}

func loadBaseline(t *testing.T) *BenchmarkResults {
	t.Helper()
	data, err := os.ReadFile("testdata/baseline_v1.json")
	if err != nil {
		t.Fatalf("failed to read baseline: %v", err)
	}
	var baseline BenchmarkResults
	if err := json.Unmarshal(data, &baseline); err != nil {
		t.Fatalf("failed to parse baseline: %v", err)
	}
	return &baseline
}

func saveResults(t *testing.T, results *BenchmarkResults) {
	t.Helper()
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal results: %v", err)
	}
	if err := os.WriteFile("testdata/results_current.json", data, 0644); err != nil {
		t.Fatalf("failed to write results: %v", err)
	}
}

func TestBenchmarkNanoBrain(t *testing.T) {
	baseURL := os.Getenv("NANO_BRAIN_URL")
	if baseURL == "" {
		baseURL = "http://localhost:3199"
	}
	searcher := &httpSearcher{
		baseURL:    baseURL,
		workspace:  "7f443561795a6fea64b6e8d35a9b06ed4d216b8a27af5e10e7137b261ade061f",
		httpClient: &http.Client{Timeout: 90 * time.Second},
	}

	dataset := NanoBrainDataset()

	results, err := Run(context.Background(), dataset, searcher, "v0.3.0-dedup+ranking")
	if err != nil {
		t.Fatalf("benchmark failed: %v", err)
	}

	saveResults(t, results)

	fmt.Println("=== Nano-Brain Search Benchmark ===")
	fmt.Printf("Dataset:     %s (scale=%d)\n", results.DatasetVersion, results.Scale)
	fmt.Printf("Version:     %s\n", results.Version)
	fmt.Printf("Workspace:   %s...\n", results.WorkspaceHash[:16])
	fmt.Println()
	fmt.Println("--- Path Metrics ---")
	fmt.Printf("P@5:   %.1f%%\n", results.PrecisionAt5Paths*100)
	fmt.Printf("R@10:  %.1f%%\n", results.RecallAt10Paths*100)
	fmt.Printf("MRR:   %.3f\n", results.MRRPaths)
	fmt.Println()
	fmt.Println("--- Latency ---")
	fmt.Printf("P50:   %.1fms\n", results.QueryP50ms)
	fmt.Printf("P95:   %.1fms\n", results.QueryP95ms)

	baseline := loadBaseline(t)
	cr := Compare(results, baseline)
	fmt.Println()
	if !cr.DatasetOK {
		fmt.Printf("❌ DATASET MISMATCH: baseline=%s, current=%s\n",
			baseline.DatasetVersion, results.DatasetVersion)
		t.Errorf("dataset version mismatch")
		return
	}

	fmt.Println("=== vs Baseline ===")
	for name, d := range cr.Deltas {
		sign := "+"
		if d.Change < 0 {
			sign = ""
		}
		fmt.Printf("%s:  %.3f → %.3f (%s%.3f)\n", name, d.Baseline, d.New, sign, d.Change)
	}
	fmt.Println()
	if cr.Passed {
		fmt.Println("✅ PASS — no regressions")
	} else {
		for _, r := range cr.Regressions {
			fmt.Printf("❌ REGRESSION: %s\n", r.Message)
		}
		t.Errorf("benchmark regressed: %d regressions", len(cr.Regressions))
	}
}
