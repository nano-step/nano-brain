package flow

import (
	"strings"
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
)

func TestRenderSequenceDiagramHeader(t *testing.T) {
	d := RenderSequenceDiagram(Flow{})
	if !strings.HasPrefix(d, "sequenceDiagram\n") {
		t.Errorf("expected sequenceDiagram header, got %q", d)
	}
}

func TestRenderSequenceDiagramFewParticipants(t *testing.T) {
	f := Flow{
		Entry:  "POST /purchase",
		Method: "POST",
		Path:   "/purchase",
		Nodes: []FlowNode{
			{ID: "POST /purchase", Name: "POST /purchase", Role: RoleEntry},
			{ID: "purchase", Name: "purchase", Role: RoleHandler},
			{ID: "getConnection", Name: "getConnection", Role: RoleFunc},
			{ID: "GET mysql", Name: "GET mysqlConnection", Role: RoleIntegration},
			{ID: "getUser", Name: "getUser", Role: RoleFunc},
			{ID: "GET steam", Name: "GET steamLevel", Role: RoleIntegration},
			{ID: "addBalance", Name: "addBalance", Role: RoleFunc},
			{ID: "POST add-balance", Name: "POST add-balance", Role: RoleIntegration},
		},
		Edges: []FlowEdge{
			{From: "POST /purchase", To: "purchase", Kind: "http"},
			{From: "purchase", To: "getConnection", Kind: "calls"},
			{From: "getConnection", To: "GET mysql", Kind: "integration"},
			{From: "purchase", To: "getUser", Kind: "calls"},
			{From: "getUser", To: "GET steam", Kind: "integration"},
			{From: "purchase", To: "addBalance", Kind: "calls"},
			{From: "addBalance", To: "POST add-balance", Kind: "integration"},
		},
	}
	diagram := RenderSequenceDiagram(f)
	count := strings.Count(diagram, "participant ")
	if count > 6 {
		t.Errorf("expected ≤6 participants, got %d", count)
	}
	if !strings.Contains(diagram, "participant Client") {
		t.Error("expected Client participant")
	}
	if !strings.Contains(diagram, "Backend") {
		t.Error("expected Backend participant")
	}
}

func TestRenderSequenceDiagramReturnArrows(t *testing.T) {
	f := Flow{
		Entry:  "GET /data",
		Method: "GET",
		Path:   "/data",
		Nodes: []FlowNode{
			{ID: "GET /data", Name: "GET /data", Role: RoleEntry},
			{ID: "handler", Name: "handler", Role: RoleHandler},
			{ID: "GET api", Name: "GET api.example.com/data", Role: RoleIntegration},
		},
		Edges: []FlowEdge{
			{From: "GET /data", To: "handler", Kind: "http"},
			{From: "handler", To: "GET api", Kind: "integration"},
		},
	}
	diagram := RenderSequenceDiagram(f)
	if !strings.Contains(diagram, "-->>") {
		t.Error("expected return arrows (-->>)")
	}
}

func TestRenderSequenceDiagramMiddlewareAsNote(t *testing.T) {
	f := Flow{
		Entry: "GET /protected",
		Nodes: []FlowNode{
			{ID: "GET /protected", Name: "GET /protected", Role: RoleEntry},
			{ID: "authMW", Name: "AuthMiddleware", Role: RoleMiddleware},
			{ID: "handler", Name: "handler", Role: RoleHandler},
			{ID: "GET api", Name: "GET api.example.com", Role: RoleIntegration},
		},
		Edges: []FlowEdge{
			{From: "GET /protected", To: "authMW", Kind: "middleware"},
			{From: "authMW", To: "handler", Kind: "middleware"},
			{From: "handler", To: "GET api", Kind: "integration"},
		},
	}
	diagram := RenderSequenceDiagram(f)
	if strings.Contains(diagram, "participant authMW") {
		t.Error("middleware should NOT be a separate participant")
	}
	if !strings.Contains(diagram, "guarded by") {
		t.Error("middleware should appear as Note with 'guarded by'")
	}
}

func TestRenderSequenceDiagramNoRawCallsLabel(t *testing.T) {
	f := Flow{
		Entry: "POST /action",
		Nodes: []FlowNode{
			{ID: "POST /action", Name: "POST /action", Role: RoleEntry},
			{ID: "handler", Name: "handler", Role: RoleHandler},
			{ID: "GET ext", Name: "GET external-api.com", Role: RoleIntegration},
		},
		Edges: []FlowEdge{
			{From: "POST /action", To: "handler", Kind: "http"},
			{From: "handler", To: "GET ext", Kind: "integration"},
		},
	}
	diagram := RenderSequenceDiagram(f)
	if strings.Contains(diagram, "->>Backend: calls") || strings.Contains(diagram, "-->>Backend: calls") {
		t.Error("should not use raw 'calls' as message label")
	}
}

func TestRenderSequenceDiagramDeterministic(t *testing.T) {
	f := Flow{
		Entry: "GET /x",
		Nodes: []FlowNode{
			{ID: "GET /x", Name: "GET /x", Role: RoleEntry},
			{ID: "h", Name: "h", Role: RoleHandler},
			{ID: "GET a", Name: "GET a.com", Role: RoleIntegration},
			{ID: "GET b", Name: "GET b.com", Role: RoleIntegration},
		},
		Edges: []FlowEdge{
			{From: "GET /x", To: "h", Kind: "http"},
			{From: "h", To: "GET a", Kind: "integration", Line: 10},
			{From: "h", To: "GET b", Kind: "integration", Line: 5},
		},
	}
	d1 := RenderSequenceDiagram(f)
	d2 := RenderSequenceDiagram(f)
	if d1 != d2 {
		t.Error("same input should produce same output")
	}
}

func TestRenderInternalLogic_SimpleSteps(t *testing.T) {
	cfg := &graph.CFG{
		Nodes: []graph.CFGNode{
			{ID: "s1", Type: "start", Label: "purchaseHandler"},
			{ID: "s2", Type: "step", Label: "validateInput"},
			{ID: "s3", Type: "step", Label: "processPayment"},
			{ID: "s4", Type: "terminal", Label: "return result"},
		},
		Edges: []graph.CFGEdge{
			{From: "s1", To: "s2", Branch: "next"},
			{From: "s2", To: "s3", Branch: "next"},
			{From: "s3", To: "s4", Branch: "next"},
		},
	}

	nodes := cfgNodeMap(cfg)
	adj := cfgAdj(cfg)
	msgCount := 0
	lines := renderInternalLogic("s1", nodes, adj, "Backend", 0, &msgCount, make(map[string]bool))

	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d: %v", len(lines), lines)
	}
	if !strings.Contains(lines[0], "purchaseHandler") {
		t.Errorf("expected start label, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "validateInput") {
		t.Errorf("expected step label, got %q", lines[1])
	}
	if !strings.Contains(lines[2], "processPayment") {
		t.Errorf("expected step label, got %q", lines[2])
	}
	if !strings.Contains(lines[3], "return result") {
		t.Errorf("expected terminal label, got %q", lines[3])
	}
	if msgCount != 4 {
		t.Errorf("expected msgCount=4, got %d", msgCount)
	}
}

func TestRenderInternalLogic_AltBlock(t *testing.T) {
	cfg := &graph.CFG{
		Nodes: []graph.CFGNode{
			{ID: "s1", Type: "start", Label: "handler"},
			{ID: "d1", Type: "decision", Label: "if (err)"},
			{ID: "s2", Type: "step", Label: "handleError"},
			{ID: "s3", Type: "step", Label: "processData"},
			{ID: "s4", Type: "terminal", Label: "return"},
		},
		Edges: []graph.CFGEdge{
			{From: "s1", To: "d1", Branch: "next"},
			{From: "d1", To: "s2", Branch: "yes"},
			{From: "d1", To: "s3", Branch: "no"},
			{From: "s2", To: "s4", Branch: "next"},
			{From: "s3", To: "s4", Branch: "next"},
		},
	}

	nodes := cfgNodeMap(cfg)
	adj := cfgAdj(cfg)
	msgCount := 0
	lines := renderInternalLogic("s1", nodes, adj, "Backend", 0, &msgCount, make(map[string]bool))

	diagram := strings.Join(lines, "\n")
	if !strings.Contains(diagram, "alt") {
		t.Error("expected alt block")
	}
	if !strings.Contains(diagram, "condition") {
		t.Error("expected 'condition' label for yes branch")
	}
	if !strings.Contains(diagram, "else") {
		t.Error("expected 'else' label for no branch")
	}
	if !strings.Contains(diagram, "handleError") {
		t.Error("expected handleError in yes branch")
	}
	if !strings.Contains(diagram, "processData") {
		t.Error("expected processData in no branch")
	}
	if !strings.Contains(diagram, "end") {
		t.Error("expected end block")
	}
}

func TestRenderInternalLogic_LoopBlock(t *testing.T) {
	cfg := &graph.CFG{
		Nodes: []graph.CFGNode{
			{ID: "s1", Type: "start", Label: "handler"},
			{ID: "d1", Type: "decision", Label: "while (items.hasNext)"},
			{ID: "s2", Type: "step", Label: "processItem"},
			{ID: "s3", Type: "terminal", Label: "return"},
		},
		Edges: []graph.CFGEdge{
			{From: "s1", To: "d1", Branch: "next"},
			{From: "d1", To: "s2", Branch: "loop"},
			{From: "s2", To: "d1", Branch: "next"},
			{From: "d1", To: "s3", Branch: "next"},
		},
	}

	nodes := cfgNodeMap(cfg)
	adj := cfgAdj(cfg)
	msgCount := 0
	lines := renderInternalLogic("s1", nodes, adj, "Backend", 0, &msgCount, make(map[string]bool))

	diagram := strings.Join(lines, "\n")
	if !strings.Contains(diagram, "loop") {
		t.Error("expected loop block")
	}
	if !strings.Contains(diagram, "processItem") {
		t.Error("expected processItem inside loop")
	}
}

func TestRenderInternalLogic_DepthTruncation(t *testing.T) {
	cfg := &graph.CFG{
		Nodes: []graph.CFGNode{
			{ID: "s1", Type: "start", Label: "handler"},
			{ID: "d1", Type: "decision", Label: "if a"},
			{ID: "d2", Type: "decision", Label: "if b"},
			{ID: "d3", Type: "decision", Label: "if c"},
			{ID: "d4", Type: "decision", Label: "if d"},
			{ID: "s2", Type: "step", Label: "deep"},
			{ID: "s3", Type: "terminal", Label: "return"},
		},
		Edges: []graph.CFGEdge{
			{From: "s1", To: "d1", Branch: "next"},
			{From: "d1", To: "d2", Branch: "yes"},
			{From: "d1", To: "s3", Branch: "no"},
			{From: "d2", To: "d3", Branch: "yes"},
			{From: "d2", To: "s3", Branch: "no"},
			{From: "d3", To: "d4", Branch: "yes"},
			{From: "d3", To: "s3", Branch: "no"},
			{From: "d4", To: "s2", Branch: "yes"},
			{From: "d4", To: "s3", Branch: "no"},
		},
	}

	nodes := cfgNodeMap(cfg)
	adj := cfgAdj(cfg)
	msgCount := 0
	lines := renderInternalLogic("s1", nodes, adj, "Backend", 0, &msgCount, make(map[string]bool))

	diagram := strings.Join(lines, "\n")
	if strings.Contains(diagram, "deep") {
		t.Error("expected depth truncation — 'deep' should not appear at depth 4")
	}
}

func TestRenderInternalLogic_MessageLimit(t *testing.T) {
	var nodes []graph.CFGNode
	var edges []graph.CFGEdge
	nodes = append(nodes, graph.CFGNode{ID: "s0", Type: "start", Label: "entry"})
	for i := 1; i <= 60; i++ {
		id := "s" + string(rune('a'+i%26))
		nodes = append(nodes, graph.CFGNode{ID: id, Type: "step", Label: "step"})
		edges = append(edges, graph.CFGEdge{From: "s" + string(rune('a'+(i-1)%26)), To: id, Branch: "next"})
	}

	cfg := &graph.CFG{Nodes: nodes, Edges: edges}
	nodeMap := cfgNodeMap(cfg)
	adj := cfgAdj(cfg)
	msgCount := 0
	lines := renderInternalLogic("s0", nodeMap, adj, "Backend", 0, &msgCount, make(map[string]bool))

	if msgCount > maxSeqMessages {
		t.Errorf("expected msgCount <= %d, got %d", maxSeqMessages, msgCount)
	}
	if len(lines) > maxSeqMessages+1 {
		t.Errorf("expected <= %d lines, got %d", maxSeqMessages+1, len(lines))
	}
}

func TestRenderInternalLogic_MissingCFG(t *testing.T) {
	diagram := RenderSequenceDiagram(Flow{})
	if !strings.HasPrefix(diagram, "sequenceDiagram\n") {
		t.Error("expected sequenceDiagram header")
	}
	if strings.Contains(diagram, "alt") {
		t.Error("no CFG should mean no alt blocks")
	}
}

func TestRenderSequenceDiagramWithCFG_CrossActor(t *testing.T) {
	f := Flow{
		Entry:       "POST /purchase",
		Method:      "POST",
		Path:        "/purchase",
		ServiceName: "Backend",
		Nodes: []FlowNode{
			{ID: "POST /purchase", Name: "POST /purchase", Role: RoleEntry},
			{ID: "purchase", Name: "purchase", Role: RoleHandler},
			{ID: "GET api", Name: "GET api.example.com", Role: RoleIntegration},
		},
		Edges: []FlowEdge{
			{From: "POST /purchase", To: "purchase", Kind: "http"},
			{From: "purchase", To: "GET api", Kind: "integration"},
		},
	}

	cfg := &graph.CFG{
		Nodes: []graph.CFGNode{
			{ID: "n1", Type: "start", Label: "purchase"},
			{ID: "n2", Type: "step", Label: "validate"},
			{ID: "n3", Type: "terminal", Label: "return"},
		},
		Edges: []graph.CFGEdge{
			{From: "n1", To: "n2", Branch: "next"},
			{From: "n2", To: "n3", Branch: "next"},
		},
	}
	cfgs := FlowCFGs{"purchase": cfg}

	diagram := RenderSequenceDiagramWithCFG(f, cfgs)
	if !strings.Contains(diagram, "->>Api Example: integration") {
		t.Errorf("expected cross-actor message to Api Example, got:\n%s", diagram)
	}
	if !strings.Contains(diagram, "Backend->>Backend: validate") {
		t.Errorf("expected internal self-message for validate step, got:\n%s", diagram)
	}
	if strings.Contains(diagram, "alt") {
		t.Error("expected no alt blocks for linear CFG")
	}
}

func TestRenderSequenceDiagramWithCFG_AltIntegration(t *testing.T) {
	f := Flow{
		Entry:       "POST /purchase",
		Method:      "POST",
		Path:        "/purchase",
		ServiceName: "Backend",
		Nodes: []FlowNode{
			{ID: "POST /purchase", Name: "POST /purchase", Role: RoleEntry},
			{ID: "purchase", Name: "purchase", Role: RoleHandler},
		},
		Edges: []FlowEdge{
			{From: "POST /purchase", To: "purchase", Kind: "http"},
		},
	}

	cfg := &graph.CFG{
		Nodes: []graph.CFGNode{
			{ID: "n1", Type: "start", Label: "purchase"},
			{ID: "d1", Type: "decision", Label: "if (err)"},
			{ID: "n2", Type: "step", Label: "handleError"},
			{ID: "n3", Type: "step", Label: "process"},
			{ID: "n4", Type: "terminal", Label: "return"},
		},
		Edges: []graph.CFGEdge{
			{From: "n1", To: "d1", Branch: "next"},
			{From: "d1", To: "n2", Branch: "yes"},
			{From: "d1", To: "n3", Branch: "no"},
			{From: "n2", To: "n4", Branch: "next"},
			{From: "n3", To: "n4", Branch: "next"},
		},
	}
	cfgs := FlowCFGs{"purchase": cfg}

	diagram := RenderSequenceDiagramWithCFG(f, cfgs)
	if !strings.Contains(diagram, "alt") {
		t.Error("expected alt block")
	}
	if !strings.Contains(diagram, "condition") {
		t.Error("expected 'condition' label")
	}
	if !strings.Contains(diagram, "else") {
		t.Error("expected 'else' label")
	}
	if !strings.Contains(diagram, "handleError") {
		t.Error("expected handleError in alt block")
	}
	if !strings.Contains(diagram, "process") {
		t.Error("expected process in alt block")
	}
}

func TestRenderSequenceDiagramWithCFG_DepthLimit(t *testing.T) {
	f := Flow{
		Entry:       "POST /test",
		Method:      "POST",
		Path:        "/test",
		ServiceName: "Backend",
		Nodes: []FlowNode{
			{ID: "POST /test", Name: "POST /test", Role: RoleEntry},
			{ID: "h", Name: "h", Role: RoleHandler},
		},
		Edges: []FlowEdge{
			{From: "POST /test", To: "h", Kind: "http"},
		},
	}

	cfg := &graph.CFG{
		Nodes: []graph.CFGNode{
			{ID: "s", Type: "start", Label: "h"},
			{ID: "d1", Type: "decision", Label: "if a"},
			{ID: "d2", Type: "decision", Label: "if b"},
			{ID: "d3", Type: "decision", Label: "if c"},
			{ID: "d4", Type: "decision", Label: "if d"},
			{ID: "deep", Type: "step", Label: "tooDeep"},
			{ID: "end", Type: "terminal", Label: "return"},
		},
		Edges: []graph.CFGEdge{
			{From: "s", To: "d1", Branch: "next"},
			{From: "d1", To: "d2", Branch: "yes"},
			{From: "d1", To: "end", Branch: "no"},
			{From: "d2", To: "d3", Branch: "yes"},
			{From: "d2", To: "end", Branch: "no"},
			{From: "d3", To: "d4", Branch: "yes"},
			{From: "d3", To: "end", Branch: "no"},
			{From: "d4", To: "deep", Branch: "yes"},
			{From: "d4", To: "end", Branch: "no"},
		},
	}
	cfgs := FlowCFGs{"h": cfg}

	diagram := RenderSequenceDiagramWithCFG(f, cfgs)
	if strings.Contains(diagram, "tooDeep") {
		t.Error("expected depth truncation — 'tooDeep' should not appear at depth 4")
	}
}

func TestRenderInternalLogic_ErrorTerminal(t *testing.T) {
	cfg := &graph.CFG{
		Nodes: []graph.CFGNode{
			{ID: "s1", Type: "start", Label: "handler"},
			{ID: "t1", Type: "terminal", Label: "NotFoundError", Kind: "error"},
		},
		Edges: []graph.CFGEdge{
			{From: "s1", To: "t1", Branch: "next"},
		},
	}

	nodes := cfgNodeMap(cfg)
	adj := cfgAdj(cfg)
	msgCount := 0
	lines := renderInternalLogic("s1", nodes, adj, "Backend", 0, &msgCount, make(map[string]bool))

	diagram := strings.Join(lines, "\n")
	if !strings.Contains(diagram, "throw NotFoundError") {
		t.Errorf("expected 'throw NotFoundError', got %q", diagram)
	}
}
