package search

import (
	"testing"
)

func TestDeduplicateResults_Empty(t *testing.T) {
	results := DeduplicateResults(nil)
	if len(results) != 0 {
		t.Errorf("expected empty, got %d", len(results))
	}
}

func TestDeduplicateResults_Single(t *testing.T) {
	r := []Result{{ID: "1", DocumentID: "doc1", Content: "hello"}}
	results := DeduplicateResults(r)
	if len(results) != 1 {
		t.Errorf("expected 1, got %d", len(results))
	}
}

func TestDeduplicateResults_SameDocumentDifferentChunks(t *testing.T) {
	r := []Result{
		{ID: "chunk1", DocumentID: "doc1", Content: "func A() {}", Score: 0.5},
		{ID: "chunk2", DocumentID: "doc1", Content: "func B() {}", Score: 0.8},
	}
	results := DeduplicateResults(r)
	if len(results) != 1 {
		t.Fatalf("expected 1, got %d", len(results))
	}
	if results[0].ID != "chunk2" {
		t.Errorf("expected chunk2 (higher score), got %s", results[0].ID)
	}
}

func TestDeduplicateResults_SameContentHash(t *testing.T) {
	r := []Result{
		{ID: "1", DocumentID: "doc1", Content: "identical", SourcePath: "/long/path/to/file.js"},
		{ID: "2", DocumentID: "doc2", Content: "identical", SourcePath: "/short/file.js"},
	}
	results := DeduplicateResults(r)
	if len(results) != 1 {
		t.Fatalf("expected 1, got %d", len(results))
	}
	if results[0].SourcePath != "/short/file.js" {
		t.Errorf("expected shorter path, got %s", results[0].SourcePath)
	}
}

func TestDeduplicateResults_DifferentContent(t *testing.T) {
	r := []Result{
		{ID: "1", DocumentID: "doc1", Content: "hello"},
		{ID: "2", DocumentID: "doc2", Content: "world"},
	}
	results := DeduplicateResults(r)
	if len(results) != 2 {
		t.Errorf("expected 2, got %d", len(results))
	}
}

func TestDeduplicateResults_PreservesOrder(t *testing.T) {
	r := []Result{
		{ID: "1", DocumentID: "doc1", Content: "a"},
		{ID: "2", DocumentID: "doc2", Content: "b"},
		{ID: "3", DocumentID: "doc3", Content: "c"},
	}
	results := DeduplicateResults(r)
	if len(results) != 3 {
		t.Fatalf("expected 3, got %d", len(results))
	}
	for i, expected := range []string{"1", "2", "3"} {
		if results[i].ID != expected {
			t.Errorf("position %d: expected %s, got %s", i, expected, results[i].ID)
		}
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"/foo/bar.js", "/foo/bar.js"},
		{"/Foo/Bar.JS", "/foo/bar.js"},
		{"/a//b/c.js", "/a/b/c.js"},
		{"/.agents/_flows/index.html", "/.agent/_flows/index.html"},
		{"/.agent/_flows/index.html", "/.agent/_flows/index.html"},
	}
	for _, tt := range tests {
		got := normalizePath(tt.input)
		if got != tt.want {
			t.Errorf("normalizePath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
