package graph_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
)

func TestPythonGraphExtractor_Supports(t *testing.T) {
	ex, err := graph.NewPythonGraphExtractor()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		ext  string
		want bool
	}{
		{".py", true},
		{".go", false},
		{".js", false},
	}
	for _, tt := range tests {
		if got := ex.Supports(tt.ext); got != tt.want {
			t.Errorf("Supports(%q) = %v, want %v", tt.ext, got, tt.want)
		}
	}
}

func TestPythonGraphExtractor_EmptyFile(t *testing.T) {
	ex, err := graph.NewPythonGraphExtractor()
	if err != nil {
		t.Fatal(err)
	}
	edges, err := ex.ExtractEdges("empty.py", []byte(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 0 {
		t.Errorf("expected 0 edges from empty file, got %d", len(edges))
	}
}

func TestPythonGraphExtractor_CommentsOnly(t *testing.T) {
	ex, err := graph.NewPythonGraphExtractor()
	if err != nil {
		t.Fatal(err)
	}
	edges, err := ex.ExtractEdges("comments.py", []byte("# just a comment\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 0 {
		t.Errorf("expected 0 edges, got %d", len(edges))
	}
}

func TestPythonGraphExtractor_SyntaxError(t *testing.T) {
	ex, err := graph.NewPythonGraphExtractor()
	if err != nil {
		t.Fatal(err)
	}
	src := []byte("def broken(\n    pass\nimport os\n")
	edges, err := ex.ExtractEdges("broken.py", src)
	if err != nil {
		t.Fatal("should not error on partial parse:", err)
	}
	_ = edges
}

func TestPythonGraphExtractor_Fixture(t *testing.T) {
	ex, err := graph.NewPythonGraphExtractor()
	if err != nil {
		t.Fatal(err)
	}

	fixturePath := filepath.Join("testdata", "python", "sample.py")
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

	if len(imports) < 2 {
		t.Errorf("expected >=2 import edges (os, pathlib), got %d: %v", len(imports), imports)
	}

	if len(contains) < 4 {
		t.Errorf("expected >=4 contains edges (main, helper, Server, CONFIG), got %d: %v", len(contains), contains)
	}

	foundConfig := false
	for _, e := range contains {
		if e.TargetNode == fixturePath+"::CONFIG" {
			foundConfig = true
		}
	}
	if !foundConfig {
		t.Error("expected module-level CONFIG assignment as contains edge")
	}

	if len(calls) == 0 {
		t.Error("expected >=1 call edges")
	}

	foundCall := false
	foundAttrCall := false
	for _, e := range calls {
		if e.TargetNode == "helper" {
			foundCall = true
		}
		if e.TargetNode == "join" {
			foundAttrCall = true
		}
	}
	if !foundCall {
		t.Error("expected call edge to helper")
	}
	if !foundAttrCall {
		t.Error("expected call edge to join (attribute call os.path.join)")
	}

	for _, e := range edges {
		if e.SourceFile == "" {
			t.Errorf("edge missing SourceFile: %+v", e)
		}
		if e.Language != "python" {
			t.Errorf("edge wrong Language: %+v", e)
		}
		if e.Line == 0 {
			t.Errorf("edge has zero Line: %+v", e)
		}
	}
}

func TestPythonGraphExtractor_AttributeCall(t *testing.T) {
	ex, err := graph.NewPythonGraphExtractor()
	if err != nil {
		t.Fatal(err)
	}
	src := []byte(`def process():
    os.path.join("/tmp", "file")
    logger.info("hello")
    direct_call()
`)
	edges, err := ex.ExtractEdges("test.py", src)
	if err != nil {
		t.Fatal(err)
	}
	callees := map[string]bool{}
	for _, e := range edges {
		if e.Kind == graph.EdgeCalls {
			callees[e.TargetNode] = true
		}
	}
	if !callees["join"] {
		t.Error("expected call edge for os.path.join")
	}
	if !callees["info"] {
		t.Error("expected call edge for logger.info")
	}
	if !callees["direct_call"] {
		t.Error("expected call edge for direct_call")
	}
}

func TestPythonGraphExtractor_NestedAssignment(t *testing.T) {
	ex, err := graph.NewPythonGraphExtractor()
	if err != nil {
		t.Fatal(err)
	}
	src := []byte(`MODULE_VAR = 1

def func():
    local_var = 2
`)
	edges, err := ex.ExtractEdges("test.py", src)
	if err != nil {
		t.Fatal(err)
	}

	for _, e := range edges {
		if e.Kind == graph.EdgeContains && e.TargetNode == "test.py::local_var" {
			t.Error("nested assignment local_var should NOT produce contains edge")
		}
	}

	foundModule := false
	for _, e := range edges {
		if e.Kind == graph.EdgeContains && e.TargetNode == "test.py::MODULE_VAR" {
			foundModule = true
		}
	}
	if !foundModule {
		t.Error("module-level MODULE_VAR should produce contains edge")
	}
}
