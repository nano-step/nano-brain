//go:build integration

package bench

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/nano-brain/nano-brain/internal/search"
)

type httpSearcher struct {
	baseURL    string
	workspace  string
	httpClient *http.Client
}

func (h *httpSearcher) HybridSearch(ctx context.Context, query string, workspace string, maxResults int, tags []string, timeRange *search.TimeRangeFilter, chunkType string) ([]search.Result, error) {
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

func TestBenchmarkNanoBrain(t *testing.T) {
	searcher := &httpSearcher{
		baseURL:    "http://host.docker.internal:3100",
		workspace:  "7f443561795a6fea64b6e8d35a9b06ed4d216b8a27af5e10e7137b261ade061f",
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}

	dataset := NanoBrainDataset()

	results, err := Run(context.Background(), dataset, searcher, "v0.3.0-dedup+ranking")
	if err != nil {
		t.Fatalf("benchmark failed: %v", err)
	}

	fmt.Println("=== Nano-Brain Search Benchmark (Post-Improvements) ===")
	fmt.Printf("Version:     %s\n", results.Version)
	fmt.Printf("Queries:     %d\n", results.QueryCount)
	fmt.Printf("Workspace:   %s...\n", results.WorkspaceHash[:16])
	fmt.Println()
	fmt.Println("--- Doc ID Metrics ---")
	fmt.Printf("P@5:   %.1f%%\n", results.PrecisionAt5*100)
	fmt.Printf("R@10:  %.1f%%\n", results.RecallAt10*100)
	fmt.Printf("MRR:   %.3f\n", results.MRR)
	fmt.Println()
	fmt.Println("--- Path Metrics ---")
	fmt.Printf("P@5:   %.1f%%\n", results.PrecisionAt5Paths*100)
	fmt.Printf("R@10:  %.1f%%\n", results.RecallAt10Paths*100)
	fmt.Printf("MRR:   %.3f\n", results.MRRPaths)
	fmt.Println()
	fmt.Println("--- Latency ---")
	fmt.Printf("P50:   %.1fms\n", results.QueryP50ms)
	fmt.Printf("P95:   %.1fms\n", results.QueryP95ms)

	if results.PrecisionAt5Paths < 0.05 {
		t.Errorf("P@5 (paths) too low: %.1f%% (expected >= 5%%)", results.PrecisionAt5Paths*100)
	}
}
