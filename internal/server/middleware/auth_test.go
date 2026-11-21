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

// cheapHash keeps the direct validateBasic tests fast under -race; the cost
// only has to be large next to a code path that skips bcrypt entirely.
func cheapHash(t *testing.T, pw string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return string(h)
}

func TestValidateBasic_Credentials(t *testing.T) {
	users := []AuthUser{{Username: "admin", PasswordHash: cheapHash(t, "secret")}}
	tests := []struct {
		name, user, pass string
		wantOK           bool
	}{
		{"valid credentials", "admin", "secret", true},
		{"wrong password", "admin", "wrong", false},
		{"unknown user", "nobody", "wrong", false},
		{"unknown user with the first user's password", "nobody", "secret", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := base64.StdEncoding.EncodeToString([]byte(tt.user + ":" + tt.pass))
			principal, ok := validateBasic(encoded, users)
			if ok != tt.wantOK {
				t.Fatalf("validateBasic ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && principal != tt.user {
				t.Errorf("principal = %q, want %q", principal, tt.user)
			}
		})
	}
}

// An unknown username must cost one bcrypt comparison, like a wrong password
// does, so response time does not reveal which usernames exist (#271). That
// comparison runs against the most expensive configured hash, so an unknown
// username is as slow as the slowest real user (#654).
func TestValidateBasic_AlwaysOneBcryptComparison(t *testing.T) {
	sameCost := []AuthUser{
		{Username: "admin", PasswordHash: cheapHash(t, "secret")},
		{Username: "second", PasswordHash: cheapHash(t, "other")},
	}
	costly, err := bcrypt.GenerateFromPassword([]byte("costly"), bcrypt.MinCost+1)
	if err != nil {
		t.Fatal(err)
	}
	mixedCost := []AuthUser{
		{Username: "cheap", PasswordHash: cheapHash(t, "secret")},
		{Username: "costly", PasswordHash: string(costly)},
		{Username: "cheap2", PasswordHash: cheapHash(t, "other")},
	}
	tests := []struct {
		name, user, pass string
		users            []AuthUser
		wantHash         string
	}{
		{"valid credentials", "admin", "secret", sameCost, sameCost[0].PasswordHash},
		{"wrong password", "second", "wrong", sameCost, sameCost[1].PasswordHash},
		{"unknown user, equal costs", "nobody", "wrong", sameCost, sameCost[0].PasswordHash},
		{"unknown user, highest cost is not first", "nobody", "wrong", mixedCost, mixedCost[1].PasswordHash},
		{"known cheap user keeps its own hash", "cheap", "secret", mixedCost, mixedCost[0].PasswordHash},
		{"known cheap user after the costly one", "cheap2", "wrong", mixedCost, mixedCost[2].PasswordHash},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var compared []string
			original := compareHash
			compareHash = func(hash, password []byte) error {
				compared = append(compared, string(hash))
				return original(hash, password)
			}
			t.Cleanup(func() { compareHash = original })

			validateBasic(base64.StdEncoding.EncodeToString([]byte(tt.user+":"+tt.pass)), tt.users)

			if len(compared) != 1 || compared[0] != tt.wantHash {
				t.Errorf("bcrypt comparisons = %d %q, want exactly one against %q", len(compared), compared, tt.wantHash)
			}
		})
	}
}
