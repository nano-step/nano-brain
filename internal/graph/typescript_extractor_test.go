package graph_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
)

func TestTypeScriptGraphExtractor_Supports(t *testing.T) {
	ex, err := graph.NewTypeScriptGraphExtractor()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		ext  string
		want bool
	}{
		{".ts", true},
		{".tsx", true},
		{".go", false},
		{".js", false},
	}
	for _, tt := range tests {
		if got := ex.Supports(tt.ext); got != tt.want {
			t.Errorf("Supports(%q) = %v, want %v", tt.ext, got, tt.want)
		}
	}
}

func TestTypeScriptGraphExtractor_EmptyFile(t *testing.T) {
	ex, err := graph.NewTypeScriptGraphExtractor()
	if err != nil {
		t.Fatal(err)
	}
	edges, err := ex.ExtractEdges("empty.ts", []byte(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 0 {
		t.Errorf("expected 0 edges from empty file, got %d", len(edges))
	}
}

func TestTypeScriptGraphExtractor_CommentsOnly(t *testing.T) {
	ex, err := graph.NewTypeScriptGraphExtractor()
	if err != nil {
		t.Fatal(err)
	}
	src := []byte("// just a comment\n/* block comment */\n")
	edges, err := ex.ExtractEdges("comments.ts", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 0 {
		t.Errorf("expected 0 edges from comments-only file, got %d", len(edges))
	}
}

func TestTypeScriptGraphExtractor_SyntaxError(t *testing.T) {
	ex, err := graph.NewTypeScriptGraphExtractor()
	if err != nil {
		t.Fatal(err)
	}
	src := []byte("function broken( { return }\nimport { x } from \"mod\";\n")
	edges, err := ex.ExtractEdges("broken.ts", src)
	if err != nil {
		t.Fatal("should not error on partial parse:", err)
	}
	_ = edges
}

func TestTypeScriptGraphExtractor_Fixture(t *testing.T) {
	ex, err := graph.NewTypeScriptGraphExtractor()
	if err != nil {
		t.Fatal(err)
	}

	fixturePath := filepath.Join("testdata", "typescript", "sample.ts")
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

	if len(imports) < 3 {
		t.Errorf("expected >=3 import edges (fs, path, lodash), got %d: %v", len(imports), imports)
	}

	foundRequire := false
	for _, e := range imports {
		if e.TargetNode == "lodash" {
			foundRequire = true
		}
	}
	if !foundRequire {
		t.Error("expected require('lodash') to produce import edge")
	}

	if len(contains) < 4 {
		t.Errorf("expected >=4 contains edges, got %d", len(contains))
	}

	if len(calls) == 0 {
		t.Error("expected >=1 call edges")
	}

	foundCall := false
	for _, e := range calls {
		if e.TargetNode == "processData" {
			foundCall = true
		}
	}
	if !foundCall {
		t.Error("expected call edge to processData")
	}

	for _, e := range edges {
		if e.SourceFile == "" {
			t.Errorf("edge missing SourceFile: %+v", e)
		}
		if e.Language != "typescript" {
			t.Errorf("edge wrong Language: %+v", e)
		}
		if e.Line == 0 {
			t.Errorf("edge has zero Line: %+v", e)
		}
	}
}

func TestTypeScriptGraphExtractor_ArrowFunction(t *testing.T) {
	ex, err := graph.NewTypeScriptGraphExtractor()
	if err != nil {
		t.Fatal(err)
	}
	src := []byte(`const handler = () => {
  processData();
  console.log("done");
};
`)
	edges, err := ex.ExtractEdges("test.ts", src)
	if err != nil {
		t.Fatal(err)
	}
	callees := map[string]bool{}
	for _, e := range edges {
		if e.Kind == graph.EdgeCalls {
			callees[e.TargetNode] = true
		}
	}
	if !callees["processData"] {
		t.Error("expected call edge for processData inside arrow function")
	}
	if !callees["log"] {
		t.Error("expected call edge for console.log inside arrow function")
	}
}

func TestTypeScriptGraphExtractor_RequireVsOtherCalls(t *testing.T) {
	ex, err := graph.NewTypeScriptGraphExtractor()
	if err != nil {
		t.Fatal(err)
	}
	src := []byte(`const fs = require("fs");
console.log("hello");
`)
	edges, err := ex.ExtractEdges("test.ts", src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range edges {
		if e.Kind == graph.EdgeImports && e.TargetNode == "hello" {
			t.Error("console.log should not produce import edge")
		}
	}
	foundFsImport := false
	for _, e := range edges {
		if e.Kind == graph.EdgeImports && e.TargetNode == "fs" {
			foundFsImport = true
		}
	}
	if !foundFsImport {
		t.Error("require('fs') should produce import edge")
	}
}
