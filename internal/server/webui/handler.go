package webui

import (
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
)

// RegisterUIRoutes mounts the SPA at /ui and /ui/* with SPA fallback.
// securityMW is applied to all /ui responses. If dist/index.html is
// missing in distFS, a fallback instructional page is served.
func RegisterUIRoutes(e *echo.Echo, distFS fs.FS, securityMW echo.MiddlewareFunc) {
	ui := e.Group("/ui", securityMW)

	if !hasIndex(distFS) {
		ui.GET("", serveMissingUI)
		ui.GET("/*", serveMissingUI)
		return
	}

	sub, _ := fs.Sub(distFS, "dist")
	httpFS := http.FS(sub)

	ui.GET("", serveIndex(httpFS))
	ui.GET("/*", spaFallback(httpFS))
}

func hasIndex(distFS fs.FS) bool {
	f, err := distFS.Open("dist/index.html")
	if err != nil {
		return false
	}
	f.Close()
	return true
}

func serveIndex(fsys http.FileSystem) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "no-cache")
		return serveFile(c, fsys, "index.html", "text/html; charset=utf-8")
	}
}

func spaFallback(fsys http.FileSystem) echo.HandlerFunc {
	return func(c echo.Context) error {
		p := strings.TrimPrefix(c.Request().URL.Path, "/ui/")

		// Exclude standalone routes that have their own handlers
		if p == "flows" {
			return echo.NewHTTPError(http.StatusNotFound)
		}

		f, err := fsys.Open(p)
		if err == nil {
			f.Close()
			if isHashedAsset(p) {
				c.Response().Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			return serveFile(c, fsys, p, mimeFor(p))
		}
		c.Response().Header().Set("Cache-Control", "no-cache")
		return serveFile(c, fsys, "index.html", "text/html; charset=utf-8")
	}
}

func serveFile(c echo.Context, fsys http.FileSystem, name, ct string) error {
	f, err := fsys.Open(name)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	defer f.Close()
	c.Response().Header().Set("Content-Type", ct)
	c.Response().WriteHeader(http.StatusOK)
	_, err = io.Copy(c.Response().Writer, f)
	return err
}

func mimeFor(p string) string {
	switch filepath.Ext(p) {
	case ".html":
		return "text/html; charset=utf-8"
	case ".js", ".mjs":
		return "application/javascript"
	case ".css":
		return "text/css"
	case ".json":
		return "application/json"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".ico":
		return "image/x-icon"
	case ".woff2":
		return "font/woff2"
	case ".woff":
		return "font/woff"
	case ".map":
		return "application/json"
	default:
		return "application/octet-stream"
	}
}

func isHashedAsset(p string) bool {
	if !strings.HasPrefix(p, "assets/") {
		return false
	}
	ext := filepath.Ext(p)
	base := strings.TrimSuffix(filepath.Base(p), ext)
	return strings.ContainsAny(base, "-.")
}

// RegisterFlowDashboardRoute registers GET /ui/flows serving the standalone
// flow dashboard HTML page. The page loads workspace and flow data via JS
// fetches to the REST API, so no server-side workspace context is needed.
func RegisterFlowDashboardRoute(e *echo.Echo, securityMW echo.MiddlewareFunc, logger zerolog.Logger) {
	e.GET("/ui/flows", func(c echo.Context) error {
		c.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
		c.Response().WriteHeader(http.StatusOK)
		_, err := io.WriteString(c.Response().Writer, FlowDashboardHTML)
		return err
	}, securityMW)
}
