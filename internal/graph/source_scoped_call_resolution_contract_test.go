package graph_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
)

var canonicalJSCallTargetContract = regexp.MustCompile(`^(?:(?:[A-Za-z0-9_$-]+/)*[A-Za-z0-9_$-]+(?:\.[A-Za-z0-9_$-]+)*\.(?:js|jsx|mjs|cjs|ts|tsx|mts|cts))::(?:[A-Za-z_$][A-Za-z0-9_$]*|[A-Za-z_$][A-Za-z0-9_$]*\.[A-Za-z_$][A-Za-z0-9_$]*|default|default\.[A-Za-z_$][A-Za-z0-9_$]*)$`)

// TestCanonicalJSCallTargetContract is the test oracle for the public
// IsCanonicalJSCallTarget helper Task 2.1 must add. Keeping the oracle here
// lets this task lock the contract without changing runtime behavior.
func TestCanonicalJSCallTargetContract(t *testing.T) {
	cases := []struct {
		name   string
		target string
		want   bool
	}{
		{"function", "src/api.ts::run", true},
		{"class-method", "src/api.ts::Api.run", true},
		{"default", "src/api.ts::default", true},
		{"default-method", "src/api.ts::default.run", true},
		{"all-admitted-extensions", "src/api.mts::run", true},
		{"dotted-filename", "src/foo.d.ts::run", true},
		{"bare", "run", false},
		{"unresolved", "<unresolved>", false},
		{"ruby-legacy", "Api::V2::StoriesController#sync", false},
		{"absolute", "/src/api.ts::run", false},
		{"parent-segment", "src/../api.ts::run", false},
		{"too-many-symbol-parts", "src/api.ts::Api.inner.run", false},
		{"unsupported-extension", "src/api.rb::run", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := canonicalJSCallTargetContract.MatchString(tc.target); got != tc.want {
				t.Fatalf("contract oracle(%q) = %t, want %t", tc.target, got, tc.want)
			}
			if got := graph.IsCanonicalJSCallTarget(tc.target); got != tc.want {
				t.Fatalf("production predicate(%q) = %t, want %t", tc.target, got, tc.want)
			}
		})
	}
}

func TestJavaScriptAndTypeScriptExtractorsSupportCanonicalModuleExtensions(t *testing.T) {
	js, err := graph.NewJavaScriptGraphExtractor()
	if err != nil {
		t.Fatalf("NewJavaScriptGraphExtractor: %v", err)
	}
	ts, err := graph.NewTypeScriptGraphExtractor()
	if err != nil {
		t.Fatalf("NewTypeScriptGraphExtractor: %v", err)
	}

	for _, ext := range []string{".js", ".jsx", ".mjs", ".cjs"} {
		if !js.Supports(ext) {
			t.Errorf("JavaScript extractor does not support %q", ext)
		}
	}
	for _, ext := range []string{".ts", ".tsx", ".mts", ".cts"} {
		if !ts.Supports(ext) {
			t.Errorf("TypeScript extractor does not support %q", ext)
		}
	}
}

func TestContextualCallTargetsRemainMetadataFree(t *testing.T) {
	root := t.TempDir()
	writeContractFixture(t, root, "repo-a/lib/missing.ts", "")
	extractor, err := graph.NewTypeScriptGraphExtractor()
	if err != nil {
		t.Fatalf("NewTypeScriptGraphExtractor: %v", err)
	}
	edges, err := extractor.ExtractEdgesWithImportContext(
		"repo-a/consumer.ts",
		[]byte("import { run } from \"./lib/missing\"; function caller() { run(); }"),
		graph.ImportContext{Exists: graph.DiskExistsChecker(root)},
	)
	if err != nil {
		t.Fatalf("ExtractEdgesWithImportContext: %v", err)
	}
	for _, edge := range edges {
		if edge.Kind == graph.EdgeCalls && edge.Metadata != nil {
			t.Fatalf("contextual call metadata = %#v, want no reason or raw-callee metadata", edge.Metadata)
		}
	}
}

func contextualCallTargets(t *testing.T, filePath, source string, root string) []string {
	t.Helper()

	extractor, err := graph.NewTypeScriptGraphExtractor()
	if err != nil {
		t.Fatalf("NewTypeScriptGraphExtractor: %v", err)
	}
	edges, err := extractor.ExtractEdgesWithImportContext(filePath, []byte(source), graph.ImportContext{
		Exists: graph.DiskExistsChecker(root),
		ReadFile: func(path string) ([]byte, error) {
			return os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		},
	})
	if err != nil {
		t.Fatalf("ExtractEdgesWithImportContext: %v", err)
	}

	var targets []string
	for _, edge := range edges {
		if edge.Kind == graph.EdgeCalls {
			targets = append(targets, edge.TargetNode)
		}
	}
	return targets
}

func javascriptContextualCallTargets(t *testing.T, filePath, source string, root string) []string {
	t.Helper()

	extractor, err := graph.NewJavaScriptGraphExtractor()
	if err != nil {
		t.Fatalf("NewJavaScriptGraphExtractor: %v", err)
	}
	edges, err := extractor.ExtractEdgesWithImportContext(filePath, []byte(source), graph.ImportContext{
		Exists: graph.DiskExistsChecker(root),
		ReadFile: func(path string) ([]byte, error) {
			return os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		},
	})
	if err != nil {
		t.Fatalf("ExtractEdgesWithImportContext: %v", err)
	}

	var targets []string
	for _, edge := range edges {
		if edge.Kind == graph.EdgeCalls {
			targets = append(targets, edge.TargetNode)
		}
	}
	return targets
}

func writeContractFixture(t *testing.T, root, relativePath, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", relativePath, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", relativePath, err)
	}
}

func requireCallTargets(t *testing.T, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("contextual call targets = %q, want %q", got, want)
	}
}
