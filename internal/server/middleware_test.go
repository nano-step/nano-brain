package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func noopHandler(c echo.Context) error {
	return c.String(http.StatusOK, "ok")
}

func applyMiddleware(mw echo.MiddlewareFunc, req *http.Request) *httptest.ResponseRecorder {
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	_ = mw(noopHandler)(c)
	return rec
}

func TestWorkspaceMiddleware_POST_WithWorkspace(t *testing.T) {
	mw := workspaceMiddleware(nil)
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"workspace":"abc"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var captured string
	handler := mw(func(c echo.Context) error {
		captured = c.Get("workspace").(string)
		return c.String(http.StatusOK, "ok")
	})
	if err := handler(c); err != nil {
		t.Fatal(err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if captured != "abc" {
		t.Errorf("expected workspace=abc, got %q", captured)
	}
}

func TestWorkspaceMiddleware_POST_MissingWorkspace(t *testing.T) {
	mw := workspaceMiddleware(nil)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := applyMiddleware(mw, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "workspace_required" {
		t.Errorf("expected error=workspace_required, got %q", body["error"])
	}
}

func TestWorkspaceMiddleware_GET_WithQueryParam(t *testing.T) {
	mw := workspaceMiddleware(nil)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/?workspace=myws", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var captured string
	handler := mw(func(c echo.Context) error {
		captured = c.Get("workspace").(string)
		return c.String(http.StatusOK, "ok")
	})
	if err := handler(c); err != nil {
		t.Fatal(err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if captured != "myws" {
		t.Errorf("expected workspace=myws, got %q", captured)
	}
}

func TestWorkspaceMiddleware_GET_MissingQueryParam(t *testing.T) {
	mw := workspaceMiddleware(nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := applyMiddleware(mw, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "workspace_required" {
		t.Errorf("expected error=workspace_required, got %q", body["error"])
	}
}

func TestWorkspaceMiddleware_AllValue(t *testing.T) {
	mw := workspaceMiddleware(nil)
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"workspace":"all"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var captured string
	handler := mw(func(c echo.Context) error {
		captured = c.Get("workspace").(string)
		return c.String(http.StatusOK, "ok")
	})
	if err := handler(c); err != nil {
		t.Fatal(err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if captured != "all" {
		t.Errorf("expected workspace=all, got %q", captured)
	}
}

func TestContentType_JSON(t *testing.T) {
	mw := contentTypeMiddleware()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
	req.Header.Set(echo.HeaderContentType, "application/json")
	req.ContentLength = int64(len(`{}`))
	rec := applyMiddleware(mw, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestContentType_PlainText(t *testing.T) {
	mw := contentTypeMiddleware()
	body := "hello"
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, "text/plain")
	req.ContentLength = int64(len(body))
	rec := applyMiddleware(mw, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415, got %d", rec.Code)
	}
	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["error"] != "unsupported_media_type" {
		t.Errorf("expected error=unsupported_media_type, got %q", resp["error"])
	}
}

func TestContentType_GET_NoBody(t *testing.T) {
	mw := contentTypeMiddleware()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := applyMiddleware(mw, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestContentType_POST_NoBody(t *testing.T) {
	mw := contentTypeMiddleware()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.ContentLength = 0
	rec := applyMiddleware(mw, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestVersionHeaderMiddleware(t *testing.T) {
	mw := versionHeaderMiddleware("1.2.3")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := applyMiddleware(mw, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	got := rec.Header().Get("X-Nano-Brain-Version")
	if got != "1.2.3" {
		t.Errorf("expected X-Nano-Brain-Version=1.2.3, got %q", got)
	}
}

func TestVersionHeaderMiddleware_EmptyVersion(t *testing.T) {
	mw := versionHeaderMiddleware("")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := applyMiddleware(mw, req)

	got := rec.Header().Get("X-Nano-Brain-Version")
	if got != "" {
		t.Errorf("expected empty version header, got %q", got)
	}
}
