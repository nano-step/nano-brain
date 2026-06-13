package flow_test

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/flow"
	"github.com/nano-brain/nano-brain/internal/graph"
)

func nodeByID(nodes []flow.FlowNode, id string) (flow.FlowNode, bool) {
	for _, n := range nodes {
		if n.ID == id {
			return n, true
		}
	}
	return flow.FlowNode{}, false
}

func hasEdge(edges []flow.FlowEdge, from, to, kind string) bool {
	for _, e := range edges {
		if e.From == from && e.To == to && e.Kind == kind {
			return true
		}
	}
	return false
}

// TestMultiHopReconciliation verifies the primary spec scenario:
// POST /api/topup →(http) HandleTopup, handlers/x.go::HandleTopup →(calls) Create,
// service/s.go::Create →(calls) Save  → flow reaches Save via reconciliation.
func TestMultiHopReconciliation(t *testing.T) {
	edges := []graph.Edge{
		{SourceNode: "POST /api/topup", TargetNode: "HandleTopup", Kind: graph.EdgeHTTP},
		{SourceNode: "handlers/x.go::HandleTopup", TargetNode: "Create", Kind: graph.EdgeCalls, SourceFile: "handlers/x.go"},
		{SourceNode: "service/s.go::Create", TargetNode: "Save", Kind: graph.EdgeCalls, SourceFile: "service/s.go"},
	}

	f := flow.BuildFlow(edges, "POST /api/topup", 10, 10)

	if _, ok := nodeByID(f.Nodes, "HandleTopup"); !ok {
		t.Error("expected HandleTopup node")
	}
	if _, ok := nodeByID(f.Nodes, "Create"); !ok {
		t.Error("expected Create node")
	}
	if _, ok := nodeByID(f.Nodes, "Save"); !ok {
		t.Error("expected Save node (multi-hop via reconciliation)")
	}
	if !hasEdge(f.Edges, "POST /api/topup", "HandleTopup", "http") {
		t.Error("expected http edge from entry to HandleTopup")
	}
	if !hasEdge(f.Edges, "HandleTopup", "Create", "calls") {
		t.Error("expected calls edge HandleTopup → Create")
	}
	if !hasEdge(f.Edges, "Create", "Save", "calls") {
		t.Error("expected calls edge Create → Save")
	}
}

// TestExactMatchWouldDeadEnd verifies that without reconciliation the chain stops,
// and that BuildFlow does NOT dead-end after one hop.
func TestExactMatchWouldDeadEnd(t *testing.T) {
	// Same as multi-hop test. If the builder only did exact matching,
	// it would look for source_node=="Create" and find nothing (source is "service/s.go::Create").
	// The flow must still reach Save.
	edges := []graph.Edge{
		{SourceNode: "POST /api/items", TargetNode: "GetItems", Kind: graph.EdgeHTTP},
		{SourceNode: "handlers/items.go::GetItems", TargetNode: "FetchAll", Kind: graph.EdgeCalls, SourceFile: "handlers/items.go"},
		{SourceNode: "repo/items.go::FetchAll", TargetNode: "QueryDB", Kind: graph.EdgeCalls, SourceFile: "repo/items.go"},
	}

	f := flow.BuildFlow(edges, "POST /api/items", 10, 10)

	if _, ok := nodeByID(f.Nodes, "QueryDB"); !ok {
		t.Error("builder dead-ended: QueryDB not reached; reconciliation required for multi-hop")
	}
}

// TestAmbiguousReconciliation verifies that when two files define same symbol,
// both are included, Ambiguous=true is set, and fanout is respected.
func TestAmbiguousReconciliation(t *testing.T) {
	// "Save" is defined in two files — both should appear (ambiguous).
	edges := []graph.Edge{
		{SourceNode: "POST /api/save", TargetNode: "HandleSave", Kind: graph.EdgeHTTP},
		{SourceNode: "handlers/save.go::HandleSave", TargetNode: "Save", Kind: graph.EdgeCalls, SourceFile: "handlers/save.go"},
		// Two definitions of Save — different source files.
		{SourceNode: "repo/a.go::Save", TargetNode: "Commit", Kind: graph.EdgeCalls, SourceFile: "repo/a.go"},
		{SourceNode: "repo/b.go::Save", TargetNode: "Flush", Kind: graph.EdgeCalls, SourceFile: "repo/b.go"},
	}

	f := flow.BuildFlow(edges, "POST /api/save", 10, 10)

	saveNode, ok := nodeByID(f.Nodes, "Save")
	if !ok {
		t.Fatal("expected Save node")
	}
	if !saveNode.Ambiguous {
		t.Error("expected Save node to be marked Ambiguous (defined in 2 files)")
	}

	// Both downstream nodes should be reachable.
	if _, ok := nodeByID(f.Nodes, "Commit"); !ok {
		t.Error("expected Commit node from repo/a.go::Save")
	}
	if _, ok := nodeByID(f.Nodes, "Flush"); !ok {
		t.Error("expected Flush node from repo/b.go::Save")
	}
}

// TestExternalLeaf verifies that a bare name with no source node is classified external.
func TestExternalLeaf(t *testing.T) {
	edges := []graph.Edge{
		{SourceNode: "GET /ping", TargetNode: "HandlePing", Kind: graph.EdgeHTTP},
		{SourceNode: "handlers/ping.go::HandlePing", TargetNode: "Fprintf", Kind: graph.EdgeCalls, SourceFile: "handlers/ping.go"},
		// No source node with symbolPart == "Fprintf" — it's external.
	}

	f := flow.BuildFlow(edges, "GET /ping", 10, 10)

	fprintfNode, ok := nodeByID(f.Nodes, "Fprintf")
	if !ok {
		t.Fatal("expected Fprintf node")
	}
	if fprintfNode.Role != flow.RoleExternal {
		t.Errorf("expected Fprintf role=external, got %q", fprintfNode.Role)
	}
}

// TestMiddlewareGuard verifies middleware edges are attached without consuming depth.
func TestMiddlewareGuard(t *testing.T) {
	edges := []graph.Edge{
		{SourceNode: "POST /api/topup", TargetNode: "HandleTopup", Kind: graph.EdgeHTTP},
		{SourceNode: "AuthMW", TargetNode: "HandleTopup", Kind: graph.EdgeMiddleware},
		{SourceNode: "handlers/topup.go::HandleTopup", TargetNode: "RunTopup", Kind: graph.EdgeCalls, SourceFile: "handlers/topup.go"},
	}

	f := flow.BuildFlow(edges, "POST /api/topup", 10, 10)

	mwNode, ok := nodeByID(f.Nodes, "AuthMW")
	if !ok {
		t.Fatal("expected AuthMW node in flow")
	}
	if mwNode.Role != flow.RoleMiddleware {
		t.Errorf("expected AuthMW role=middleware, got %q", mwNode.Role)
	}
	if !hasEdge(f.Edges, "AuthMW", "HandleTopup", "middleware") {
		t.Error("expected middleware edge AuthMW → HandleTopup")
	}
	// Handler should still reach RunTopup (middleware did not consume depth).
	if _, ok := nodeByID(f.Nodes, "RunTopup"); !ok {
		t.Error("expected RunTopup node — middleware must not consume depth")
	}
}

// TestCycle verifies that cycles do not cause infinite loops and each node appears once.
func TestCycle(t *testing.T) {
	edges := []graph.Edge{
		{SourceNode: "GET /loop", TargetNode: "A", Kind: graph.EdgeHTTP},
		{SourceNode: "pkg/a.go::A", TargetNode: "B", Kind: graph.EdgeCalls, SourceFile: "pkg/a.go"},
		{SourceNode: "pkg/b.go::B", TargetNode: "A", Kind: graph.EdgeCalls, SourceFile: "pkg/b.go"}, // cycle
	}

	f := flow.BuildFlow(edges, "GET /loop", 10, 10)

	// Count occurrences of A.
	count := 0
	for _, n := range f.Nodes {
		if n.ID == "A" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected A to appear exactly once, got %d", count)
	}
}

// TestDepthCap verifies that nodes deeper than maxDepth are not included.
func TestDepthCap(t *testing.T) {
	// Chain: entry → H → L1 → L2 → L3 (depth 3 from handler)
	edges := []graph.Edge{
		{SourceNode: "GET /deep", TargetNode: "H", Kind: graph.EdgeHTTP},
		{SourceNode: "pkg/h.go::H", TargetNode: "L1", Kind: graph.EdgeCalls, SourceFile: "pkg/h.go"},
		{SourceNode: "pkg/l1.go::L1", TargetNode: "L2", Kind: graph.EdgeCalls, SourceFile: "pkg/l1.go"},
		{SourceNode: "pkg/l2.go::L2", TargetNode: "L3", Kind: graph.EdgeCalls, SourceFile: "pkg/l2.go"},
	}

	f := flow.BuildFlow(edges, "GET /deep", 2, 10)

	// With maxDepth=2: H is at depth 0, L1 at depth 1, L2 at depth 2 (but expanding L2 requires depth<2 — L2 is added but not expanded).
	// So L3 should NOT appear.
	if _, ok := nodeByID(f.Nodes, "L3"); ok {
		t.Error("L3 should not appear with maxDepth=2")
	}
	if _, ok := nodeByID(f.Nodes, "L1"); !ok {
		t.Error("L1 should appear within maxDepth=2")
	}
}

// TestFanoutCap verifies that per-node fanout is capped at maxFanout.
func TestFanoutCap(t *testing.T) {
	// Handler calls 5 targets, maxFanout=3.
	edges := []graph.Edge{
		{SourceNode: "GET /wide", TargetNode: "Wide", Kind: graph.EdgeHTTP},
		{SourceNode: "pkg/wide.go::Wide", TargetNode: "T1", Kind: graph.EdgeCalls, SourceFile: "pkg/wide.go"},
		{SourceNode: "pkg/wide.go::Wide", TargetNode: "T2", Kind: graph.EdgeCalls, SourceFile: "pkg/wide.go"},
		{SourceNode: "pkg/wide.go::Wide", TargetNode: "T3", Kind: graph.EdgeCalls, SourceFile: "pkg/wide.go"},
		{SourceNode: "pkg/wide.go::Wide", TargetNode: "T4", Kind: graph.EdgeCalls, SourceFile: "pkg/wide.go"},
		{SourceNode: "pkg/wide.go::Wide", TargetNode: "T5", Kind: graph.EdgeCalls, SourceFile: "pkg/wide.go"},
	}

	f := flow.BuildFlow(edges, "GET /wide", 10, 3)

	count := 0
	for _, n := range f.Nodes {
		switch n.ID {
		case "T1", "T2", "T3", "T4", "T5":
			count++
		}
	}
	if count > 3 {
		t.Errorf("expected at most 3 targets due to fanout cap, got %d", count)
	}
}

// TestEntryParsing verifies method and path are parsed from the entry string.
func TestEntryParsing(t *testing.T) {
	edges := []graph.Edge{
		{SourceNode: "DELETE /api/v1/items", TargetNode: "DeleteItem", Kind: graph.EdgeHTTP},
	}
	f := flow.BuildFlow(edges, "DELETE /api/v1/items", 10, 10)
	if f.Method != "DELETE" {
		t.Errorf("expected method DELETE, got %q", f.Method)
	}
	if f.Path != "/api/v1/items" {
		t.Errorf("expected path /api/v1/items, got %q", f.Path)
	}
}

// TestRoleClassification verifies name-based heuristics.
func TestRoleClassification(t *testing.T) {
	edges := []graph.Edge{
		{SourceNode: "POST /api/pay", TargetNode: "HandlePay", Kind: graph.EdgeHTTP},
		{SourceNode: "handlers/pay.go::HandlePay", TargetNode: "PaymentService", Kind: graph.EdgeCalls, SourceFile: "handlers/pay.go"},
		{SourceNode: "svc/pay.go::PaymentService", TargetNode: "UserRepo", Kind: graph.EdgeCalls, SourceFile: "svc/pay.go"},
		{SourceNode: "repo/user.go::UserRepo", TargetNode: "ExternalLeaf", Kind: graph.EdgeCalls, SourceFile: "repo/user.go"},
		// ExternalLeaf has no source node entry.
	}

	f := flow.BuildFlow(edges, "POST /api/pay", 10, 10)

	svcNode, ok := nodeByID(f.Nodes, "PaymentService")
	if !ok {
		t.Fatal("expected PaymentService node")
	}
	if svcNode.Role != flow.RoleService {
		t.Errorf("expected PaymentService role=service, got %q", svcNode.Role)
	}

	repoNode, ok := nodeByID(f.Nodes, "UserRepo")
	if !ok {
		t.Fatal("expected UserRepo node")
	}
	if repoNode.Role != flow.RoleRepo {
		t.Errorf("expected UserRepo role=repo, got %q", repoNode.Role)
	}

	extNode, ok := nodeByID(f.Nodes, "ExternalLeaf")
	if !ok {
		t.Fatal("expected ExternalLeaf node")
	}
	if extNode.Role != flow.RoleExternal {
		t.Errorf("expected ExternalLeaf role=external, got %q", extNode.Role)
	}
}

// TestUnresolvedHandler verifies the flow returns even when handler cannot be reconciled.
func TestUnresolvedHandler(t *testing.T) {
	// Entry has http edge but the handler has no calls edges and no source node.
	edges := []graph.Edge{
		{SourceNode: "POST /api/noop", TargetNode: "MysteryHandler", Kind: graph.EdgeHTTP},
		// No calls edges from MysteryHandler.
	}

	f := flow.BuildFlow(edges, "POST /api/noop", 10, 10)

	if _, ok := nodeByID(f.Nodes, "MysteryHandler"); !ok {
		t.Error("expected MysteryHandler as terminal node even when unresolved")
	}
	if f.Entry != "POST /api/noop" {
		t.Error("expected entry to be set")
	}
}
