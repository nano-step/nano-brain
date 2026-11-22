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
	hashes  []string
}

func (*retrySummarizer) RunOnce(context.Context, string) (int, int, int, error) {
	panic("manual retry must not call the unfiltered scheduler")
}

func (s *retrySummarizer) RetryContentHashes(_ context.Context, _ string, hashes []string) (int, int, int, error) {
	s.hashes = hashes
	s.queries.retried = true
	return 0, 0, 1, nil
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
				len(s.hashes) != 1 || s.hashes[0] != "symbol-hash" {
				t.Fatalf("retry result=%v, resolved=%v, requested=%v", response, queries.resolved, s.hashes)
			}
		})
	}
}
