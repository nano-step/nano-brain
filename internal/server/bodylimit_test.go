package server_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nano-brain/nano-brain/internal/config"
)

// zeros is an endless body, so an oversized request needs no large buffer.
type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

// No request body may be buffered past the limit, whether or not the client
// declares its length, on routes with and without the workspace middleware
// (#653).
func TestRequestBodyLimit(t *testing.T) {
	const limit = 32 << 20
	s := newGuardTestServer(t, config.AuthConfig{})

	tests := []struct {
		name          string
		path          string
		body          io.Reader
		contentLength int64
		wantTooLarge  bool
	}{
		{"declared length over the limit", "/api/v1/query", strings.NewReader("{}"), limit + 1, true},
		{"declared length over the limit, no workspace middleware", "/api/v1/init", strings.NewReader("{}"), limit + 1, true},
		{"undeclared length over the limit", "/api/v1/query", io.LimitReader(zeros{}, limit+1), -1, true},
		{"declared length at the limit", "/api/v1/query", strings.NewReader("{}"), limit, false},
		{"small body", "/api/v1/query", strings.NewReader(`{"workspace":"abc"}`), -1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "http://localhost:3100"+tt.path, tt.body)
			req.ContentLength = tt.contentLength
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			s.ServeHTTP(rec, req)

			if got := rec.Code == http.StatusRequestEntityTooLarge; got != tt.wantTooLarge {
				t.Errorf("status %d, want 413=%v", rec.Code, tt.wantTooLarge)
			}
		})
	}
}
