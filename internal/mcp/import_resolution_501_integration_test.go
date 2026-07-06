//go:build integration

package mcp_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

// --- Issue #501: JS/TS/Vue import specifiers stored unresolved, so reverse
// impact (memory_impact direction:"in", edge_type:"imports") returns 0 for
// real importers of a file reached only via an alias or relative specifier.
//
// These tests run the REAL TypeScriptGraphExtractor against the committed
// testdata/import-fixture (internal/graph/testdata/import-fixture), through
// the same ImportContext machinery internal/watcher wires at index time
// (graph.BuildAliasIndex + graph.DiskExistsChecker), then persist the
// resulting edges into nanobrain_test exactly as internal/watcher would,
// and assert memory_impact finds the real importer.

// importFixtureRoot resolves the shared graph-package testdata fixture used
// by both internal/graph's unit tests and this MCP integration test.
func importFixtureRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("../graph/testdata/import-fixture")
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	if _, err := os.Stat(abs); err != nil {
		t.Fatalf("import fixture not found at %s: %v", abs, err)
	}
	return abs
}

// extractFixtureImportEdges runs the real extractor + resolver against one
// fixture file (relFile, workspace-relative to root) and returns only its
// "imports" edges.
func extractFixtureImportEdges(t *testing.T, root, relFile string) []graph.Edge {
	t.Helper()
	idx, err := graph.BuildAliasIndex(root)
	if err != nil {
		t.Fatalf("BuildAliasIndex: %v", err)
	}
	ex, err := graph.NewTypeScriptGraphExtractor()
	if err != nil {
		t.Fatalf("NewTypeScriptGraphExtractor: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(root, relFile))
	if err != nil {
		t.Fatalf("read fixture %s: %v", relFile, err)
	}
	ic := graph.ImportContext{
		AliasMap: idx.AliasMapFor(relFile),
		Exists:   graph.DiskExistsChecker(root),
	}
	edges, err := ex.ExtractEdgesWithImportContext(relFile, content, ic)
	if err != nil {
		t.Fatalf("extract %s: %v", relFile, err)
	}
	var imports []graph.Edge
	for _, e := range edges {
		if e.Kind == graph.EdgeImports {
			imports = append(imports, e)
		}
	}
	return imports
}

// AC-B1 + AC-B2 + AC-B3: index repo-a/consumer.ts (alias import, relative
// import, bare package import) and confirm memory_impact direction:"in"
// resolves the alias/relative importers but leaves the bare package alone.
func TestMemoryImpact_ImportEdge_AliasAndRelativeResolved(t *testing.T) {
	ctx, q, wsHash, callTool := setupFindingsMCP(t)
	root := importFixtureRoot(t)

	edges := extractFixtureImportEdges(t, root, "repo-a/consumer.ts")
	if len(edges) == 0 {
		t.Fatalf("no imports edges extracted for repo-a/consumer.ts")
	}
	for _, e := range edges {
		meta, err := json.Marshal(e.Metadata)
		if err != nil {
			t.Fatalf("marshal metadata: %v", err)
		}
		if err := q.UpsertGraphEdge(ctx, sqlc.UpsertGraphEdgeParams{
			WorkspaceHash: wsHash,
			SourceNode:    e.SourceNode,
			TargetNode:    e.TargetNode,
			EdgeType:      string(e.Kind),
			SourceFile:    e.SourceFile,
			Metadata:      meta,
		}); err != nil {
			t.Fatalf("upsert edge %s->%s: %v", e.SourceNode, e.TargetNode, err)
		}
	}

	// AC-B1: alias-resolved import (~/utils/enums -> repo-a/utils/enums.ts).
	aliasResult := callTool("memory_impact", map[string]any{
		"workspace": wsHash,
		"node":      "repo-a/utils/enums.ts",
		"direction": "in",
		"edge_type": "imports",
	})
	aliasResp := unmarshalGraphResp(t, aliasResult)
	aliasImpacted, _ := aliasResp["impacted"].([]any)
	if len(aliasImpacted) != 1 {
		t.Fatalf("AC-B1: alias import impacted count = %d, want 1: %+v", len(aliasImpacted), aliasImpacted)
	}
	if got := aliasImpacted[0].(map[string]any)["node"].(string); got != "repo-a/consumer.ts" {
		t.Errorf("AC-B1: alias import impacted[0].node = %q, want repo-a/consumer.ts", got)
	}

	// AC-B2: relative-resolved import (./sibling -> repo-a/sibling.ts).
	relResult := callTool("memory_impact", map[string]any{
		"workspace": wsHash,
		"node":      "repo-a/sibling.ts",
		"direction": "in",
		"edge_type": "imports",
	})
	relResp := unmarshalGraphResp(t, relResult)
	relImpacted, _ := relResp["impacted"].([]any)
	if len(relImpacted) != 1 {
		t.Fatalf("AC-B2: relative import impacted count = %d, want 1: %+v", len(relImpacted), relImpacted)
	}
	if got := relImpacted[0].(map[string]any)["node"].(string); got != "repo-a/consumer.ts" {
		t.Errorf("AC-B2: relative import impacted[0].node = %q, want repo-a/consumer.ts", got)
	}

	// AC-B3: bare package specifier ("vue") is untouched — it is still
	// queryable under its own raw name (unchanged behavior), never under a
	// resolved workspace path (there is none to resolve to).
	bareResult := callTool("memory_impact", map[string]any{
		"workspace": wsHash,
		"node":      "vue",
		"direction": "in",
		"edge_type": "imports",
	})
	bareResp := unmarshalGraphResp(t, bareResult)
	bareImpacted, _ := bareResp["impacted"].([]any)
	if len(bareImpacted) != 1 {
		t.Fatalf("AC-B3: bare package impacted count = %d, want 1: %+v", len(bareImpacted), bareImpacted)
	}
	if got := bareImpacted[0].(map[string]any)["node"].(string); got != "repo-a/consumer.ts" {
		t.Errorf("AC-B3: bare package impacted[0].node = %q, want repo-a/consumer.ts", got)
	}
}

// G3 regression guard at the MCP layer: repo-a and repo-b each define their
// own "~/" alias root under ONE shared fixture tree. If alias resolution
// ever regresses to a single global map, both would resolve to the same
// target and memory_impact would return BOTH consumers for one node — a
// fabricated cross-repo edge. This test asserts exact, non-overlapping
// importer sets.
func TestMemoryImpact_ImportEdge_NoCrossRepoCollision(t *testing.T) {
	ctx, q, wsHash, callTool := setupFindingsMCP(t)
	root := importFixtureRoot(t)

	for _, relFile := range []string{"repo-a/consumer.ts", "repo-b/consumer.ts"} {
		for _, e := range extractFixtureImportEdges(t, root, relFile) {
			meta, err := json.Marshal(e.Metadata)
			if err != nil {
				t.Fatalf("marshal metadata: %v", err)
			}
			if err := q.UpsertGraphEdge(ctx, sqlc.UpsertGraphEdgeParams{
				WorkspaceHash: wsHash,
				SourceNode:    e.SourceNode,
				TargetNode:    e.TargetNode,
				EdgeType:      string(e.Kind),
				SourceFile:    e.SourceFile,
				Metadata:      meta,
			}); err != nil {
				t.Fatalf("upsert edge %s->%s: %v", e.SourceNode, e.TargetNode, err)
			}
		}
	}

	aResult := callTool("memory_impact", map[string]any{
		"workspace": wsHash,
		"node":      "repo-a/utils/enums.ts",
		"direction": "in",
		"edge_type": "imports",
	})
	aResp := unmarshalGraphResp(t, aResult)
	aImpacted, _ := aResp["impacted"].([]any)
	if len(aImpacted) != 1 || aImpacted[0].(map[string]any)["node"].(string) != "repo-a/consumer.ts" {
		t.Fatalf("repo-a/utils/enums.ts impacted = %+v, want exactly [repo-a/consumer.ts]", aImpacted)
	}

	bResult := callTool("memory_impact", map[string]any{
		"workspace": wsHash,
		"node":      "repo-b/src/utils/enums.ts",
		"direction": "in",
		"edge_type": "imports",
	})
	bResp := unmarshalGraphResp(t, bResult)
	bImpacted, _ := bResp["impacted"].([]any)
	if len(bImpacted) != 1 || bImpacted[0].(map[string]any)["node"].(string) != "repo-b/consumer.ts" {
		t.Fatalf("repo-b/src/utils/enums.ts impacted = %+v, want exactly [repo-b/consumer.ts]", bImpacted)
	}
}
