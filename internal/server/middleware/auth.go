package middleware

import (
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
)

type AuthUser struct {
	Username     string
	PasswordHash string
}

type AuthSnapshot struct {
	Enabled     bool
	Realm       string
	Users       []AuthUser
	Tokens      []string
	BypassPaths []string
}

var zeroToken = make([]byte, 32)

func Auth(cfg AuthSnapshot, logger zerolog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !cfg.Enabled {
				return next(c)
			}

			path := c.Request().URL.Path
			for _, bp := range cfg.BypassPaths {
				if path == bp {
					return next(c)
				}
			}

			header := c.Request().Header.Get("Authorization")
			if header == "" {
				return unauthorized(c, cfg.Realm)
			}

			if strings.HasPrefix(header, "Basic ") {
				if principal, ok := validateBasic(header[6:], cfg.Users); ok {
					c.Set("auth.principal", principal)
					return next(c)
				}
			} else if strings.HasPrefix(header, "Bearer ") {
				if principal, ok := validateBearer(header[7:], cfg.Tokens); ok {
					c.Set("auth.principal", principal)
					return next(c)
				}
			}

			logAuthFailure(c, logger)
			return unauthorized(c, cfg.Realm)
		}
	}
}

func validateBasic(encoded string, users []AuthUser) (string, bool) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", false
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return "", false
	}
	username, password := parts[0], parts[1]

	for _, u := range users {
		if subtle.ConstantTimeCompare([]byte(u.Username), []byte(username)) == 1 {
			if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) == nil {
				return username, true
			}
			return "", false
		}
	}
	return "", false
}

func validateBearer(token string, validTokens []string) (string, bool) {
	tokenBytes := []byte(token)
	matched := false

	if len(validTokens) == 0 {
		subtle.ConstantTimeCompare(tokenBytes, zeroToken)
		return "", false
	}

	for _, vt := range validTokens {
		if subtle.ConstantTimeCompare(tokenBytes, []byte(vt)) == 1 {
			matched = true
		}
	}
	if matched {
		return "token", true
	}
	return "", false
}

func unauthorized(c echo.Context, realm string) error {
	c.Response().Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
	return c.NoContent(http.StatusUnauthorized)
}

func logAuthFailure(c echo.Context, logger zerolog.Logger) {
	ip := c.RealIP()
	ua := c.Request().UserAgent()
	if len(ua) > 80 {
		ua = ua[:80]
	}
	logger.Warn().Str("ip", ip).Str("user_agent", ua).Msg("auth failure")
}

