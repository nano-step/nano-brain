package flow

import (
	"strings"
	"testing"
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
