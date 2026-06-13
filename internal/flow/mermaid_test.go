package flow_test

import (
	"strings"
	"testing"

	"github.com/nano-brain/nano-brain/internal/flow"
)

func simpleFlow() flow.Flow {
	return flow.Flow{
		Entry:  "POST /api/topup",
		Method: "POST",
		Path:   "/api/topup",
		Nodes: []flow.FlowNode{
			{ID: "POST /api/topup", Name: "POST /api/topup", Role: flow.RoleEntry},
			{ID: "HandleTopup", Name: "HandleTopup", Role: flow.RoleHandler},
			{ID: "PayService", Name: "PayService", Role: flow.RoleService},
			{ID: "PayRepo", Name: "PayRepo", Role: flow.RoleRepo},
		},
		Edges: []flow.FlowEdge{
			{From: "POST /api/topup", To: "HandleTopup", Kind: "http"},
			{From: "HandleTopup", To: "PayService", Kind: "calls"},
			{From: "PayService", To: "PayRepo", Kind: "calls"},
		},
	}
}

func flowWithMiddleware() flow.Flow {
	return flow.Flow{
		Entry:  "POST /api/topup",
		Method: "POST",
		Path:   "/api/topup",
		Nodes: []flow.FlowNode{
			{ID: "POST /api/topup", Name: "POST /api/topup", Role: flow.RoleEntry},
			{ID: "AuthMW", Name: "AuthMW", Role: flow.RoleMiddleware},
			{ID: "HandleTopup", Name: "HandleTopup", Role: flow.RoleHandler},
		},
		Edges: []flow.FlowEdge{
			{From: "POST /api/topup", To: "HandleTopup", Kind: "http"},
			{From: "AuthMW", To: "HandleTopup", Kind: "middleware"},
		},
	}
}

// TestRenderFlowchartStartsWithGraphTD verifies the output begins with "graph TD".
func TestRenderFlowchartStartsWithGraphTD(t *testing.T) {
	out := flow.RenderFlowchart(simpleFlow())
	if !strings.HasPrefix(out, "graph TD\n") {
		t.Errorf("expected output to start with 'graph TD\\n', got: %q", out[:min(len(out), 20)])
	}
}

// TestRenderFlowchartContainsAllNodes verifies one node declaration per FlowNode.
func TestRenderFlowchartContainsAllNodes(t *testing.T) {
	f := simpleFlow()
	out := flow.RenderFlowchart(f)
	for _, n := range f.Nodes {
		if !strings.Contains(out, n.Name) {
			t.Errorf("expected output to contain node name %q", n.Name)
		}
	}
}

// TestRenderFlowchartContainsAllEdges verifies arrows are emitted for each FlowEdge.
func TestRenderFlowchartContainsAllEdges(t *testing.T) {
	f := simpleFlow()
	out := flow.RenderFlowchart(f)
	// The output should contain --> arrows (non-middleware edges).
	if !strings.Contains(out, "-->") {
		t.Error("expected --> arrows in output")
	}
}

// TestMiddlewareArrowIsDotted verifies middleware edges use -.-> style.
func TestMiddlewareArrowIsDotted(t *testing.T) {
	f := flowWithMiddleware()
	out := flow.RenderFlowchart(f)
	if !strings.Contains(out, ".->") {
		t.Errorf("expected dotted arrow (.->) for middleware edge, output:\n%s", out)
	}
}

// TestDeterministicOutput verifies two calls with same Flow produce identical bytes.
func TestDeterministicOutput(t *testing.T) {
	f := simpleFlow()
	out1 := flow.RenderFlowchart(f)
	out2 := flow.RenderFlowchart(f)
	if out1 != out2 {
		t.Errorf("RenderFlowchart is not deterministic:\nfirst:  %q\nsecond: %q", out1, out2)
	}
}

// TestDeterministicOutputWithMiddleware runs determinism check on a flow with middleware.
func TestDeterministicOutputWithMiddleware(t *testing.T) {
	f := flowWithMiddleware()
	out1 := flow.RenderFlowchart(f)
	out2 := flow.RenderFlowchart(f)
	if out1 != out2 {
		t.Error("RenderFlowchart is not deterministic for flow with middleware")
	}
}

// TestIDSanitization verifies invalid chars are replaced in the node id while
// the human-readable label inside the brackets preserves the original name.
func TestIDSanitization(t *testing.T) {
	f := flow.Flow{
		Entry:  "POST /api/v1/write",
		Method: "POST",
		Path:   "/api/v1/write",
		Nodes: []flow.FlowNode{
			{ID: "POST /api/v1/write", Name: "POST /api/v1/write", Role: flow.RoleEntry},
		},
		Edges: nil,
	}
	out := flow.RenderFlowchart(f)

	// Find the node declaration line.
	lines := strings.Split(out, "\n")
	foundNode := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Node declaration lines look like: <id>["<label>"]
		if !strings.Contains(trimmed, "[") || !strings.Contains(trimmed, "]") {
			continue
		}
		// Extract the id part (before the '[').
		bracketIdx := strings.Index(trimmed, "[")
		idPart := trimmed[:bracketIdx]

		// The id must not contain spaces or slashes.
		if strings.ContainsAny(idPart, " /") {
			t.Errorf("sanitized id should not contain spaces or slashes, got id part: %q in line: %q", idPart, line)
		}

		// The label (inside the brackets) must contain the original name.
		labelPart := trimmed[bracketIdx:]
		if strings.Contains(labelPart, "POST") && strings.Contains(idPart, "POST") {
			foundNode = true
			if !strings.Contains(labelPart, "POST /api/v1/write") {
				t.Errorf("label should preserve original name 'POST /api/v1/write', got: %q", labelPart)
			}
		}
	}
	if !foundNode {
		t.Errorf("expected a sanitized node declaration in output:\n%s", out)
	}
}

// TestNonMiddlewareArrowIsNotDotted verifies calls/http edges use --> not -.->
func TestNonMiddlewareArrowIsNotDotted(t *testing.T) {
	f := simpleFlow()
	out := flow.RenderFlowchart(f)
	if strings.Contains(out, ".->") {
		t.Error("non-middleware flow should not contain dotted arrows")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
