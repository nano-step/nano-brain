package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

const versionHeader = "X-Nano-Brain-Version"

func registerMiddleware(s *Server) {
	s.echo.Use(versionHeaderMiddleware(s.version))
	s.echo.HTTPErrorHandler = httpErrorHandler(s)
}

func versionHeaderMiddleware(version string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set(versionHeader, version)
			return next(c)
		}
	}
}

func workspaceMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			var workspace string

			method := c.Request().Method
			if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch {
				body, err := io.ReadAll(c.Request().Body)
				if err != nil {
					return echo.NewHTTPError(http.StatusBadRequest, "failed to read request body")
				}
				c.Request().Body = io.NopCloser(bytes.NewReader(body))

				var req struct {
					Workspace string `json:"workspace"`
				}
				_ = json.Unmarshal(body, &req)
				workspace = req.Workspace
			} else {
				workspace = c.QueryParam("workspace")
			}

			if workspace == "" {
				return c.JSON(http.StatusBadRequest, map[string]string{
					"error":   "workspace_required",
					"message": "A workspace identifier is required. Pass workspace in request body (POST) or query string (GET). Use 'all' for cross-workspace queries.",
				})
			}

			c.Set("workspace", workspace)
			return next(c)
		}
	}
}

func contentTypeMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			method := c.Request().Method
			if method != http.MethodPost && method != http.MethodPut && method != http.MethodPatch {
				return next(c)
			}

			r := c.Request()
			hasBody := r.ContentLength > 0 ||
				strings.EqualFold(r.Header.Get("Transfer-Encoding"), "chunked")
			if !hasBody {
				return next(c)
			}

			ct := r.Header.Get(echo.HeaderContentType)
			if !strings.HasPrefix(ct, "application/json") {
				return c.JSON(http.StatusUnsupportedMediaType, map[string]string{
					"error":   "unsupported_media_type",
					"message": ErrUnsupportedMediaType.Error(),
				})
			}

			return next(c)
		}
	}
}

func httpErrorHandler(s *Server) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}

		code := http.StatusInternalServerError
		errType := "internal_error"
		msg := "an internal error occurred"

		var he *echo.HTTPError
		switch {
		case errors.As(err, &he):
			code = he.Code
			msg = "request error"
			if m, ok := he.Message.(string); ok {
				msg = m
			}
			errType = "http_error"
		case errors.Is(err, ErrWorkspaceRequired):
			code = http.StatusBadRequest
			errType = "workspace_required"
			msg = err.Error()
		case errors.Is(err, ErrUnsupportedMediaType):
			code = http.StatusUnsupportedMediaType
			errType = "unsupported_media_type"
			msg = err.Error()
		}

		s.logger.Error().Err(err).Int("status", code).Msg("request error")

		_ = c.JSON(code, map[string]string{
			"error":   errType,
			"message": msg,
		})
	}
}
