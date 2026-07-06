//go:build integration

package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

// --- Issue #553 (REST gap): POST /api/v1/graph/impact misses calls-edge callers ---
//
// collectImpact (internal/server/handlers/impact.go) had the identical bug the
// memory_impact MCP tool had: calls-edge targets are stored bare (e.g. "checkAccess"),
// not "file::symbol", so a qualified-only frontier never matched a bare-stored calls
// target via GetImpactorsByTargets's target_node = ANY($2) predicate. Fixed by routing
// both surfaces through the shared symbol.ExpandImpactFrontier helper.

func callGraphImpact(t *testing.T, q handlers.ImpactQuerier, wsHash, node, edgeType string, maxDepth int) map[string]interface{} {
	t.Helper()
	e := echo.New()
	h := handlers.GraphImpact(q, nopLogger())

	body, err := json.Marshal(map[string]interface{}{
		"node":      node,
		"edge_type": edgeType,
		"max_depth": maxDepth,
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/graph/impact", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", wsHash)

	if err := h(c); err != nil {
		t.Fatalf("GraphImpact handler: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode impact response: %v", err)
	}
	return resp
}

// AC-A1 + single-repo-exactness: a direct caller of a bare-target calls edge is
// returned, and exactly once (no cross-repo/duplicate false positives within this
// workspace-scoped fixture).
func TestGraphImpact_BareCallsTarget_DirectCallerReturned(t *testing.T) {
	q := newQueries(t)
	ctx := context.Background()
	wsHash := "ws_" + uuid.New().String()

	if err := q.UpsertGraphEdge(ctx, sqlc.UpsertGraphEdgeParams{
		WorkspaceHash: wsHash,
		SourceNode:    "caller.go::doThing",
		TargetNode:    "B", // bare, as calls-edge extractors write it
		EdgeType:      "calls",
		SourceFile:    "caller.go",
		Metadata:      []byte("{}"),
	}); err != nil {
		t.Fatalf("upsert edge: %v", err)
	}

	resp := callGraphImpact(t, q, wsHash, "b.go::B", "", 1)
	impacted, _ := resp["impacted"].([]interface{})
	if len(impacted) < 1 {
		t.Fatalf("impacted count = %d, want >=1 (caller caller.go::doThing): %+v", len(impacted), impacted)
	}

	found := 0
	for _, im := range impacted {
		if im.(map[string]interface{})["node"].(string) == "caller.go::doThing" {
			found++
		}
	}
	if found != 1 {
		t.Errorf("caller.go::doThing found %d times in this workspace, want exactly 1 (single-repo exactness)", found)
	}
	if len(impacted) != 1 {
		t.Errorf("impacted count = %d, want exactly 1 in this isolated workspace fixture: %+v", len(impacted), impacted)
	}
}

// AC-A2: transitive callers past depth 1 are also resolved — the bare-suffix
// seeding must apply to every "next" frontier batch built during the depth loop,
// not just the initial seed.
func TestGraphImpact_BareCallsTarget_TransitiveCallersAtDepth(t *testing.T) {
	q := newQueries(t)
	ctx := context.Background()
	wsHash := "ws_" + uuid.New().String()

	// A calls B, B calls C — both calls-edge targets stored bare.
	edges := []struct{ source, target string }{
		{"a.go::A", "B"},
		{"b.go::B", "C"},
	}
	for _, e := range edges {
		if err := q.UpsertGraphEdge(ctx, sqlc.UpsertGraphEdgeParams{
			WorkspaceHash: wsHash,
			SourceNode:    e.source,
			TargetNode:    e.target,
			EdgeType:      "calls",
			SourceFile:    "",
			Metadata:      []byte("{}"),
		}); err != nil {
			t.Fatalf("upsert edge %+v: %v", e, err)
		}
	}

	// depth=1 only resolves the direct caller of C (B).
	depth1Resp := callGraphImpact(t, q, wsHash, "c.go::C", "", 1)
	depth1Impacted, _ := depth1Resp["impacted"].([]interface{})
	if len(depth1Impacted) != 1 {
		t.Fatalf("depth=1 impacted count = %d, want 1 (B only): %+v", len(depth1Impacted), depth1Impacted)
	}
	if depth1Impacted[0].(map[string]interface{})["node"].(string) != "b.go::B" {
		t.Errorf("depth=1 impacted[0].node = %v, want b.go::B", depth1Impacted[0].(map[string]interface{})["node"])
	}

	// depth=2 (and depth=3) must additionally resolve the transitive caller A.
	for _, depth := range []int{2, 3} {
		resp := callGraphImpact(t, q, wsHash, "c.go::C", "", depth)
		impacted, _ := resp["impacted"].([]interface{})
		if len(impacted) != 2 {
			t.Fatalf("max_depth=%d impacted count = %d, want 2 (B at depth1, A at depth2): %+v", depth, len(impacted), impacted)
		}
		seen := map[string]int{}
		for _, im := range impacted {
			m := im.(map[string]interface{})
			seen[m["node"].(string)] = int(m["depth"].(float64))
		}
		if seen["b.go::B"] != 1 {
			t.Errorf("max_depth=%d: b.go::B depth = %d, want 1", depth, seen["b.go::B"])
		}
		if seen["a.go::A"] != 2 {
			t.Errorf("max_depth=%d: a.go::A depth = %d, want 2 (transitive caller must be found)", depth, seen["a.go::A"])
		}
	}
}

// AC-A3 no-regression: an already-qualified imports-edge target with no "::" suffix
// keeps working exactly as before — ExpandImpactFrontier is a no-op on it, so no
// duplicate or spurious matches are introduced by the bare-suffix expansion.
func TestGraphImpact_NoRegression_QualifiedImportsTarget(t *testing.T) {
	q := newQueries(t)
	ctx := context.Background()
	wsHash := "ws_" + uuid.New().String()

	if err := q.UpsertGraphEdge(ctx, sqlc.UpsertGraphEdgeParams{
		WorkspaceHash: wsHash,
		SourceNode:    "consumer.ts",
		TargetNode:    "lib.ts", // already-resolved import target, no "::" suffix
		EdgeType:      "imports",
		SourceFile:    "consumer.ts",
		Metadata:      []byte("{}"),
	}); err != nil {
		t.Fatalf("upsert imports edge: %v", err)
	}

	resp := callGraphImpact(t, q, wsHash, "lib.ts", "imports", 1)
	impacted, _ := resp["impacted"].([]interface{})
	if len(impacted) != 1 {
		t.Fatalf("impacted count = %d, want exactly 1: %+v", len(impacted), impacted)
	}
	if impacted[0].(map[string]interface{})["node"].(string) != "consumer.ts" {
		t.Errorf("impacted[0].node = %v, want consumer.ts", impacted[0].(map[string]interface{})["node"])
	}
}
