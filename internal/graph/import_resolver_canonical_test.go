package graph_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/rs/zerolog"
)

// TestCanonicalForm_ResolvedTargetMatchesStoredSourceNode pins the load-bearing
// invariant of #501: the resolved import target_node byte-equals the stored
// source_node form of the imported file, and both are workspace-relative. If
// these ever diverge, reverse traversal silently returns zero rows again.
//
// It drives the real TypeScript extractor over the committed fixture, then runs
// the real resolver (with the real tsconfig-derived alias map and an os.Stat
// exists predicate rooted at the fixture), exactly as the watcher does.
func TestCanonicalForm_ResolvedTargetMatchesStoredSourceNode(t *testing.T) {
	logger := zerolog.Nop()
	root := mustAbs(t, "testdata/alias-import")

	ex, err := graph.NewTypeScriptGraphExtractor()
	if err != nil {
		t.Fatal(err)
	}

	// The watcher passes workspace-relative, forward-slashed paths to the
	// extractor, so SourceNode/SourceFile come back in that same form.
	const importer = "composables/useThing.ts"
	content, err := os.ReadFile(filepath.Join(root, importer))
	if err != nil {
		t.Fatal(err)
	}
	edges, err := ex.ExtractEdges(importer, content)
	if err != nil {
		t.Fatal(err)
	}

	alias := graph.LoadAliasMap(root, logger)
	exists := func(rel string) bool {
		fi, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		return err == nil && !fi.IsDir()
	}

	var importEdge *graph.Edge
	for i := range edges {
		if edges[i].Kind == graph.EdgeImports {
			importEdge = &edges[i]
			break
		}
	}
	if importEdge == nil {
		t.Fatalf("no import edge extracted from %s (got %d edges)", importer, len(edges))
	}

	resolved := graph.ResolveImportPath(
		importEdge.TargetNode, importEdge.SourceFile, root, alias, exists, logger,
	)

	// The stored source_node form of the imported file is its workspace-relative
	// path — the same string the extractor would assign when indexing it.
	const importedSourceNode = "utils/enums.ts"

	if resolved != importedSourceNode {
		t.Fatalf("resolved target_node %q != imported file source_node %q (canonical form broken)",
			resolved, importedSourceNode)
	}
	// And the canonicalizer must agree: an already-relative file node passes
	// through unchanged, so a reverse lookup on it byte-matches the stored edge.
	if got := canonicalizeRelative(root, importedSourceNode); got != importedSourceNode {
		t.Fatalf("canonicalizer changed relative node %q -> %q", importedSourceNode, got)
	}
}

// canonicalizeRelative mirrors mcp.resolveNodeAgainstWorkspace's no-DB rules:
// already-relative file nodes pass through unchanged (the canonical stored form).
func canonicalizeRelative(workspaceRoot, node string) string {
	if filepath.Ext(node) == "" {
		return node
	}
	if filepath.IsAbs(node) {
		// strip prefix
		prefix := workspaceRoot
		if prefix != "" && prefix[len(prefix)-1] != '/' {
			prefix += "/"
		}
		if len(node) > len(prefix) && node[:len(prefix)] == prefix {
			return node[len(prefix):]
		}
		return node
	}
	return node
}

func mustAbs(t *testing.T, p string) string {
	t.Helper()
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}
