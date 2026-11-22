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

// jsonString is an unterminated JSON string of n bytes after the opening
// brace: a body the JSON decoder keeps reading instead of rejecting at once.
func jsonString(n int64) io.Reader {
	return io.MultiReader(strings.NewReader(`{"root_path":"`), io.LimitReader(repeat('a'), n))
}

type repeat byte

func (r repeat) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(r)
	}
	return len(p), nil
}

// No request body may be buffered past the limit, whether or not the client
// declares its length, on routes with and without the workspace middleware
// (#653). REST routes are limited to 4 MiB and answer 413 either way; /mcp
// and /sse keep the 32 MiB the MCP transport allows (#673).
func TestRequestBodyLimit(t *testing.T) {
	const (
		restLimit = 4 << 20
		mcpLimit  = 32 << 20
	)
	s := newGuardTestServer(t, config.AuthConfig{})

	tests := []struct {
		name          string
		path          string
		body          io.Reader
		contentLength int64
		wantTooLarge  bool
	}{
		{"declared length over the limit", "/api/v1/query", strings.NewReader("{}"), restLimit + 1, true},
		{"declared length over the limit, no workspace middleware", "/api/v1/init", strings.NewReader("{}"), restLimit + 1, true},
		{"declared length over the limit, root route", "/api/harvest", strings.NewReader("{}"), restLimit + 1, true},
		{"undeclared length over the limit", "/api/v1/query", io.LimitReader(zeros{}, restLimit+1), -1, true},
		{"undeclared length over the limit, handler bind", "/api/v1/init", jsonString(restLimit + 1), -1, true},
		{"declared length at the limit", "/api/v1/query", strings.NewReader("{}"), restLimit, false},
		{"small body", "/api/v1/query", strings.NewReader(`{"workspace":"abc"}`), -1, false},
		{"mcp declared length over the REST limit", "/mcp", jsonString(restLimit + 1), restLimit + 1 + 14, false},
		{"mcp undeclared length over the REST limit", "/mcp", jsonString(restLimit + 1), -1, false},
		{"sse declared length over the REST limit", "/sse", strings.NewReader("{}"), restLimit + 1, false},
		{"mcp declared length over the MCP limit", "/mcp", strings.NewReader("{}"), mcpLimit + 1, true},
		{"sse declared length over the MCP limit", "/sse", strings.NewReader("{}"), mcpLimit + 1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "http://localhost:3100"+tt.path, tt.body)
			req.ContentLength = tt.contentLength
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")
			rec := httptest.NewRecorder()
			s.ServeHTTP(rec, req)

			if got := rec.Code == http.StatusRequestEntityTooLarge; got != tt.wantTooLarge {
				t.Errorf("status %d, want 413=%v", rec.Code, tt.wantTooLarge)
			}
		})
	}
}
