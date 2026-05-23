package server

import (
	"errors"
	"net/http"

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
		}

		s.logger.Error().Err(err).Int("status", code).Msg("request error")

		_ = c.JSON(code, map[string]string{
			"error":   errType,
			"message": msg,
		})
	}
}
