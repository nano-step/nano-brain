package graph_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
)

func TestGoGraphExtractor_Supports(t *testing.T) {
	ex, err := graph.NewGoGraphExtractor()
	if err != nil {
		t.Fatal(err)
	}
	if !ex.Supports(".go") {
		t.Error("should support .go")
	}
	if ex.Supports(".ts") {
		t.Error("should not support .ts")
	}
}

func TestGoGraphExtractor_Empty(t *testing.T) {
	ex, err := graph.NewGoGraphExtractor()
	if err != nil {
		t.Fatal(err)
	}
	edges, err := ex.ExtractEdges("empty.go", []byte("package empty\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range edges {
		if e.Kind == graph.EdgeImports || e.Kind == graph.EdgeCalls {
			t.Errorf("unexpected edge in empty file: %+v", e)
		}
	}
}

func TestGoGraphExtractor_Fixture(t *testing.T) {
	ex, err := graph.NewGoGraphExtractor()
	if err != nil {
		t.Fatal(err)
	}

	fixturePath := filepath.Join("testdata", "simple", "main.go")
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}

	edges, err := ex.ExtractEdges(fixturePath, content)
	if err != nil {
		t.Fatal(err)
	}

	var contains, imports, calls []graph.Edge
	for _, e := range edges {
		switch e.Kind {
		case graph.EdgeContains:
			contains = append(contains, e)
		case graph.EdgeImports:
			imports = append(imports, e)
		case graph.EdgeCalls:
			calls = append(calls, e)
		}
	}

	if len(contains) < 3 {
		t.Errorf("expected ≥3 contains edges, got %d", len(contains))
	}

	if len(imports) < 2 {
		t.Errorf("expected ≥2 import edges, got %d: %v", len(imports), imports)
	}

	foundGraphImport := false
	for _, e := range imports {
		if e.TargetNode == "github.com/nano-brain/nano-brain/internal/graph" {
			foundGraphImport = true
		}
	}
	if !foundGraphImport {
		t.Errorf("expected import of internal/graph, got: %v", imports)
	}

	if len(calls) == 0 {
		t.Errorf("expected ≥1 call edges, got 0")
	}

	for _, e := range edges {
		if e.SourceFile == "" {
			t.Errorf("edge missing SourceFile: %+v", e)
		}
		if e.Language != "go" {
			t.Errorf("edge wrong Language: %+v", e)
		}
		if e.Line == 0 {
			t.Errorf("edge has zero Line: %+v", e)
		}
	}
}

func TestGoGraphExtractor_NoPanic_LargeFile(t *testing.T) {
	ex, err := graph.NewGoGraphExtractor()
	if err != nil {
		t.Fatal(err)
	}
	src := make([]byte, 0, 64*1024)
	src = append(src, "package bigfile\n\nimport \"fmt\"\n"...)
	for i := 0; i < 500; i++ {
		src = append(src, []byte("func noop() { fmt.Println() }\n")...)
	}
	edges, err := ex.ExtractEdges("big.go", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) == 0 {
		t.Error("expected edges from large file")
	}
}
