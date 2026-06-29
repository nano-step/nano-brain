package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/harvest"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type mockSummarizeSummarizer struct {
	summarizeErr error
	called       int
	capturedMeta harvest.SummaryMeta
}

func (m *mockSummarizeSummarizer) SummarizeAndPersist(_ context.Context, _ string, meta harvest.SummaryMeta) error {
	m.called++
	m.capturedMeta = meta
	return m.summarizeErr
}

type mockSummarizeQuerier struct {
	listDocs         []sqlc.ListSessionDocumentsByWorkspaceRow
	listErr          error
	capturedParams   sqlc.ListSessionDocumentsByWorkspaceParams
	existingByPath   map[string]sqlc.Document
	getByPathErr     error
}

func (m *mockSummarizeQuerier) ListSessionDocumentsByWorkspace(_ context.Context, arg sqlc.ListSessionDocumentsByWorkspaceParams) ([]sqlc.ListSessionDocumentsByWorkspaceRow, error) {
	m.capturedParams = arg
	return m.listDocs, m.listErr
}

func (m *mockSummarizeQuerier) GetDocumentBySourcePath(_ context.Context, arg sqlc.GetDocumentBySourcePathParams) (sqlc.Document, error) {
	if m.existingByPath != nil {
		if doc, ok := m.existingByPath[arg.SourcePath]; ok {
			return doc, nil
		}
	}
	if m.getByPathErr != nil {
		return sqlc.Document{}, m.getByPathErr
	}
	return sqlc.Document{}, errors.New("not found")
}

func makeSumDoc(sourcePath string, tags []string) sqlc.ListSessionDocumentsByWorkspaceRow {
	return sqlc.ListSessionDocumentsByWorkspaceRow{
		ID:         uuid.New(),
		Title:      "Session: test session",
		SourcePath: sourcePath,
		Tags:       tags,
		Content:    "session content",
		CreatedAt:  time.Now(),
	}
}

func existingDoc(id uuid.UUID) sqlc.Document {
	return sqlc.Document{ID: id}
}

func newSummarizeCtx(t *testing.T, method, body, workspace string) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	var bodyReader *strings.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	} else {
		bodyReader = strings.NewReader("{}")
	}
	req := httptest.NewRequest(method, "/api/v1/summarize", bodyReader)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", workspace)
	return c, rec
}

func TestSummarize_Success(t *testing.T) {
	summaryDocID := uuid.New()
	q := &mockSummarizeQuerier{
		listDocs: []sqlc.ListSessionDocumentsByWorkspaceRow{
			makeSumDoc("opencode://sessions/sess-001", []string{"opencode"}),
			makeSumDoc("opencode://sessions/sess-002", []string{"opencode"}),
			makeSumDoc("opencode://sessions/sess-003", []string{"opencode"}),
		},
		existingByPath: map[string]sqlc.Document{
			"summary://opencode/sess-002": existingDoc(summaryDocID),
			"summary://opencode/sess-003": existingDoc(summaryDocID),
		},
	}
	sum := &mockSummarizeSummarizer{}

	c, rec := newSummarizeCtx(t, http.MethodPost, `{"workspace":"ws1","limit":5}`, "ws1")

	h := handlers.TriggerSummarize(func() handlers.SummarizeSummarizer { return sum }, q, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp handlers.SummarizeResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Summarized != 1 {
		t.Errorf("summarized = %d, want 1", resp.Summarized)
	}
	if resp.Skipped != 2 {
		t.Errorf("skipped = %d, want 2", resp.Skipped)
	}
	if resp.Errors != 0 {
		t.Errorf("errors = %d, want 0", resp.Errors)
	}
}

func TestSummarize_NilSummarizer(t *testing.T) {
	q := &mockSummarizeQuerier{}
	c, _ := newSummarizeCtx(t, http.MethodPost, `{"workspace":"ws1"}`, "ws1")

	h := handlers.TriggerSummarize(func() handlers.SummarizeSummarizer { return nil }, q, zerolog.Nop())
	err := h(c)
	if err == nil {
		t.Fatal("expected error for nil summarizer")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok || he.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %v", err)
	}
	if !strings.Contains(fmt.Sprint(he.Message), "summarization not configured") {
		t.Errorf("expected 'summarization not configured' in message, got %v", he.Message)
	}
}

func TestSummarize_InvalidRequest(t *testing.T) {
	q := &mockSummarizeQuerier{}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/summarize", strings.NewReader("{bad json"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws1")

	h := handlers.TriggerSummarize(func() handlers.SummarizeSummarizer { return &mockSummarizeSummarizer{} }, q, zerolog.Nop())
	err := h(c)
	if err == nil {
		t.Fatal("expected error for bad JSON")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok || he.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %v", err)
	}
}

func TestSummarize_LimitCap(t *testing.T) {
	q := &mockSummarizeQuerier{listDocs: nil}
	sum := &mockSummarizeSummarizer{}

	c, _ := newSummarizeCtx(t, http.MethodPost, `{"workspace":"ws1","limit":100}`, "ws1")

	h := handlers.TriggerSummarize(func() handlers.SummarizeSummarizer { return sum }, q, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if q.capturedParams.Lim != 20 {
		t.Errorf("expected lim=20 (capped), got %d", q.capturedParams.Lim)
	}
}

func TestSummarize_ForceFlag(t *testing.T) {
	summaryDocID := uuid.New()
	q := &mockSummarizeQuerier{
		listDocs: []sqlc.ListSessionDocumentsByWorkspaceRow{
			makeSumDoc("opencode://sessions/sess-001", []string{"opencode"}),
		},
		existingByPath: map[string]sqlc.Document{
			"summary://opencode/sess-001": existingDoc(summaryDocID),
		},
	}
	sum := &mockSummarizeSummarizer{}

	c, rec := newSummarizeCtx(t, http.MethodPost, `{"workspace":"ws1","force":true}`, "ws1")

	h := handlers.TriggerSummarize(func() handlers.SummarizeSummarizer { return sum }, q, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp handlers.SummarizeResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Summarized != 1 {
		t.Errorf("summarized = %d, want 1 (force bypasses pre-check)", resp.Summarized)
	}
	if resp.Skipped != 0 {
		t.Errorf("skipped = %d, want 0 (force=true)", resp.Skipped)
	}
	if sum.called != 1 {
		t.Errorf("summarizer called %d times, want 1", sum.called)
	}
}

func TestSummarize_SourceFilter_Claude(t *testing.T) {
	q := &mockSummarizeQuerier{listDocs: nil}
	sum := &mockSummarizeSummarizer{}

	c, _ := newSummarizeCtx(t, http.MethodPost, `{"workspace":"ws1","source":"claude"}`, "ws1")

	h := handlers.TriggerSummarize(func() handlers.SummarizeSummarizer { return sum }, q, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if q.capturedParams.TagFilter != "claude_code" {
		t.Errorf("tag_filter = %q, want %q", q.capturedParams.TagFilter, "claude_code")
	}
}

func TestSummarize_SummarizeError(t *testing.T) {
	q := &mockSummarizeQuerier{
		listDocs: []sqlc.ListSessionDocumentsByWorkspaceRow{
			makeSumDoc("opencode://sessions/sess-001", []string{"opencode"}),
		},
		getByPathErr: errors.New("not found"),
	}
	sum := &mockSummarizeSummarizer{summarizeErr: errors.New("LLM error")}

	c, rec := newSummarizeCtx(t, http.MethodPost, `{"workspace":"ws1"}`, "ws1")

	h := handlers.TriggerSummarize(func() handlers.SummarizeSummarizer { return sum }, q, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp handlers.SummarizeResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Summarized != 0 {
		t.Errorf("summarized = %d, want 0", resp.Summarized)
	}
	if resp.Skipped != 0 {
		t.Errorf("skipped = %d, want 0", resp.Skipped)
	}
	if resp.Errors != 1 {
		t.Errorf("errors = %d, want 1", resp.Errors)
	}
}
