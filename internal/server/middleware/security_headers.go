package middleware

import "github.com/labstack/echo/v4"

// SecurityHeaders returns Echo middleware that sets CSP, nosniff,
// frame-deny, and referrer-policy headers. Intended for the /ui group only.
func SecurityHeaders() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			h := c.Response().Header()
			h.Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; frame-ancestors 'none'")
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "same-origin")
			return next(c)
		}
	}
}
