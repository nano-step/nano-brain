package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/labstack/echo/v4"
)

func noopMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return next
	}
}

func securityMW() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("X-Content-Type-Options", "nosniff")
			c.Response().Header().Set("X-Frame-Options", "DENY")
			return next(c)
		}
	}
}

func testFSWithIndex() fstest.MapFS {
	return fstest.MapFS{
		"dist/index.html":              {Data: []byte(`<!DOCTYPE html><html><body>hello SPA</body></html>`)},
		"dist/assets/main-abc123.js":   {Data: []byte(`console.log("app");`)},
		"dist/assets/style-def456.css": {Data: []byte(`body{margin:0}`)},
	}
}

func emptyDistFS() fstest.MapFS {
	return fstest.MapFS{
		"dist/.gitkeep": {Data: []byte{}},
	}
}

func TestUIServesIndex(t *testing.T) {
	e := echo.New()
	RegisterUIRoutes(e, testFSWithIndex(), noopMiddleware())

	req := httptest.NewRequest(http.MethodGet, "/ui", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /ui status = %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("GET /ui Content-Type = %q, want text/html", ct)
	}
	if !strings.Contains(rec.Body.String(), "hello SPA") {
		t.Fatalf("GET /ui body missing expected content")
	}
	cc := rec.Header().Get("Cache-Control")
	if cc != "no-cache" {
		t.Fatalf("GET /ui Cache-Control = %q, want no-cache", cc)
	}
}

func TestUISPAFallback(t *testing.T) {
	e := echo.New()
	RegisterUIRoutes(e, testFSWithIndex(), noopMiddleware())

	req := httptest.NewRequest(http.MethodGet, "/ui/memory/abc-123", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /ui/memory/abc-123 status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "hello SPA") {
		t.Fatalf("SPA fallback should return index.html content")
	}
}

func TestUIHashedAssetCache(t *testing.T) {
	e := echo.New()
	RegisterUIRoutes(e, testFSWithIndex(), noopMiddleware())

	req := httptest.NewRequest(http.MethodGet, "/ui/assets/main-abc123.js", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /ui/assets/main-abc123.js status = %d, want 200", rec.Code)
	}
	cc := rec.Header().Get("Cache-Control")
	if !strings.Contains(cc, "immutable") {
		t.Fatalf("hashed asset Cache-Control = %q, want immutable", cc)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "application/javascript" {
		t.Fatalf("hashed asset Content-Type = %q, want application/javascript", ct)
	}
}

func TestUIMissingFallback(t *testing.T) {
	e := echo.New()
	RegisterUIRoutes(e, emptyDistFS(), noopMiddleware())

	req := httptest.NewRequest(http.MethodGet, "/ui", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /ui (missing) status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Web UI not built") {
		t.Fatalf("missing fallback body should contain 'Web UI not built'")
	}
}

func TestUIMissingFallbackSubpath(t *testing.T) {
	e := echo.New()
	RegisterUIRoutes(e, emptyDistFS(), noopMiddleware())

	req := httptest.NewRequest(http.MethodGet, "/ui/anything", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /ui/anything (missing) status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Web UI not built") {
		t.Fatalf("missing fallback body should contain 'Web UI not built'")
	}
}

func TestUISecurityHeadersApplied(t *testing.T) {
	e := echo.New()
	RegisterUIRoutes(e, emptyDistFS(), securityMW())

	req := httptest.NewRequest(http.MethodGet, "/ui", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("security header X-Content-Type-Options missing on /ui")
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("security header X-Frame-Options missing on /ui")
	}
}

func TestUISecurityHeadersNotOnAPI(t *testing.T) {
	e := echo.New()
	RegisterUIRoutes(e, emptyDistFS(), securityMW())
	e.GET("/api/v1/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Header().Get("X-Content-Type-Options") != "" {
		t.Fatal("security headers should NOT be on /api/v1/* routes")
	}
}
