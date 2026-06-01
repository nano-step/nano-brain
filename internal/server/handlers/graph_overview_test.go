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
)

type mockOverviewQuerier struct {
	topRows  []sqlc.ListTopGraphNodesByDegreeRow
	count    int64
	edgeRows []sqlc.GraphEdge
	lastEdgeTypes []string
	lastLimit     int32
}

func (m *mockOverviewQuerier) ListTopGraphNodesByDegree(_ context.Context, arg sqlc.ListTopGraphNodesByDegreeParams) ([]sqlc.ListTopGraphNodesByDegreeRow, error) {
	m.lastEdgeTypes = arg.Column2
	m.lastLimit = arg.Limit
	return m.topRows, nil
}

func (m *mockOverviewQuerier) CountDistinctGraphNodes(_ context.Context, _ sqlc.CountDistinctGraphNodesParams) (int64, error) {
	return m.count, nil
}

func (m *mockOverviewQuerier) ListEdgesTouchingNodes(_ context.Context, _ sqlc.ListEdgesTouchingNodesParams) ([]sqlc.GraphEdge, error) {
	return m.edgeRows, nil
}

func (m *mockOverviewQuerier) ListDocumentsByIDs(_ context.Context, _ sqlc.ListDocumentsByIDsParams) ([]sqlc.ListDocumentsByIDsRow, error) {
	return nil, nil
}

func runOverviewHandler(t *testing.T, q *mockOverviewQuerier, body string) (*httptest.ResponseRecorder, error) {
	t.Helper()
	h := handlers.GraphOverview(q, nopLogger())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/graph/overview", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.Set("workspace", "abc")
	return rec, h(c)
}

func TestGraphOverview_ResponseShape(t *testing.T) {
	q := &mockOverviewQuerier{
		topRows: []sqlc.ListTopGraphNodesByDegreeRow{
			{Node: "func1", Degree: 10},
			{Node: "func2", Degree: 5},
		},
		count: 2,
		edgeRows: []sqlc.GraphEdge{
			{SourceNode: "func1", TargetNode: "func2", EdgeType: "calls"},
		},
	}
	rec, err := runOverviewHandler(t, q, `{"mode":"code","limit":50}`)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"node_kind", "nodes", "edges", "truncated"} {
		if _, ok := resp[k]; !ok {
			t.Errorf("missing key %q", k)
		}
	}
	nodes := resp["nodes"].([]interface{})
	if len(nodes) != 2 {
		t.Errorf("got %d nodes, want 2", len(nodes))
	}
	if resp["truncated"] != false {
		t.Errorf("truncated = %v, want false", resp["truncated"])
	}
	if resp["node_kind"] != "symbol" {
		t.Errorf("node_kind = %v, want 'symbol' for code mode", resp["node_kind"])
	}
}

func TestGraphOverview_CodeModeDefaults(t *testing.T) {
	q := &mockOverviewQuerier{count: 0}
	_, err := runOverviewHandler(t, q, `{"mode":"code"}`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"calls", "imports", "contains"}
	if len(q.lastEdgeTypes) != 3 {
		t.Fatalf("got %d edge types, want 3", len(q.lastEdgeTypes))
	}
	for i, et := range want {
		if q.lastEdgeTypes[i] != et {
			t.Errorf("edge_type[%d] = %q, want %q", i, q.lastEdgeTypes[i], et)
		}
	}
}

func TestGraphOverview_KnowledgeModeDefaults(t *testing.T) {
	q := &mockOverviewQuerier{count: 0}
	_, err := runOverviewHandler(t, q, `{"mode":"knowledge"}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(q.lastEdgeTypes) != 1 || q.lastEdgeTypes[0] != "references" {
		t.Errorf("knowledge mode edge_types = %v, want [references]", q.lastEdgeTypes)
	}
}

func TestGraphOverview_EmptyWorkspace(t *testing.T) {
	q := &mockOverviewQuerier{count: 0}
	rec, err := runOverviewHandler(t, q, `{"mode":"code"}`)
	if err != nil {
		t.Fatal(err)
	}
	var resp struct {
		Nodes     []interface{} `json:"nodes"`
		Edges     []interface{} `json:"edges"`
		Truncated bool          `json:"truncated"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Nodes == nil {
		t.Error("nodes must be empty array, not null")
	}
	if resp.Edges == nil {
		t.Error("edges must be empty array, not null")
	}
	if len(resp.Nodes) != 0 || len(resp.Edges) != 0 {
		t.Errorf("expected empty graph, got %d nodes %d edges", len(resp.Nodes), len(resp.Edges))
	}
	if resp.Truncated {
		t.Error("truncated should be false for empty workspace")
	}
}

func TestGraphOverview_LimitClamping(t *testing.T) {
	q := &mockOverviewQuerier{count: 0}

	_, err := runOverviewHandler(t, q, `{"mode":"code"}`)
	if err != nil {
		t.Fatal(err)
	}
	if q.lastLimit != 50 {
		t.Errorf("missing limit → got %d, want default 50", q.lastLimit)
	}

	_, err = runOverviewHandler(t, q, `{"mode":"code","limit":0}`)
	if err != nil {
		t.Fatal(err)
	}
	if q.lastLimit != 50 {
		t.Errorf("limit=0 → got %d, want default 50", q.lastLimit)
	}

	_, err = runOverviewHandler(t, q, `{"mode":"code","limit":500}`)
	if err != nil {
		t.Fatal(err)
	}
	if q.lastLimit != 200 {
		t.Errorf("limit=500 → got %d, want clamped 200", q.lastLimit)
	}
}

func TestGraphOverview_TruncatedFlag(t *testing.T) {
	q := &mockOverviewQuerier{
		topRows: []sqlc.ListTopGraphNodesByDegreeRow{{Node: "func1"}},
		count:   100,
	}
	rec, err := runOverviewHandler(t, q, `{"mode":"code","limit":50}`)
	if err != nil {
		t.Fatal(err)
	}
	var resp struct {
		Truncated bool `json:"truncated"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.Truncated {
		t.Errorf("truncated should be true when total (100) > limit (50)")
	}
}

func TestGraphOverview_IncludesImplicitEndpointNodes(t *testing.T) {
	q := &mockOverviewQuerier{
		topRows: []sqlc.ListTopGraphNodesByDegreeRow{
			{Node: "hub", Degree: 100},
		},
		count: 50,
		edgeRows: []sqlc.GraphEdge{
			{SourceNode: "hub", TargetNode: "leaf1", EdgeType: "calls"},
			{SourceNode: "hub", TargetNode: "leaf2", EdgeType: "calls"},
			{SourceNode: "external", TargetNode: "hub", EdgeType: "imports"},
		},
	}
	rec, err := runOverviewHandler(t, q, `{"mode":"code","limit":50}`)
	if err != nil {
		t.Fatal(err)
	}
	var resp struct {
		Nodes []struct {
			ID string `json:"id"`
		} `json:"nodes"`
		Edges []struct {
			Source string `json:"source"`
			Target string `json:"target"`
		} `json:"edges"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Nodes) != 4 {
		t.Errorf("expected 4 nodes (hub + 3 implicit), got %d: %+v", len(resp.Nodes), resp.Nodes)
	}
	if len(resp.Edges) != 3 {
		t.Errorf("expected 3 edges, got %d", len(resp.Edges))
	}
	wantIDs := map[string]bool{"hub": false, "leaf1": false, "leaf2": false, "external": false}
	for _, n := range resp.Nodes {
		if _, ok := wantIDs[n.ID]; ok {
			wantIDs[n.ID] = true
		}
	}
	for id, found := range wantIDs {
		if !found {
			t.Errorf("missing expected node %q", id)
		}
	}
}
