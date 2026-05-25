package server

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
)

const (
	versionHeader   = "X-Nano-Brain-Version"
	requestIDHeader = "X-Request-ID"
)

func registerMiddleware(s *Server) {
	s.echo.Use(requestLoggingMiddleware(s.logger))
	s.echo.Use(versionHeaderMiddleware(s.version))
	s.echo.HTTPErrorHandler = httpErrorHandler(s)
}

// generateShortID returns an 8-character hex request ID derived from 4 random
// bytes. Falls back to a timestamp-based string if crypto/rand fails.
func generateShortID() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UTC().Format("150405.000000")
	}
	return hex.EncodeToString(b[:])
}

// requestLoggingMiddleware attaches a per-request zerolog.Logger to the Echo
// context, generates or propagates X-Request-ID, and emits start/completion
// log entries. Stored under context key "logger" for handlers to retrieve via
// handlers.LoggerFromCtx.
func requestLoggingMiddleware(logger zerolog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()

			reqID := req.Header.Get(requestIDHeader)
			if reqID == "" {
				reqID = generateShortID()
			}
			c.Response().Header().Set(requestIDHeader, reqID)

			reqLogger := logger.With().
				Str("request_id", reqID).
				Str("method", req.Method).
				Str("path", req.URL.Path).
				Logger()
			c.Set("logger", reqLogger)
			c.Set("request_id", reqID)

			reqLogger.Debug().Msg("request started")

			start := time.Now()
			err := next(c)
			latency := time.Since(start)

			status := c.Response().Status
			if err != nil {
				var he *echo.HTTPError
				if errors.As(err, &he) {
					status = he.Code
				} else if status < 400 {
					status = http.StatusInternalServerError
				}
			}

			evt := reqLogger.Info()
			if status >= 500 {
				evt = reqLogger.Error()
			}
			evt.Int("status", status).
				Int64("latency_ms", latency.Milliseconds()).
				Msg("request completed")

			return err
		}
	}
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
