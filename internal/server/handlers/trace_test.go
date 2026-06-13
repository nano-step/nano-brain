package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

// mockTraceQuerier implements handlers.TraceQuerier for unit tests.
type mockTraceQuerier struct {
	// outgoing maps exact source_node -> edges returned by GetOutgoingEdges
	outgoing map[string][]sqlc.GraphEdge
	// bySymbol maps symbol (bare or exact) -> edges returned by GetOutgoingEdgesBySymbol
	bySymbol map[string][]sqlc.GraphEdge
}

func (m *mockTraceQuerier) GetOutgoingEdges(_ context.Context, arg sqlc.GetOutgoingEdgesParams) ([]sqlc.GraphEdge, error) {
	return m.outgoing[arg.SourceNode], nil
}

func (m *mockTraceQuerier) GetOutgoingEdgesBySymbol(_ context.Context, arg sqlc.GetOutgoingEdgesBySymbolParams) ([]sqlc.GraphEdge, error) {
	return m.bySymbol[arg.SourceNode], nil
}

func edge(src, tgt string) sqlc.GraphEdge {
	return sqlc.GraphEdge{SourceNode: src, TargetNode: tgt, EdgeType: "calls"}
}

// TestTraceCallChain_SingleHop verifies that a qualified entry node ("file::Func")
// uses GetOutgoingEdges and returns one hop.
func TestTraceCallChain_SingleHop(t *testing.T) {
	q := &mockTraceQuerier{
		outgoing: map[string][]sqlc.GraphEdge{
			"search.go::SearchAll": {edge("search.go::SearchAll", "BM25SearchAll")},
		},
	}

	chain := runTrace(t, q, "search.go::SearchAll", 5)

	if len(chain) != 1 {
		t.Fatalf("expected 1 step, got %d: %v", len(chain), chain)
	}
	if chain[0].Node != "BM25SearchAll" {
		t.Errorf("expected BM25SearchAll, got %q", chain[0].Node)
	}
	if chain[0].Depth != 1 {
		t.Errorf("expected depth 1, got %d", chain[0].Depth)
	}
}

// TestTraceCallChain_MultiHop verifies that bare callee names are reconciled via
// GetOutgoingEdgesBySymbol enabling traversal beyond depth 1.
//
// Chain: "pkg.go::EntryFunc" --calls--> "Mid" --calls--> "Leaf"
// After hop 1, cur.node = "Mid" (bare) → GetOutgoingEdgesBySymbol("Mid") → "Leaf"
func TestTraceCallChain_MultiHop(t *testing.T) {
	q := &mockTraceQuerier{
		outgoing: map[string][]sqlc.GraphEdge{
			// Qualified entry — found via exact lookup
			"pkg.go::EntryFunc": {edge("pkg.go::EntryFunc", "Mid")},
		},
		bySymbol: map[string][]sqlc.GraphEdge{
			// Bare "Mid" — reconciled to its defining file::symbol form
			"Mid": {edge("mid.go::Mid", "Leaf")},
		},
	}

	chain := runTrace(t, q, "pkg.go::EntryFunc", 5)

	if len(chain) != 2 {
		t.Fatalf("expected 2 steps (Mid + Leaf), got %d: %v", len(chain), chain)
	}

	nodes := map[string]int{}
	for _, s := range chain {
		nodes[s.Node] = s.Depth
	}

	if nodes["Mid"] != 1 {
		t.Errorf("expected Mid at depth 1, got depth %d", nodes["Mid"])
	}
	if nodes["Leaf"] != 2 {
		t.Errorf("expected Leaf at depth 2, got depth %d", nodes["Leaf"])
	}
}

// TestTraceCallChain_CycleBreaking verifies cycles do not produce infinite loops.
func TestTraceCallChain_CycleBreaking(t *testing.T) {
	q := &mockTraceQuerier{
		outgoing: map[string][]sqlc.GraphEdge{
			"a.go::A": {edge("a.go::A", "b.go::B")},
			"b.go::B": {edge("b.go::B", "a.go::A")}, // cycle back
		},
	}

	chain := runTrace(t, q, "a.go::A", 5)

	// Only b.go::B should appear; a.go::A is the entry (seen) so skipped
	if len(chain) != 1 {
		t.Fatalf("expected 1 step (cycle broken), got %d: %v", len(chain), chain)
	}
	if chain[0].Node != "b.go::B" {
		t.Errorf("expected b.go::B, got %q", chain[0].Node)
	}
}

// TestTraceCallChain_MaxDepth verifies traversal stops at maxDepth.
func TestTraceCallChain_MaxDepth(t *testing.T) {
	// Chain: A -> B -> C -> D (depth 3 would be D), but maxDepth=2
	q := &mockTraceQuerier{
		outgoing: map[string][]sqlc.GraphEdge{
			"f.go::A": {edge("f.go::A", "f.go::B")},
			"f.go::B": {edge("f.go::B", "f.go::C")},
			"f.go::C": {edge("f.go::C", "f.go::D")},
		},
	}

	chain := runTrace(t, q, "f.go::A", 2)

	// depth 1 = B, depth 2 = C; D is at depth 3 and should be excluded
	if len(chain) != 2 {
		t.Fatalf("expected 2 steps (depth capped at 2), got %d: %v", len(chain), chain)
	}
}

// TestTraceHandler_BadRequest verifies HTTP 400 on missing node field.
func TestTraceHandler_BadRequest(t *testing.T) {
	q := &mockTraceQuerier{}
	logger := zerolog.Nop()
	h := handlers.GraphTrace(q, logger)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/trace", strings.NewReader(`{}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws1")

	err := h(c)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected *echo.HTTPError, got %T", err)
	}
	if he.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", he.Code)
	}
}

// runTrace is a helper that calls GraphTrace via HTTP and decodes the chain.
func runTrace(t *testing.T, q handlers.TraceQuerier, node string, maxDepth int) []traceStep {
	t.Helper()

	logger := zerolog.Nop()
	h := handlers.GraphTrace(q, logger)

	e := echo.New()
	body, _ := json.Marshal(map[string]interface{}{"node": node, "max_depth": maxDepth})
	req := httptest.NewRequest(http.MethodPost, "/trace", strings.NewReader(string(body)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws1")

	if err := h(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}

	var resp struct {
		Entry string      `json:"entry"`
		Chain []traceStep `json:"chain"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	return resp.Chain
}

// traceStep mirrors handlers.traceStep for decoding test responses.
type traceStep struct {
	Node  string `json:"node"`
	Depth int    `json:"depth"`
	Via   string `json:"via"`
}
