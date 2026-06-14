package flow_test

import (
	"strings"
	"testing"

	"github.com/nano-brain/nano-brain/internal/flow"
)

func seqSimpleFlow() flow.Flow {
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

func seqFlowWithMiddleware() flow.Flow {
	return flow.Flow{
		Entry:  "POST /api/topup",
		Method: "POST",
		Path:   "/api/topup",
		Nodes: []flow.FlowNode{
			{ID: "POST /api/topup", Name: "POST /api/topup", Role: flow.RoleEntry},
			{ID: "AuthMW", Name: "AuthMW", Role: flow.RoleMiddleware},
			{ID: "HandleTopup", Name: "HandleTopup", Role: flow.RoleHandler},
			{ID: "PayService", Name: "PayService", Role: flow.RoleService},
		},
		Edges: []flow.FlowEdge{
			{From: "POST /api/topup", To: "HandleTopup", Kind: "http"},
			{From: "AuthMW", To: "HandleTopup", Kind: "middleware"},
			{From: "HandleTopup", To: "PayService", Kind: "calls"},
		},
	}
}

func TestRenderSequenceDiagramHeader(t *testing.T) {
	out := flow.RenderSequenceDiagram(seqSimpleFlow())
	if !strings.HasPrefix(out, "sequenceDiagram\n") {
		t.Errorf("expected output to start with 'sequenceDiagram\\n', got: %q", out[:min(len(out), 30)])
	}
}

func TestRenderSequenceDiagramClientParticipant(t *testing.T) {
	out := flow.RenderSequenceDiagram(seqSimpleFlow())
	if !strings.Contains(out, "participant Client") {
		t.Errorf("expected 'participant Client' for the entry node, output:\n%s", out)
	}
	// Entry's raw name should not appear as a participant alias (it should be Client).
	if strings.Contains(out, "participant POST") {
		t.Errorf("entry node raw name should not appear as participant, output:\n%s", out)
	}
}

func TestRenderSequenceDiagramAllParticipants(t *testing.T) {
	f := seqSimpleFlow()
	out := flow.RenderSequenceDiagram(f)
	for _, name := range []string{"HandleTopup", "PayService", "PayRepo"} {
		if !strings.Contains(out, name) {
			t.Errorf("expected participant %q in output:\n%s", name, out)
		}
	}
}

func TestRenderSequenceDiagramMessages(t *testing.T) {
	out := flow.RenderSequenceDiagram(seqSimpleFlow())
	// Must contain ->> arrows.
	if !strings.Contains(out, "->>") {
		t.Errorf("expected ->> arrows in sequence diagram, output:\n%s", out)
	}
	// Entry http edge: Client->>HandleTopup.
	if !strings.Contains(out, "Client->>HandleTopup") {
		t.Errorf("expected 'Client->>HandleTopup' message, output:\n%s", out)
	}
	// Downstream calls.
	if !strings.Contains(out, "HandleTopup->>PayService") {
		t.Errorf("expected 'HandleTopup->>PayService' message, output:\n%s", out)
	}
	if !strings.Contains(out, "PayService->>PayRepo") {
		t.Errorf("expected 'PayService->>PayRepo' message, output:\n%s", out)
	}
}

func TestRenderSequenceDiagramMiddlewareNote(t *testing.T) {
	out := flow.RenderSequenceDiagram(seqFlowWithMiddleware())
	// Middleware should appear as a Note over, not as an arrow.
	if !strings.Contains(out, "Note over") {
		t.Errorf("expected 'Note over' for middleware guard, output:\n%s", out)
	}
	if !strings.Contains(out, "AuthMW") {
		t.Errorf("expected AuthMW in middleware note, output:\n%s", out)
	}
	// Middleware should NOT generate a ->> message.
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "AuthMW->>") {
			t.Errorf("middleware should not produce a ->> message, got: %q", line)
		}
	}
}

func TestRenderSequenceDiagramDeterministic(t *testing.T) {
	f := seqSimpleFlow()
	out1 := flow.RenderSequenceDiagram(f)
	out2 := flow.RenderSequenceDiagram(f)
	if out1 != out2 {
		t.Errorf("RenderSequenceDiagram is not deterministic")
	}
}

func TestRenderSequenceDiagramDeterministicWithMiddleware(t *testing.T) {
	f := seqFlowWithMiddleware()
	out1 := flow.RenderSequenceDiagram(f)
	out2 := flow.RenderSequenceDiagram(f)
	if out1 != out2 {
		t.Error("RenderSequenceDiagram is not deterministic for flow with middleware")
	}
}

func TestRenderSequenceDiagramEmptyFlow(t *testing.T) {
	f := flow.Flow{}
	out := flow.RenderSequenceDiagram(f)
	if !strings.HasPrefix(out, "sequenceDiagram\n") {
		t.Errorf("empty flow should still produce valid header, got: %q", out)
	}
}

// TestRenderSequenceDiagramLineOrdering verifies that calls are ordered by source
// line number when line info is present, not alphabetically.
func TestRenderSequenceDiagramLineOrdering(t *testing.T) {
	// HandleTopup calls ZService on line 10, then AService on line 20.
	// Without line ordering, AService would come first (alphabetical).
	f := flow.Flow{
		Entry:  "POST /api/topup",
		Method: "POST",
		Path:   "/api/topup",
		Nodes: []flow.FlowNode{
			{ID: "POST /api/topup", Name: "POST /api/topup", Role: flow.RoleEntry},
			{ID: "HandleTopup", Name: "HandleTopup", Role: flow.RoleHandler},
			{ID: "ZService", Name: "ZService", Role: flow.RoleService},
			{ID: "AService", Name: "AService", Role: flow.RoleService},
		},
		Edges: []flow.FlowEdge{
			{From: "POST /api/topup", To: "HandleTopup", Kind: "http"},
			{From: "HandleTopup", To: "ZService", Kind: "calls", Line: 10},
			{From: "HandleTopup", To: "AService", Kind: "calls", Line: 20},
		},
	}
	out := flow.RenderSequenceDiagram(f)

	// ZService (line 10) must appear before AService (line 20).
	zPos := strings.Index(out, "ZService")
	aPos := strings.Index(out, "AService")
	if zPos < 0 || aPos < 0 {
		t.Fatalf("expected both ZService and AService in output:\n%s", out)
	}
	if zPos > aPos {
		t.Errorf("ZService (line 10) should appear before AService (line 20) in output:\n%s", out)
	}
}

func seqFlowWithConditionalEdges() flow.Flow {
	return flow.Flow{
		Entry:  "POST /api/data",
		Method: "POST",
		Path:   "/api/data",
		Nodes: []flow.FlowNode{
			{ID: "POST /api/data", Name: "POST /api/data", Role: flow.RoleEntry},
			{ID: "HandleData", Name: "HandleData", Role: flow.RoleHandler},
			{ID: "IfBranch", Name: "IfBranch", Role: flow.RoleService},
			{ID: "ElseBranch", Name: "ElseBranch", Role: flow.RoleService},
			{ID: "NormalCall", Name: "NormalCall", Role: flow.RoleService},
		},
		Edges: []flow.FlowEdge{
			{From: "POST /api/data", To: "HandleData", Kind: "http"},
			{From: "HandleData", To: "IfBranch", Kind: "calls", Conditional: true},
			{From: "HandleData", To: "ElseBranch", Kind: "calls", Conditional: true},
			{From: "HandleData", To: "NormalCall", Kind: "calls", Conditional: false},
		},
	}
}

func TestRenderSequenceDiagramAltBlock(t *testing.T) {
	f := seqFlowWithConditionalEdges()
	out := flow.RenderSequenceDiagram(f)

	if !strings.Contains(out, "alt conditional") {
		t.Errorf("expected 'alt conditional' for consecutive conditional messages, output:\n%s", out)
	}
	if !strings.Contains(out, "end") {
		t.Errorf("expected 'end' to close alt block, output:\n%s", out)
	}
	altPos := strings.Index(out, "alt conditional")
	if altPos < 0 {
		t.Fatal("missing 'alt conditional' in output")
	}
	ifArrow := strings.Index(out, "HandleData->>IfBranch")
	elseArrow := strings.Index(out, "HandleData->>ElseBranch")
	if ifArrow < 0 || elseArrow < 0 {
		t.Fatalf("expected message arrows for IfBranch and ElseBranch in output:\n%s", out)
	}
	if ifArrow < altPos {
		t.Errorf("IfBranch message arrow should appear after 'alt', output:\n%s", out)
	}
	if elseArrow < altPos {
		t.Errorf("ElseBranch message arrow should appear after 'alt', output:\n%s", out)
	}
}

func TestRenderSequenceDiagramOptBlock(t *testing.T) {
	f := flow.Flow{
		Entry:  "GET /api/check",
		Method: "GET",
		Path:   "/api/check",
		Nodes: []flow.FlowNode{
			{ID: "GET /api/check", Name: "GET /api/check", Role: flow.RoleEntry},
			{ID: "H", Name: "H", Role: flow.RoleHandler},
			{ID: "Maybe", Name: "Maybe", Role: flow.RoleService},
			{ID: "Always", Name: "Always", Role: flow.RoleService},
		},
		Edges: []flow.FlowEdge{
			{From: "GET /api/check", To: "H", Kind: "http"},
			{From: "H", To: "Maybe", Kind: "calls", Conditional: true},
			{From: "H", To: "Always", Kind: "calls", Conditional: false},
		},
	}
	out := flow.RenderSequenceDiagram(f)

	if !strings.Contains(out, "opt conditional") {
		t.Errorf("expected 'opt conditional' for single conditional message, output:\n%s", out)
	}
	if !strings.Contains(out, "end") {
		t.Errorf("expected 'end' to close opt block, output:\n%s", out)
	}
}

func TestRenderSequenceDiagramNonConditionalUnchanged(t *testing.T) {
	f := seqSimpleFlow()
	out := flow.RenderSequenceDiagram(f)
	if strings.Contains(out, "conditional") {
		t.Errorf("flow with no conditional edges should not mention 'conditional', output:\n%s", out)
	}
}

func TestRenderSequenceDiagramRoleLabels(t *testing.T) {
	out := flow.RenderSequenceDiagram(seqSimpleFlow())
	// Non-entry participants should include the role in their label.
	if !strings.Contains(out, "(handler)") {
		t.Errorf("expected '(handler)' role label in participant declaration, output:\n%s", out)
	}
	if !strings.Contains(out, "(service)") {
		t.Errorf("expected '(service)' role label in participant declaration, output:\n%s", out)
	}
}
