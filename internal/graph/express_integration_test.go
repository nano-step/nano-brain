//go:build integration

package graph_test

import (
	"os"
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/rs/zerolog"
)

func TestExpressExtractor_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	content, err := os.ReadFile("../../test/fixtures/express/app.ts")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	logger := zerolog.Nop()
	ex, err := graph.NewExpressExtractor(logger)
	if err != nil {
		t.Fatalf("NewExpressExtractor: %v", err)
	}

	edges, err := ex.ExtractEdges("test/fixtures/express/app.ts", content)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Extracted %d edges", len(edges))
	for _, edge := range edges {
		t.Logf("  %s -> %s [%s]", edge.SourceNode, edge.TargetNode, edge.Kind)
	}

	httpEdges := 0
	mwEdges := 0
	for _, edge := range edges {
		switch edge.Kind {
		case graph.EdgeHTTP:
			httpEdges++
		case graph.EdgeMiddleware:
			mwEdges++
		}
	}

	if httpEdges < 5 {
		t.Errorf("expected at least 5 HTTP edges, got %d", httpEdges)
	}
	if mwEdges < 2 {
		t.Errorf("expected at least 2 middleware edges, got %d", mwEdges)
	}

	cases := []struct{ verb, path, handler string }{
		{"GET", "/users", "userController.list"},
		{"POST", "/users", "userController.create"},
		{"GET", "/users/:id", "userController.getById"},
		{"GET", "/posts", "postController.list"},
		{"POST", "/posts", "postController.create"},
		{"GET", "/health", "healthController.check"},
	}
	for _, tc := range cases {
		entryNode := tc.verb + " " + tc.path
		hits := findEdges(edges, graph.EdgeHTTP, entryNode, tc.handler)
		if len(hits) == 0 {
			t.Errorf("missing http edge %s → %s", entryNode, tc.handler)
		}
	}
}
