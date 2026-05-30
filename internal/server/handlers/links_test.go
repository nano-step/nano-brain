package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

type mockBacklinksQuerier struct {
	rows  []sqlc.ListBacklinksByTargetRow
	total int64
}

func (m *mockBacklinksQuerier) ListBacklinksByTarget(_ context.Context, _ sqlc.ListBacklinksByTargetParams) ([]sqlc.ListBacklinksByTargetRow, error) {
	return m.rows, nil
}
func (m *mockBacklinksQuerier) CountBacklinksByTarget(_ context.Context, _ sqlc.CountBacklinksByTargetParams) (int64, error) {
	return m.total, nil
}

type mockLinkQueryResolver struct {
	idExists bool
	idErr    error
	titleIDs []uuid.UUID
	titleErr error
}

func (m *mockLinkQueryResolver) ResolveID(_ context.Context, _ string, _ uuid.UUID) (bool, error) {
	return m.idExists, m.idErr
}
func (m *mockLinkQueryResolver) ResolveTitle(_ context.Context, _, _ string) ([]uuid.UUID, error) {
	return m.titleIDs, m.titleErr
}

func TestResolveLink_ByID(t *testing.T) {
	id := uuid.New()
	r := &mockLinkQueryResolver{idExists: true}
	e := echo.New()
	h := handlers.ResolveLink(r, nopLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/links/resolve?query="+id.String()+"&workspace=w1", nil)
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
		Results []struct {
			ID    string `json:"id"`
			Match string `json:"match"`
		} `json:"results"`
		Query string `json:"query"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Results))
	}
	if resp.Results[0].ID != id.String() {
		t.Errorf("expected id %s, got %s", id.String(), resp.Results[0].ID)
	}
	if resp.Results[0].Match != "id" {
		t.Errorf("expected match=id, got %q", resp.Results[0].Match)
	}
}

func TestResolveLink_ByTitle(t *testing.T) {
	docID := uuid.New()
	r := &mockLinkQueryResolver{titleIDs: []uuid.UUID{docID}}
	e := echo.New()
	h := handlers.ResolveLink(r, nopLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/links/resolve?query=My+Document&workspace=w1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "w1")

	if err := h(c); err != nil {
		t.Fatal(err)
	}

	var resp struct {
		Results []struct {
			ID    string `json:"id"`
			Match string `json:"match"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Results))
	}
	if resp.Results[0].ID != docID.String() {
		t.Errorf("expected id %s, got %s", docID.String(), resp.Results[0].ID)
	}
	if resp.Results[0].Match != "title" {
		t.Errorf("expected match=title, got %q", resp.Results[0].Match)
	}
}

func TestResolveLink_Ambiguous(t *testing.T) {
	id1, id2 := uuid.New(), uuid.New()
	r := &mockLinkQueryResolver{titleIDs: []uuid.UUID{id1, id2}}
	e := echo.New()
	h := handlers.ResolveLink(r, nopLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/links/resolve?query=Shared+Title&workspace=w1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "w1")

	if err := h(c); err != nil {
		t.Fatal(err)
	}

	var resp struct {
		Results []struct {
			Match string `json:"match"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("expected 2 results (ambiguous), got %d", len(resp.Results))
	}
	for _, r := range resp.Results {
		if r.Match != "title" {
			t.Errorf("expected match=title, got %q", r.Match)
		}
	}
}

func TestResolveLink_NotFound(t *testing.T) {
	r := &mockLinkQueryResolver{idExists: false, titleIDs: nil}
	e := echo.New()
	h := handlers.ResolveLink(r, nopLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/links/resolve?query="+uuid.New().String()+"&workspace=w1", nil)
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
		Results []interface{} `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(resp.Results))
	}
}

func TestBacklinks_EmptyResult(t *testing.T) {
	e := echo.New()
	h := handlers.Backlinks(&mockBacklinksQuerier{}, nopLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/links/some-doc/backlinks?workspace=w1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "w1")
	c.SetParamNames("doc_id")
	c.SetParamValues("some-doc")

	if err := h(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}

	var resp struct {
		Total int             `json:"total"`
		Items json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Total != 0 {
		t.Errorf("expected 0 total, got %d", resp.Total)
	}
}

func TestBacklinks_PopulatedResult(t *testing.T) {
	docID := uuid.New()
	q := &mockBacklinksQuerier{
		total: 1,
		rows: []sqlc.ListBacklinksByTargetRow{{
			ID: docID, Title: "Linker Doc", Collection: "memory",
			UpdatedAt: time.Now(), Tags: []string{"test"},
			Content: "See [[some-doc]] for details",
		}},
	}
	e := echo.New()
	h := handlers.Backlinks(q, nopLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/links/some-doc/backlinks?workspace=w1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "w1")
	c.SetParamNames("doc_id")
	c.SetParamValues("some-doc")

	if err := h(c); err != nil {
		t.Fatal(err)
	}

	var resp struct {
		Total int `json:"total"`
		Items []struct {
			Snippet string `json:"snippet"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Total != 1 {
		t.Errorf("expected 1, got %d", resp.Total)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}
	if resp.Items[0].Snippet == "" {
		t.Error("expected non-empty snippet")
	}
}
