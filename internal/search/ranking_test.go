package search

import (
	"testing"
)

func TestApplyCodeAwareBoost_NoQuery(t *testing.T) {
	results := []Result{
		{ID: "1", SourcePath: "internal/handlers/search.go", Score: 1.0},
	}
	out := ApplyCodeAwareBoost(results, "", 1.2, 1.3)
	if out[0].Score != 1.0 {
		t.Errorf("expected score 1.0 with empty query, got %f", out[0].Score)
	}
}

func TestApplyCodeAwareBoost_PathMatch(t *testing.T) {
	results := []Result{
		{ID: "1", SourcePath: "internal/handlers/search.go", Score: 1.0},
		{ID: "2", SourcePath: "internal/config/config.go", Score: 1.0},
		{ID: "3", SourcePath: "README.md", Score: 1.0},
	}
	out := ApplyCodeAwareBoost(results, "search handler", 1.2, 1.0)

	if out[0].Score <= 1.0 {
		t.Errorf("expected boost for search.go path, got %f", out[0].Score)
	}
	if out[1].Score != 1.0 {
		t.Errorf("expected no boost for config.go, got %f", out[1].Score)
	}
	if out[2].Score != 1.0 {
		t.Errorf("expected no boost for README.md, got %f", out[2].Score)
	}
}

func TestApplyCodeAwareBoost_TitleMatch(t *testing.T) {
	results := []Result{
		{ID: "1", Title: "SearchHandler Implementation", SourcePath: "x.go", Score: 1.0},
		{ID: "2", Title: "Config Setup", SourcePath: "y.go", Score: 1.0},
	}
	out := ApplyCodeAwareBoost(results, "search handler", 1.0, 1.3)

	if out[0].Score <= 1.0 {
		t.Errorf("expected title boost for SearchHandler, got %f", out[0].Score)
	}
	if out[1].Score != 1.0 {
		t.Errorf("expected no title boost for Config, got %f", out[1].Score)
	}
}

func TestApplyCodeAwareBoost_CumulativeBoost(t *testing.T) {
	results := []Result{
		{ID: "1", Title: "Search Handler", SourcePath: "internal/search/handler.go", Score: 1.0},
	}
	out := ApplyCodeAwareBoost(results, "search", 1.2, 1.3)

	expected := 1.0 * 1.2 * 1.3
	if out[0].Score != expected {
		t.Errorf("expected cumulative boost %f, got %f", expected, out[0].Score)
	}
}

func TestApplyCodeAwareBoost_EmptyResults(t *testing.T) {
	out := ApplyCodeAwareBoost(nil, "search", 1.2, 1.3)
	if len(out) != 0 {
		t.Errorf("expected empty results, got %d", len(out))
	}
}

func TestExtractQueryKeywords_StopWords(t *testing.T) {
	kw := extractQueryKeywords("what is the search handler doing")
	found := false
	for _, k := range kw {
		if k == "search" || k == "handler" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'search' or 'handler' in keywords, got %v", kw)
	}
	for _, k := range kw {
		if k == "what" || k == "the" || k == "is" {
			t.Errorf("stop word '%s' should be filtered out", k)
		}
	}
}

func TestExtractQueryKeywords_Deduplication(t *testing.T) {
	kw := extractQueryKeywords("search search search")
	if len(kw) != 1 || kw[0] != "search" {
		t.Errorf("expected [search], got %v", kw)
	}
}

func TestExtractQueryKeywords_ShortTokens(t *testing.T) {
	kw := extractQueryKeywords("a go io os")
	for _, k := range kw {
		if len(k) < 2 {
			t.Errorf("short token '%s' should be filtered", k)
		}
	}
}

func TestApplyExtensionBoost_CodeAndDoc(t *testing.T) {
	results := []Result{
		{ID: "1", SourcePath: "service.go", Score: 1.0},
		{ID: "2", SourcePath: "handler.ts", Score: 1.0},
		{ID: "3", SourcePath: "README.md", Score: 1.0},
		{ID: "4", SourcePath: "notes.txt", Score: 1.0},
		{ID: "5", SourcePath: "data.json", Score: 1.0},
	}
	out := ApplyExtensionBoost(results, 1.1, 0.9)

	if out[0].Score != 1.1 {
		t.Errorf(".go: expected 1.1, got %f", out[0].Score)
	}
	if out[1].Score != 1.1 {
		t.Errorf(".ts: expected 1.1, got %f", out[1].Score)
	}
	if out[2].Score != 0.9 {
		t.Errorf(".md: expected 0.9, got %f", out[2].Score)
	}
	if out[3].Score != 0.9 {
		t.Errorf(".txt: expected 0.9, got %f", out[3].Score)
	}
	if out[4].Score != 1.0 {
		t.Errorf(".json: expected 1.0 (no change), got %f", out[4].Score)
	}
}
