package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/codesummarize"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type retryFailureQuerier struct {
	before, after []sqlc.GetUnresolvedFailuresRow
	retried       bool
	resolved      bool
}

func (q *retryFailureQuerier) GetUnresolvedFailures(context.Context, string) ([]sqlc.GetUnresolvedFailuresRow, error) {
	if q.retried {
		return q.after, nil
	}
	return q.before, nil
}

func (q *retryFailureQuerier) ResolveFailure(context.Context, uuid.UUID) error {
	q.resolved = true
	return nil
}

func (q *retryFailureQuerier) UpdateCodeSummarizationFailure(context.Context, sqlc.UpdateCodeSummarizationFailureParams) error {
	return nil
}

type retrySummarizer struct {
	queries *retryFailureQuerier
	ids     []uuid.UUID
	err     error
}

func (*retrySummarizer) RunOnce(context.Context, string) (int, int, int, error) {
	panic("manual retry must not call the unfiltered scheduler")
}

func (s *retrySummarizer) RetryFailures(_ context.Context, _ string, ids []uuid.UUID) (int, int, int, error) {
	s.ids = ids
	s.queries.retried = true
	return 0, 0, 1, s.err
}

func TestRetryCodeSummarize_BudgetExhaustionPreservesPartialSuccess(t *testing.T) {
	first := sqlc.GetUnresolvedFailuresRow{ID: uuid.New(), WorkspaceHash: "ws", ContentHash: "symbol-A"}
	second := sqlc.GetUnresolvedFailuresRow{ID: uuid.New(), WorkspaceHash: "ws", ContentHash: "symbol-B"}
	for _, tc := range []struct {
		name, path, body string
		all              bool
	}{
		{"selected", "/api/v1/code/summarize/retry",
			`{"failure_ids":["` + first.ID.String() + `","` + second.ID.String() + `"]}`, false},
		{"all", "/api/v1/code/summarize/retry-all", `{}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			queries := &retryFailureQuerier{
				before: []sqlc.GetUnresolvedFailuresRow{first, second},
				after:  []sqlc.GetUnresolvedFailuresRow{second},
			}
			s := &retrySummarizer{queries: queries, err: codesummarize.ErrBudgetExhausted}
			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.Set("workspace", "ws")
			get := func() handlers.CodeSummarizer { return s }
			cfg := config.CodeSummarizationConfig{Enabled: true}
			var h echo.HandlerFunc
			if tc.all {
				h = handlers.RetryAllCodeSummarize(get, queries, cfg, zerolog.Nop())
			} else {
				h = handlers.RetryCodeSummarize(get, queries, cfg, zerolog.Nop())
			}
			if err := h(c); err != nil {
				t.Fatal(err)
			}
			var response map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if rec.Code != http.StatusOK || response["succeeded"] != float64(1) ||
				response["failed"] != float64(1) || !strings.Contains(response["message"].(string), "budget") {
				t.Fatalf("partial budget retry result: %v", response)
			}
		})
	}
}

func TestRetryCodeSummarize_OnlyReportsResolvedFailuresAsSucceeded(t *testing.T) {
	id := uuid.New()
	for _, tc := range []struct {
		name      string
		remaining bool
		wantOK    float64
	}{
		{"failed retry", true, 0},
		{"successful retry", false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			failure := sqlc.GetUnresolvedFailuresRow{ID: id, WorkspaceHash: "ws", ContentHash: "symbol-hash"}
			queries := &retryFailureQuerier{before: []sqlc.GetUnresolvedFailuresRow{failure}}
			if tc.remaining {
				queries.after = []sqlc.GetUnresolvedFailuresRow{failure}
			}
			s := &retrySummarizer{queries: queries}
			e := echo.New()
			body := `{"failure_ids":["` + id.String() + `"]}`
			req := httptest.NewRequest(http.MethodPost, "/api/v1/code/summarize/retry", strings.NewReader(body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.Set("workspace", "ws")
			h := handlers.RetryCodeSummarize(func() handlers.CodeSummarizer { return s }, queries,
				config.CodeSummarizationConfig{Enabled: true}, zerolog.Nop())
			if err := h(c); err != nil {
				t.Fatal(err)
			}
			var response map[string]float64
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if rec.Code != http.StatusOK || response["succeeded"] != tc.wantOK ||
				response["failed"] != 1-tc.wantOK || queries.resolved ||
				len(s.ids) != 1 || s.ids[0] != id {
				t.Fatalf("retry result=%v, resolved=%v, requested=%v", response, queries.resolved, s.ids)
			}
		})
	}
}
