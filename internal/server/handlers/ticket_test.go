package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

// --- mock ---

type mockTicketQuerier struct {
	rows []sqlc.ListDocumentsByTagRow
	err  error
	// captured args for assertions
	capturedTag        string
	capturedCollection string
	capturedLimit      int32
}

func (m *mockTicketQuerier) ListDocumentsByTag(ctx context.Context, arg sqlc.ListDocumentsByTagParams) ([]sqlc.ListDocumentsByTagRow, error) {
	m.capturedTag = arg.Column1
	m.capturedCollection = arg.Collection
	m.capturedLimit = arg.Limit
	return m.rows, m.err
}

// --- helpers ---

func newEchoWithTicketParam(ticket string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/by-ticket?ticket="+ticket, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	return c, rec
}

func makeRow(workspaceHash, title, sourcePath string, tags []string, content string) sqlc.ListDocumentsByTagRow {
	return sqlc.ListDocumentsByTagRow{
		ID:            uuid.New(),
		WorkspaceHash: workspaceHash,
		Title:         title,
		SourcePath:    sourcePath,
		Collection:    "sessions",
		Tags:          tags,
		Content:       content,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}

func decodeResults(t *testing.T, rec *httptest.ResponseRecorder) []handlers.TicketSessionResult {
	t.Helper()
	var out []handlers.TicketSessionResult
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

// --- tests ---

// TestTicketHandler_CrossWorkspace: two docs from different workspaces → both returned.
func TestTicketHandler_CrossWorkspace(t *testing.T) {
	row1 := makeRow("ws-aaa", "Session A", "summary://claude/sess-1",
		[]string{"ticket:DEV-4706", "claude_code"}, "Content about DEV-4706 in workspace A")
	row2 := makeRow("ws-bbb", "Session B", "summary://opencode/sess-2",
		[]string{"ticket:DEV-4706", "opencode"}, "Content about DEV-4706 in workspace B")

	mock := &mockTicketQuerier{rows: []sqlc.ListDocumentsByTagRow{row1, row2}}
	c, rec := newEchoWithTicketParam("DEV-4706")

	err := handlers.TicketHandler(mock, zerolog.Nop())(c)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	results := decodeResults(t, rec)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	workspaces := map[string]bool{}
	for _, r := range results {
		workspaces[r.WorkspaceHash] = true
	}
	if !workspaces["ws-aaa"] || !workspaces["ws-bbb"] {
		t.Errorf("expected both workspace hashes in results, got %v", workspaces)
	}

	// tag value sent to querier must be "ticket:DEV-4706"
	if mock.capturedTag != "ticket:DEV-4706" {
		t.Errorf("expected tag 'ticket:DEV-4706', got %q", mock.capturedTag)
	}
	// collection must always be "sessions"
	if mock.capturedCollection != "sessions" {
		t.Errorf("expected collection 'sessions', got %q", mock.capturedCollection)
	}
}

// TestTicketHandler_EmptyTicket: missing ticket param → 400.
func TestTicketHandler_EmptyTicket(t *testing.T) {
	mock := &mockTicketQuerier{}
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/by-ticket", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handlers.TicketHandler(mock, zerolog.Nop())(c)
	if err == nil {
		t.Fatal("expected an HTTP error for missing ticket param")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected *echo.HTTPError, got %T", err)
	}
	if he.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", he.Code)
	}
}

// TestTicketHandler_QuerierError: querier returns error → 500.
func TestTicketHandler_QuerierError(t *testing.T) {
	mock := &mockTicketQuerier{err: errors.New("db failure")}
	c, _ := newEchoWithTicketParam("DEV-4706")

	err := handlers.TicketHandler(mock, zerolog.Nop())(c)
	if err == nil {
		t.Fatal("expected an HTTP error when querier fails")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected *echo.HTTPError, got %T", err)
	}
	if he.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", he.Code)
	}
}

// TestTicketHandler_HashStyleTicket: ticket=#42 → tag "ticket:#42".
func TestTicketHandler_HashStyleTicket(t *testing.T) {
	row := makeRow("ws-aaa", "PR session", "summary://claude/sess-99",
		[]string{"ticket:#42", "claude"}, "Fixed issue #42")
	mock := &mockTicketQuerier{rows: []sqlc.ListDocumentsByTagRow{row}}
	c, rec := newEchoWithTicketParam("%2342") // URL-encoded "#42"

	err := handlers.TicketHandler(mock, zerolog.Nop())(c)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if mock.capturedTag != "ticket:#42" {
		t.Errorf("expected tag 'ticket:#42', got %q", mock.capturedTag)
	}
}

// TestTicketHandler_LimitEnforced: querier is called with limit=50 regardless of result count.
func TestTicketHandler_LimitEnforced(t *testing.T) {
	// Build 60 rows; the query uses LIMIT 50 in SQL, so the mock just returns what
	// it has. The important thing is the handler calls with Limit=50.
	rows := make([]sqlc.ListDocumentsByTagRow, 60)
	for i := range rows {
		rows[i] = makeRow("ws-aaa", "Session", "summary://claude/sess", []string{"ticket:DEV-1"}, "content")
	}
	mock := &mockTicketQuerier{rows: rows}
	c, rec := newEchoWithTicketParam("DEV-1")

	err := handlers.TicketHandler(mock, zerolog.Nop())(c)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if mock.capturedLimit != 50 {
		t.Errorf("expected limit 50 passed to querier, got %d", mock.capturedLimit)
	}
}

// TestTicketHandler_SourceFromPath: when tags lack a source tag, source is derived from source_path.
func TestTicketHandler_SourceFromPath(t *testing.T) {
	row := makeRow("ws-aaa", "Claude session", "summary://claude/sess-10",
		[]string{"ticket:DEV-100"}, "content") // no "claude" tag
	mock := &mockTicketQuerier{rows: []sqlc.ListDocumentsByTagRow{row}}
	c, rec := newEchoWithTicketParam("DEV-100")

	if err := handlers.TicketHandler(mock, zerolog.Nop())(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	results := decodeResults(t, rec)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Source != "claude" {
		t.Errorf("expected source 'claude', got %q", results[0].Source)
	}
}

// TestTicketHandler_UnknownTicket: querier returns empty → handler returns empty array (not error).
func TestTicketHandler_UnknownTicket(t *testing.T) {
	mock := &mockTicketQuerier{rows: nil}
	c, rec := newEchoWithTicketParam("UNKNOWN-9999")

	err := handlers.TicketHandler(mock, zerolog.Nop())(c)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	results := decodeResults(t, rec)
	if len(results) != 0 {
		t.Errorf("expected empty array, got %d results", len(results))
	}
}
