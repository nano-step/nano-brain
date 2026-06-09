//go:build benchmark

package search_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/nano-brain/nano-brain/internal/search"
)

type queryAPIResponse struct {
	Results []queryAPIResult `json:"results"`
}

type queryAPIResult struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	Snippet       string  `json:"snippet"`
	Score         float64 `json:"score"`
	Collection    string  `json:"collection"`
	WorkspaceHash string  `json:"workspace_hash"`
	SourcePath    string  `json:"source_path"`
	DocumentID    string  `json:"document_id"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

// apiResultToSearchResult maps an API response result to a search.Result.
// The API returns snippet (not full content) — we use snippet as Content
// so the existing scoring logic works on available text.
func apiResultToSearchResult(r queryAPIResult) search.Result {
	return search.Result{
		ID:            r.ID,
		DocumentID:    r.DocumentID,
		WorkspaceHash: r.WorkspaceHash,
		Title:         r.Title,
		Content:       r.Snippet,
		Snippet:       r.Snippet,
		Score:         r.Score,
		Collection:    r.Collection,
		SourcePath:    r.SourcePath,
	}
}

// callQueryAPI sends a query to the nano-brain hybrid search API and returns
// the results. Uses NANO_BRAIN_SERVER (default http://localhost:3100) and
// NANO_BRAIN_BENCH_WORKSPACE (default: nano-brain workspace hash).
func callQueryAPI(ctx context.Context, query string, limit int) ([]search.Result, error) {
	serverURL := os.Getenv("NANO_BRAIN_SERVER")
	if serverURL == "" {
		serverURL = "http://localhost:3100"
	}
	workspace := os.Getenv("NANO_BRAIN_BENCH_WORKSPACE")
	if workspace == "" {
		workspace = "7f443561795a6fea64b6e8d35a9b06ed4d216b8a27af5e10e7137b261ade061f"
	}

	body := map[string]interface{}{
		"workspace": workspace,
		"query":     query,
		"limit":     limit,
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, serverURL+"/api/v1/query", bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp queryAPIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	results := make([]search.Result, len(apiResp.Results))
	for i, r := range apiResp.Results {
		results[i] = apiResultToSearchResult(r)
	}
	return results, nil
}

type BenchmarkQuery struct {
	Query            string   `json:"query"`
	Category         string   `json:"category"`
	ExpectedKeywords []string `json:"expected_keywords"`
	ExpectedSymbols  []string `json:"expected_symbols"`
	RelevanceHints   []string `json:"relevance_hints"`
}

type BenchmarkReport struct {
	OverallNDCG5  float64
	OverallNDCG10 float64
	OverallMRR    float64
	ByCategory    map[string]CategoryReport
}

type CategoryReport struct {
	NDCG5    float64
	NDCG10   float64
	Recall5  float64
	Recall10 float64
	MRR      float64
	Count    int
}

func loadBenchmarkQueries(t *testing.T) []BenchmarkQuery {
	t.Helper()
	data, err := os.ReadFile("../../testdata/search_benchmarks.json")
	if err != nil {
		t.Fatalf("failed to load benchmark fixtures: %v", err)
	}
	var queries []BenchmarkQuery
	if err := json.Unmarshal(data, &queries); err != nil {
		t.Fatalf("failed to parse benchmark fixtures: %v", err)
	}
	return queries
}

func scoreResult(result search.Result, query BenchmarkQuery) float64 {
	var score float64
	content := strings.ToLower(result.Content + " " + result.Title + " " + result.SourcePath)

	for _, kw := range query.ExpectedKeywords {
		if strings.Contains(content, strings.ToLower(kw)) {
			score += 1.0
		}
	}

	for _, sym := range query.ExpectedSymbols {
		if strings.Contains(content, strings.ToLower(sym)) {
			score += 2.0
		}
	}

	maxPossible := float64(len(query.ExpectedKeywords)) + float64(len(query.ExpectedSymbols))*2
	if maxPossible == 0 {
		return 0
	}
	return score / maxPossible
}

func isRelevant(result search.Result, query BenchmarkQuery) bool {
	return scoreResult(result, query) >= 0.3
}

func evaluateResults(results []search.Result, query BenchmarkQuery) (ndcg5, ndcg10, recall5, recall10 float64, firstRelevantRank int) {
	relevanceScores := make([]float64, len(results))
	for i, r := range results {
		relevanceScores[i] = scoreResult(r, query)
	}

	ndcg5 = search.NDCG(relevanceScores, 5)
	ndcg10 = search.NDCG(relevanceScores, 10)

	totalRelevant := len(query.ExpectedKeywords) + len(query.ExpectedSymbols)
	if totalRelevant > 5 {
		totalRelevant = 5
	}

	var found5, found10 int
	firstRelevantRank = 0
	for i, r := range results {
		if isRelevant(r, query) {
			if firstRelevantRank == 0 {
				firstRelevantRank = i + 1
			}
			if i < 5 {
				found5++
			}
			if i < 10 {
				found10++
			}
		}
	}

	recall5 = search.Recall(found5, totalRelevant)
	recall10 = search.Recall(found10, totalRelevant)
	return
}

func TestSearchBenchmark(t *testing.T) {
	queries := loadBenchmarkQueries(t)
	if len(queries) < 50 {
		t.Fatalf("expected at least 50 benchmark queries, got %d", len(queries))
	}

	t.Logf("Loaded %d benchmark queries", len(queries))
	serverURL := os.Getenv("NANO_BRAIN_SERVER")
	if serverURL == "" {
		serverURL = "http://localhost:3100"
	}
	t.Logf("Using nano-brain server: %s", serverURL)

	report := BenchmarkReport{
		ByCategory: make(map[string]CategoryReport),
	}

	ctx := context.Background()
	var totalFailures int
	var allNDCG5, allNDCG10 []float64
	var allMRRRanks []int

	categoryNDCG5 := make(map[string][]float64)
	categoryNDCG10 := make(map[string][]float64)
	categoryRecall5 := make(map[string][]float64)
	categoryRecall10 := make(map[string][]float64)
	categoryMRR := make(map[string][]int)
	categoryCounts := make(map[string]int)

	for _, q := range queries {
		results, err := callQueryAPI(ctx, q.Query, 20)
		if err != nil {
			t.Logf("WARNING: query %q failed: %v", q.Query, err)
			totalFailures++
			continue
		}

		ndcg5, ndcg10, recall5, recall10, firstRank := evaluateResults(results, q)

		allNDCG5 = append(allNDCG5, ndcg5)
		allNDCG10 = append(allNDCG10, ndcg10)
		allMRRRanks = append(allMRRRanks, firstRank)

		categoryNDCG5[q.Category] = append(categoryNDCG5[q.Category], ndcg5)
		categoryNDCG10[q.Category] = append(categoryNDCG10[q.Category], ndcg10)
		categoryRecall5[q.Category] = append(categoryRecall5[q.Category], recall5)
		categoryRecall10[q.Category] = append(categoryRecall10[q.Category], recall10)
		categoryMRR[q.Category] = append(categoryMRR[q.Category], firstRank)
		categoryCounts[q.Category]++
	}

	report.OverallNDCG5 = search.MeanNDCG(allNDCG5)
	report.OverallNDCG10 = search.MeanNDCG(allNDCG10)
	report.OverallMRR = search.MeanMRR(allMRRRanks)

	for cat := range categoryCounts {
		report.ByCategory[cat] = CategoryReport{
			NDCG5:    search.MeanNDCG(categoryNDCG5[cat]),
			NDCG10:   search.MeanNDCG(categoryNDCG10[cat]),
			Recall5:  search.MeanNDCG(categoryRecall5[cat]),
			Recall10: search.MeanNDCG(categoryRecall10[cat]),
			MRR:      search.MeanMRR(categoryMRR[cat]),
			Count:    categoryCounts[cat],
		}
	}

	if totalFailures > 0 {
		t.Logf("WARNING: %d queries failed to reach the server (results excluded from metrics)", totalFailures)
	}

	t.Log("\n=== Search Benchmark Results ===")
	t.Logf("Overall nDCG@5:  %.4f", report.OverallNDCG5)
	t.Logf("Overall nDCG@10: %.4f", report.OverallNDCG10)
	t.Logf("Overall MRR:     %.4f", report.OverallMRR)
	t.Log("")

	for cat, cr := range report.ByCategory {
		t.Logf("Category: %s (n=%d)", cat, cr.Count)
		t.Logf("  nDCG@5:    %.4f", cr.NDCG5)
		t.Logf("  nDCG@10:   %.4f", cr.NDCG10)
		t.Logf("  Recall@5:  %.4f", cr.Recall5)
		t.Logf("  Recall@10: %.4f", cr.Recall10)
		t.Logf("  MRR:       %.4f", cr.MRR)
	}

	ndcg5Threshold := getEnvFloat("BENCH_NDCG5_THRESHOLD", 0.0)
	ndcg5MaxDrop := getEnvFloat("BENCH_NDCG5_MAX_DROP", 0.05)
	baselineNDCG5 := getEnvFloat("BENCH_BASELINE_NDCG5", 0.0)

	if ndcg5Threshold > 0 && report.OverallNDCG5 < ndcg5Threshold {
		t.Errorf("QUALITY GATE FAIL: nDCG@5 %.4f < threshold %.4f", report.OverallNDCG5, ndcg5Threshold)
	}

	if baselineNDCG5 > 0 {
		drop := baselineNDCG5 - report.OverallNDCG5
		if drop > ndcg5MaxDrop {
			t.Errorf("QUALITY GATE FAIL: nDCG@5 dropped %.4f from baseline %.4f (max allowed drop: %.4f)",
				drop, baselineNDCG5, ndcg5MaxDrop)
		}
		improvement := report.OverallNDCG5 - baselineNDCG5
		if improvement >= 0.03 {
			t.Logf("QUALITY GATE PASS: nDCG@5 improved by %.4f (>=3%%)", improvement)
		}
	}

	reportJSON, _ := json.MarshalIndent(report, "", "  ")
	t.Logf("\nJSON Report:\n%s", string(reportJSON))

	outPath := os.Getenv("BENCH_REPORT_PATH")
	if outPath != "" {
		if err := os.WriteFile(outPath, reportJSON, 0644); err != nil {
			t.Logf("WARNING: failed to write report to %s: %v", outPath, err)
		} else {
			t.Logf("Report written to %s", outPath)
		}
	}
}

func getEnvFloat(key string, defaultVal float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	var f float64
	_, err := fmt.Sscanf(v, "%f", &f)
	if err != nil {
		return defaultVal
	}
	return f
}
