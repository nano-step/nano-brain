package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func csrfSetup(boundAddr string) (echo.MiddlewareFunc, echo.HandlerFunc) {
	mw := CSRF(boundAddr)
	handler := func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}
	return mw, handler
}

func TestCSRF_7StepMatrix(t *testing.T) {
	mw, handler := csrfSetup("localhost:3100")

	tests := []struct {
		name       string
		method     string
		xReqWith   string
		origin     string
		referer    string
		wantStatus int
	}{
		{"GET bypasses CSRF", http.MethodGet, "", "", "", http.StatusOK},
		{"Rule1: X-Requested-With header allows", http.MethodPost, "nano-brain-ui", "", "", http.StatusOK},
		{"Rule1: dashboard header allows", http.MethodPost, "nano-brain-dashboard", "", "", http.StatusOK},
		{"Rule1: unknown X-Requested-With falls through", http.MethodPost, "evil", "http://evil.com", "", http.StatusForbidden},
		{"Rule2: no Origin no Referer allows (CLI)", http.MethodPost, "", "", "", http.StatusOK},
		{"Rule3: Origin null rejects", http.MethodPost, "", "null", "", http.StatusForbidden},
		{"Rule4: Origin same host allows", http.MethodPost, "", "http://localhost:3100", "", http.StatusOK},
		{"Rule5: Origin different host rejects", http.MethodPost, "", "http://evil.com", "", http.StatusForbidden},
		{"Rule6: no Origin, Referer same host allows", http.MethodPost, "", "", "http://localhost:3100/page", http.StatusOK},
		{"Rule7: no Origin, Referer different host rejects", http.MethodPost, "", "", "http://evil.com/page", http.StatusForbidden},
		{"PUT also checked", http.MethodPut, "", "http://evil.com", "", http.StatusForbidden},
		{"DELETE also checked", http.MethodDelete, "", "http://evil.com", "", http.StatusForbidden},
		{"PATCH also checked", http.MethodPatch, "", "http://evil.com", "", http.StatusForbidden},
		{"PATCH without Origin allows (CLI)", http.MethodPatch, "", "", "", http.StatusOK},
		{"Loopback 127.0.0.1 matches localhost", http.MethodPost, "", "http://127.0.0.1:3100", "", http.StatusOK},
		{"Loopback [::1] matches localhost", http.MethodPost, "", "http://[::1]:3100", "", http.StatusOK},
		{"Loopback different port allows (proxy scenario)", http.MethodPost, "", "http://localhost:9999", "", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(tt.method, "/api/v1/test", nil)
			if tt.xReqWith != "" {
				req.Header.Set("X-Requested-With", tt.xReqWith)
			}
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.referer != "" {
				req.Header.Set("Referer", tt.referer)
			}
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			h := mw(handler)
			err := h(c)

			status := rec.Code
			if err != nil {
				if he, ok := err.(*echo.HTTPError); ok {
					status = he.Code
				}
			}

			if status != tt.wantStatus {
				t.Errorf("got status %d, want %d", status, tt.wantStatus)
			}
		})
	}
}

// TestCSRF_RequestHostMatch covers the Origin/Referer rules accepting the
// request's own Host header in addition to the bound address — the only way a
// same-origin page can match when the daemon is bound to a wildcard address.
func TestCSRF_RequestHostMatch(t *testing.T) {
	tests := []struct {
		name       string
		boundAddr  string
		host       string
		origin     string
		referer    string
		wantStatus int
	}{
		{"wildcard bind, loopback Host, loopback Origin other port", "0.0.0.0:3100", "localhost:3100", "http://localhost:5173", "", http.StatusOK},
		{"wildcard bind, loopback Host, same Origin", "0.0.0.0:3100", "localhost:3100", "http://localhost:3100", "", http.StatusOK},
		{"wildcard bind, public Host, same Origin", "0.0.0.0:3100", "brain.example.com", "https://brain.example.com", "", http.StatusOK},
		{"wildcard bind, container Host, same Origin", "0.0.0.0:3100", "host.docker.internal:3100", "http://host.docker.internal:3100", "", http.StatusOK},
		{"wildcard bind, loopback Host, same Referer", "0.0.0.0:3100", "localhost:3100", "", "http://localhost:3100/page", http.StatusOK},
		{"wildcard bind, public Host, same Referer", "0.0.0.0:3100", "brain.example.com", "", "https://brain.example.com/page", http.StatusOK},
		{"wildcard bind, loopback Host, foreign Origin", "0.0.0.0:3100", "localhost:3100", "http://evil.example", "", http.StatusForbidden},
		{"wildcard bind, loopback Host, foreign Referer", "0.0.0.0:3100", "localhost:3100", "", "http://evil.example/page", http.StatusForbidden},
		{"wildcard bind, public Host, foreign Origin", "0.0.0.0:3100", "brain.example.com", "https://evil.example", "", http.StatusForbidden},
		{"loopback bind, loopback Host, foreign Origin", "localhost:3100", "localhost:3100", "http://evil.example", "", http.StatusForbidden},
		{"null Origin rejects with matching-looking Host", "0.0.0.0:3100", "null", "null", "", http.StatusForbidden},
		{"null Origin rejects on loopback bind", "localhost:3100", "localhost:3100", "null", "", http.StatusForbidden},
		// Port handling is not loosened: outside loopback, host:port must
		// match exactly, as it always has against the bound address.
		{"public Host, same name other port rejects", "0.0.0.0:3100", "brain.example.com:3100", "http://brain.example.com:8080", "", http.StatusForbidden},
		{"empty Host never matches an empty Origin host", "0.0.0.0:3100", "", "http://", "", http.StatusForbidden},
		// Allowed here only because the Host guard runs first and rejects
		// this Host; the full chain is pinned in internal/server
		// (TestRebindingSameOriginRejectedByChain).
		{"foreign Host with same foreign Origin passes CSRF alone", "0.0.0.0:3100", "evil.example", "http://evil.example", "", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw, handler := csrfSetup(tt.boundAddr)
			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/test", nil)
			req.Host = tt.host
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.referer != "" {
				req.Header.Set("Referer", tt.referer)
			}
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			err := mw(handler)(c)

			status := rec.Code
			if err != nil {
				if he, ok := err.(*echo.HTTPError); ok {
					status = he.Code
				}
			}
			if status != tt.wantStatus {
				t.Errorf("bound %q Host %q Origin %q Referer %q: got %d, want %d", tt.boundAddr, tt.host, tt.origin, tt.referer, status, tt.wantStatus)
			}
		})
	}
}
