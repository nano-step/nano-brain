package middleware

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
)

var testLogger = zerolog.Nop()

func hashPassword(t *testing.T, pw string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(pw), 10)
	if err != nil {
		t.Fatal(err)
	}
	return string(h)
}

func basicHeader(user, pass string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
}

func newTestEcho(cfg AuthSnapshot) (*echo.Echo, *bool) {
	e := echo.New()
	reached := new(bool)
	handler := func(c echo.Context) error {
		*reached = true
		return c.String(http.StatusOK, "ok")
	}
	e.Use(Auth(cfg, testLogger))
	e.GET("/health", handler)
	e.GET("/api/v1/query", handler)
	e.POST("/api/v1/write", handler)
	return e, reached
}

func TestAuth_Disabled_Passthrough(t *testing.T) {
	e, reached := newTestEcho(AuthSnapshot{Enabled: false})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/query", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !*reached {
		t.Error("handler not reached when auth disabled")
	}
}

func TestAuth_BypassHealth(t *testing.T) {
	e, reached := newTestEcho(AuthSnapshot{
		Enabled:     true,
		Realm:       "test",
		BypassPaths: []string{"/health"},
	})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for /health bypass, got %d", rec.Code)
	}
	if !*reached {
		t.Error("handler not reached for bypass path")
	}
}

func TestAuth_NoHeader_401(t *testing.T) {
	e, _ := newTestEcho(AuthSnapshot{
		Enabled: true,
		Realm:   "test",
		Users:   []AuthUser{{Username: "admin", PasswordHash: hashPassword(t, "pass")}},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/query", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
	wwwAuth := rec.Header().Get("WWW-Authenticate")
	if wwwAuth == "" {
		t.Error("missing WWW-Authenticate header")
	}
	if wwwAuth != `Basic realm="test"` {
		t.Errorf("unexpected WWW-Authenticate: %q", wwwAuth)
	}
}

func TestAuth_ValidBasic_200(t *testing.T) {
	e, reached := newTestEcho(AuthSnapshot{
		Enabled: true,
		Realm:   "test",
		Users:   []AuthUser{{Username: "admin", PasswordHash: hashPassword(t, "secret")}},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/query", nil)
	req.Header.Set("Authorization", basicHeader("admin", "secret"))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !*reached {
		t.Error("handler not reached with valid basic auth")
	}
}

func TestAuth_WrongPassword_401(t *testing.T) {
	e, _ := newTestEcho(AuthSnapshot{
		Enabled: true,
		Realm:   "test",
		Users:   []AuthUser{{Username: "admin", PasswordHash: hashPassword(t, "secret")}},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/query", nil)
	req.Header.Set("Authorization", basicHeader("admin", "wrong"))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuth_ValidBearer_200(t *testing.T) {
	token := "nbt_test-token-value-here"
	e, reached := newTestEcho(AuthSnapshot{
		Enabled: true,
		Realm:   "test",
		Tokens:  []string{token},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/query", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !*reached {
		t.Error("handler not reached with valid bearer")
	}
}

func TestAuth_WrongBearer_401(t *testing.T) {
	e, _ := newTestEcho(AuthSnapshot{
		Enabled: true,
		Realm:   "test",
		Tokens:  []string{"nbt_correct-token"},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/query", nil)
	req.Header.Set("Authorization", "Bearer nbt_wrong-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuth_EmptyTokens_BearerAttempt_401(t *testing.T) {
	e, _ := newTestEcho(AuthSnapshot{
		Enabled: true,
		Realm:   "test",
		Users:   []AuthUser{{Username: "admin", PasswordHash: hashPassword(t, "pass")}},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/query", nil)
	req.Header.Set("Authorization", "Bearer some-random-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for bearer with no tokens configured, got %d", rec.Code)
	}
}

func TestAuth_MultipleUsers(t *testing.T) {
	users := []AuthUser{
		{Username: "alice", PasswordHash: hashPassword(t, "alice-pass")},
		{Username: "bob", PasswordHash: hashPassword(t, "bob-pass")},
	}
	e, _ := newTestEcho(AuthSnapshot{
		Enabled: true,
		Realm:   "test",
		Users:   users,
	})

	for _, tc := range []struct {
		user, pass string
		wantCode   int
	}{
		{"alice", "alice-pass", 200},
		{"bob", "bob-pass", 200},
		{"alice", "bob-pass", 401},
		{"charlie", "any", 401},
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/query", nil)
		req.Header.Set("Authorization", basicHeader(tc.user, tc.pass))
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != tc.wantCode {
			t.Errorf("user=%s pass=%s: expected %d, got %d", tc.user, tc.pass, tc.wantCode, rec.Code)
		}
	}
}

