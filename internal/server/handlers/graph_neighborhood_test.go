package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

type mockNeighborhoodQuerier struct {
	outEdges []sqlc.GraphEdge
}

func (m *mockNeighborhoodQuerier) GetOutgoingEdges(_ context.Context, _ sqlc.GetOutgoingEdgesParams) ([]sqlc.GraphEdge, error) {
	return m.outEdges, nil
}
func (m *mockNeighborhoodQuerier) GetIncomingEdges(_ context.Context, _ sqlc.GetIncomingEdgesParams) ([]sqlc.GraphEdge, error) {
	return nil, nil
}
func (m *mockNeighborhoodQuerier) GetEdgesByNodes(_ context.Context, _ sqlc.GetEdgesByNodesParams) ([]sqlc.GraphEdge, error) {
	return nil, nil
}
func (m *mockNeighborhoodQuerier) ListDocumentsByIDs(_ context.Context, _ sqlc.ListDocumentsByIDsParams) ([]sqlc.ListDocumentsByIDsRow, error) {
	return nil, nil
}

func TestGraphNeighborhood_SymbolDefault(t *testing.T) {
	e := echo.New()
	h := handlers.GraphNeighborhood(&mockNeighborhoodQuerier{}, nopLogger())

	body := `{"focus":"pkg/main.go::main","depth":1,"workspace":"w1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/graph/neighborhood", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "w1")

	if err := h(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}

	var resp struct {
		NodeKind string `json:"node_kind"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.NodeKind != "symbol" {
		t.Errorf("expected node_kind=symbol, got %q", resp.NodeKind)
	}
}

func TestGraphNeighborhood_InvalidNodeKind(t *testing.T) {
	e := echo.New()
	h := handlers.GraphNeighborhood(&mockNeighborhoodQuerier{}, nopLogger())

	body := `{"focus":"x","depth":1,"node_kind":"invalid","workspace":"w1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/graph/neighborhood", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "w1")

	err := h(c)
	if err == nil {
		t.Fatal("expected error for invalid node_kind")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok || he.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %v", err)
	}
}

func TestGraphNeighborhood_InvalidDepth(t *testing.T) {
	e := echo.New()
	h := handlers.GraphNeighborhood(&mockNeighborhoodQuerier{}, nopLogger())

	body := `{"focus":"x","depth":10,"workspace":"w1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/graph/neighborhood", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "w1")

	err := h(c)
	if err == nil {
		t.Fatal("expected error for invalid depth")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok || he.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %v", err)
	}
}

func TestGraphNeighborhood_UnknownFocusEmpty(t *testing.T) {
	e := echo.New()
	h := handlers.GraphNeighborhood(&mockNeighborhoodQuerier{}, nopLogger())

	body := `{"focus":"nonexistent","depth":1,"workspace":"w1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/graph/neighborhood", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "w1")

	if err := h(c); err != nil {
		t.Fatal(err)
	}

	var resp struct {
		Nodes []interface{} `json:"nodes"`
		Edges []interface{} `json:"edges"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Edges) != 0 {
		t.Error("expected empty edges for unknown focus")
	}
}

type docModeNeighborhoodQuerier struct {
	outEdges    []sqlc.GraphEdge
	inEdges     []sqlc.GraphEdge
	docRows     []sqlc.ListDocumentsByIDsRow
	capturedOut []sqlc.GetOutgoingEdgesParams
}

func (m *docModeNeighborhoodQuerier) GetOutgoingEdges(_ context.Context, arg sqlc.GetOutgoingEdgesParams) ([]sqlc.GraphEdge, error) {
	m.capturedOut = append(m.capturedOut, arg)
	return m.outEdges, nil
}
func (m *docModeNeighborhoodQuerier) GetIncomingEdges(_ context.Context, _ sqlc.GetIncomingEdgesParams) ([]sqlc.GraphEdge, error) {
	return m.inEdges, nil
}
func (m *docModeNeighborhoodQuerier) GetEdgesByNodes(_ context.Context, _ sqlc.GetEdgesByNodesParams) ([]sqlc.GraphEdge, error) {
	return nil, nil
}
func (m *docModeNeighborhoodQuerier) ListDocumentsByIDs(_ context.Context, _ sqlc.ListDocumentsByIDsParams) ([]sqlc.ListDocumentsByIDsRow, error) {
	return m.docRows, nil
}

func TestGraphNeighborhood_DocModeEnrichment(t *testing.T) {
	focusID := uuid.New()
	neighborID := uuid.New()
	now := time.Now().Truncate(time.Second)

	q := &docModeNeighborhoodQuerier{
		outEdges: []sqlc.GraphEdge{
			{SourceNode: focusID.String(), TargetNode: neighborID.String(), EdgeType: "references", WorkspaceHash: "w1"},
		},
		docRows: []sqlc.ListDocumentsByIDsRow{
			{ID: focusID, Title: "Focus Doc", Collection: "memory", UpdatedAt: now, Tags: []string{"a"}},
			{ID: neighborID, Title: "Neighbor Doc", Collection: "code", UpdatedAt: now, Tags: []string{"b"}},
		},
	}

	e := echo.New()
	h := handlers.GraphNeighborhood(q, nopLogger())

	body := `{"focus":"` + focusID.String() + `","depth":1,"direction":"both","node_kind":"doc","workspace":"w1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/graph/neighborhood", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "w1")

	if err := h(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}

	var resp struct {
		NodeKind string `json:"node_kind"`
		Nodes    []struct {
			ID         string   `json:"id"`
			Title      string   `json:"title"`
			Collection string   `json:"collection"`
			UpdatedAt  *string  `json:"updated_at"`
			Tags       []string `json:"tags"`
		} `json:"nodes"`
		Edges []struct {
			EdgeType string `json:"edge_type"`
		} `json:"edges"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.NodeKind != "doc" {
		t.Errorf("expected node_kind=doc, got %q", resp.NodeKind)
	}
	if len(resp.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(resp.Nodes))
	}

	enriched := 0
	for _, n := range resp.Nodes {
		if n.Title != "" && n.Collection != "" && n.UpdatedAt != nil {
			enriched++
		}
	}
	if enriched != 2 {
		t.Errorf("expected 2 enriched nodes (title+collection+updated_at), got %d", enriched)
	}
}

func TestGraphNeighborhood_DocModeEdgeFilter(t *testing.T) {
	focusID := uuid.New()
	neighborID := uuid.New()

	q := &docModeNeighborhoodQuerier{
		outEdges: []sqlc.GraphEdge{
			{SourceNode: focusID.String(), TargetNode: neighborID.String(), EdgeType: "references", WorkspaceHash: "w1"},
		},
	}

	e := echo.New()
	h := handlers.GraphNeighborhood(q, nopLogger())

	body := `{"focus":"` + focusID.String() + `","depth":1,"direction":"out","node_kind":"doc","edge_types":["calls","contains"],"workspace":"w1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/graph/neighborhood", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "w1")

	if err := h(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}

	if len(q.capturedOut) == 0 {
		t.Fatal("expected at least one GetOutgoingEdges call")
	}
	if q.capturedOut[0].Column3 != "references" {
		t.Errorf("expected server to force edge filter to 'references' for doc mode, got %q", q.capturedOut[0].Column3)
	}

	var resp struct {
		Edges []struct {
			EdgeType string `json:"edge_type"`
		} `json:"edges"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	for _, edge := range resp.Edges {
		if edge.EdgeType != "references" {
			t.Errorf("expected edge_type=references, got %q", edge.EdgeType)
		}
	}
}
