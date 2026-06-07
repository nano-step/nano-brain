package webui

import (
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestRealEmbedFS(t *testing.T) {
	// Walk embedded FS
	t.Log("=== Walking embedded FS ===")
	count := 0
	_ = fs.WalkDir(EmbedFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			info, _ := d.Info()
			t.Logf("  %s (%d bytes)", path, info.Size())
			count++
		}
		return nil
	})
	t.Logf("Total files: %d", count)

	// Try opening via http.FS
	sub, err := fs.Sub(EmbedFS, "dist")
	if err != nil {
		t.Fatalf("fs.Sub failed: %v", err)
	}
	httpFS := http.FS(sub)

	testPaths := []string{
		"assets/index-CmYQJD18.js",
		"assets/react-9mTO3gfj.js",
		"assets/router-UhuWy72c.js",
		"assets/index-0b9C5XlC.css",
	}
	for _, p := range testPaths {
		f, err := httpFS.Open(p)
		if err != nil {
			t.Logf("  Open(%s) FAILED: %v", p, err)
		} else {
			f.Close()
			t.Logf("  Open(%s) OK", p)
		}
	}

	// Simulate full request cycle
	t.Log("=== Simulating spaFallback via Echo ===")
	e := echo.New()
	RegisterUIRoutes(e, EmbedFS, func(next echo.HandlerFunc) echo.HandlerFunc { return next })

	reqPaths := []string{
		"/ui/assets/index-CmYQJD18.js",
		"/ui/assets/react-9mTO3gfj.js",
		"/ui/assets/router-UhuWy72c.js",
		"/ui/assets/index-0b9C5XlC.css",
		"/ui/assets/sigma-D3QLc3pN.js",
	}
	for _, p := range reqPaths {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		ct := rec.Header().Get("Content-Type")
		body := rec.Body.String()
		isHTML := strings.HasPrefix(body, "<!DOCTYPE")
		result := "✅"
		if isHTML && strings.HasSuffix(p, ".js") {
			result = "❌ BUG"
		}
		t.Logf("  %s %s → %d | %s | isHTML=%v", result, p, rec.Code, ct, isHTML)
	}

	// Direct TrimPrefix test
	t.Log("=== Path resolution debug ===")
	for _, fullPath := range reqPaths {
		p := strings.TrimPrefix(fullPath, "/ui/")
		t.Logf("  TrimPrefix(%q, '/ui/') = %q", fullPath, p)
		f, err := httpFS.Open(p)
		if err != nil {
			t.Logf("    httpFS.Open FAILED: %v", err)
		} else {
			stat, _ := f.Stat()
			t.Logf("    httpFS.Open OK (size=%d, isDir=%v)", stat.Size(), stat.IsDir())
			f.Close()
		}
	}

	// Check what c.Request().URL.Path actually contains
	t.Log("=== Echo path param debug ===")
	e2 := echo.New()
	var capturedPaths []string
	e2.GET("/ui/*", func(c echo.Context) error {
		urlPath := c.Request().URL.Path
		param := c.Param("*")
		capturedPaths = append(capturedPaths, fmt.Sprintf("URL.Path=%s Param(*)=%s", urlPath, param))
		return c.String(200, "ok")
	})
	for _, p := range reqPaths {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		rec := httptest.NewRecorder()
		e2.ServeHTTP(rec, req)
	}
	for _, cp := range capturedPaths {
		t.Logf("  %s", cp)
	}
}
