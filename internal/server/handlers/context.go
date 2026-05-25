package handlers

import (
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
)

// LoggerFromCtx returns the per-request zerolog.Logger stored by the
// request-logging middleware. If no logger is present in the context,
// it returns fallback.
func LoggerFromCtx(c echo.Context, fallback zerolog.Logger) zerolog.Logger {
	if l, ok := c.Get("logger").(zerolog.Logger); ok {
		return l
	}
	return fallback
}
