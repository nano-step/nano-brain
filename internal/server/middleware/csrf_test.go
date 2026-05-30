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
		{"Rule2: no Origin no Referer allows (CLI)", http.MethodPost, "", "", "", http.StatusOK},
		{"Rule3: Origin null rejects", http.MethodPost, "", "null", "", http.StatusForbidden},
		{"Rule4: Origin same host allows", http.MethodPost, "", "http://localhost:3100", "", http.StatusOK},
		{"Rule5: Origin different host rejects", http.MethodPost, "", "http://evil.com", "", http.StatusForbidden},
		{"Rule6: no Origin, Referer same host allows", http.MethodPost, "", "", "http://localhost:3100/page", http.StatusOK},
		{"Rule7: no Origin, Referer different host rejects", http.MethodPost, "", "", "http://evil.com/page", http.StatusForbidden},
		{"PUT also checked", http.MethodPut, "", "http://evil.com", "", http.StatusForbidden},
		{"DELETE also checked", http.MethodDelete, "", "http://evil.com", "", http.StatusForbidden},
		{"Loopback 127.0.0.1 matches localhost", http.MethodPost, "", "http://127.0.0.1:3100", "", http.StatusOK},
		{"Loopback [::1] matches localhost", http.MethodPost, "", "http://[::1]:3100", "", http.StatusOK},
		{"Wrong port rejects", http.MethodPost, "", "http://localhost:9999", "", http.StatusForbidden},
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
