package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestSecurityHeaders(t *testing.T) {
	e := echo.New()
	mw := SecurityHeaders()
	handler := mw(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/ui/index.html", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := handler(c); err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"Content-Security-Policy": "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; frame-ancestors 'none'",
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"Referrer-Policy":         "same-origin",
	}

	for header, expected := range want {
		got := rec.Header().Get(header)
		if got != expected {
			t.Errorf("%s: got %q, want %q", header, got, expected)
		}
	}
}
