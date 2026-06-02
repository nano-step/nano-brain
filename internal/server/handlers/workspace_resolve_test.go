package handlers_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type mockResolver struct {
	fn func(ctx context.Context, hash string) (sqlc.Workspace, error)
}

func (m *mockResolver) GetWorkspaceByHash(ctx context.Context, hash string) (sqlc.Workspace, error) {
	return m.fn(ctx, hash)
}

func newResolveRequest(t *testing.T, body string) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/resolve", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func decodeResolveResponse(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v\nbody=%s", err, rec.Body.String())
	}
	return got
}

func TestResolveWorkspace_Registered(t *testing.T) {
	absPath, err := filepath.Abs("/tmp/registered-project")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	wantHash, err := storage.WorkspaceHash(absPath)
	if err != nil {
		t.Fatalf("WorkspaceHash: %v", err)
	}

	q := &mockResolver{
		fn: func(_ context.Context, hash string) (sqlc.Workspace, error) {
			if hash != wantHash {
				t.Fatalf("expected hash %q, got %q", wantHash, hash)
			}
			return sqlc.Workspace{
				ID:        uuid.New(),
				Hash:      wantHash,
				Name:      "custom-display-name",
				Path:      absPath,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}, nil
		},
	}

	c, rec := newResolveRequest(t, `{"path":"/tmp/registered-project"}`)
	if err := handlers.ResolveWorkspace(q, zerolog.Nop())(c); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	got := decodeResolveResponse(t, rec)
	if got["workspace_hash"] != wantHash {
		t.Errorf("workspace_hash: got %v want %s", got["workspace_hash"], wantHash)
	}
	if got["root_path"] != absPath {
		t.Errorf("root_path: got %v want %s", got["root_path"], absPath)
	}
	if got["name"] != "custom-display-name" {
		t.Errorf("name: got %v want %s", got["name"], "custom-display-name")
	}
	if got["registered"] != true {
		t.Errorf("registered: got %v want true", got["registered"])
	}
}

func TestResolveWorkspace_NotRegistered(t *testing.T) {
	absPath, _ := filepath.Abs("/tmp/never-registered")
	wantHash, _ := storage.WorkspaceHash(absPath)

	q := &mockResolver{
		fn: func(_ context.Context, _ string) (sqlc.Workspace, error) {
			return sqlc.Workspace{}, sql.ErrNoRows
		},
	}

	c, rec := newResolveRequest(t, `{"path":"/tmp/never-registered"}`)
	if err := handlers.ResolveWorkspace(q, zerolog.Nop())(c); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	got := decodeResolveResponse(t, rec)
	if got["workspace_hash"] != wantHash {
		t.Errorf("workspace_hash: got %v want %s", got["workspace_hash"], wantHash)
	}
	if got["root_path"] != absPath {
		t.Errorf("root_path: got %v want %s", got["root_path"], absPath)
	}
	if got["name"] != "never-registered" {
		t.Errorf("name: got %v want never-registered (filepath.Base)", got["name"])
	}
	if got["registered"] != false {
		t.Errorf("registered: got %v want false", got["registered"])
	}
}

func TestResolveWorkspace_MissingPathField(t *testing.T) {
	q := &mockResolver{fn: func(_ context.Context, _ string) (sqlc.Workspace, error) {
		t.Fatal("mock should not be called when path is empty")
		return sqlc.Workspace{}, nil
	}}
	cases := []struct {
		name string
		body string
	}{
		{"empty body", `{}`},
		{"empty path string", `{"path":""}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := newResolveRequest(t, tc.body)
			err := handlers.ResolveWorkspace(q, zerolog.Nop())(c)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			he, ok := err.(*echo.HTTPError)
			if !ok {
				t.Fatalf("expected *echo.HTTPError, got %T: %v", err, err)
			}
			if he.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d", he.Code)
			}
		})
	}
}

func TestResolveWorkspace_InvalidJSON(t *testing.T) {
	q := &mockResolver{fn: func(_ context.Context, _ string) (sqlc.Workspace, error) {
		t.Fatal("mock should not be called on invalid JSON")
		return sqlc.Workspace{}, nil
	}}
	c, _ := newResolveRequest(t, `{bad json`)
	err := handlers.ResolveWorkspace(q, zerolog.Nop())(c)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected *echo.HTTPError, got %T: %v", err, err)
	}
	if he.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", he.Code)
	}
}

func TestResolveWorkspace_RelativePathNormalized(t *testing.T) {
	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	wantHash, err := storage.WorkspaceHash(cwd)
	if err != nil {
		t.Fatalf("WorkspaceHash: %v", err)
	}

	q := &mockResolver{
		fn: func(_ context.Context, hash string) (sqlc.Workspace, error) {
			if hash != wantHash {
				t.Errorf("expected normalized hash %q, got %q", wantHash, hash)
			}
			return sqlc.Workspace{}, sql.ErrNoRows
		},
	}

	c, rec := newResolveRequest(t, `{"path":"."}`)
	if err := handlers.ResolveWorkspace(q, zerolog.Nop())(c); err != nil {
		t.Fatalf("handler: %v", err)
	}
	got := decodeResolveResponse(t, rec)
	if got["root_path"] != cwd {
		t.Errorf("root_path: got %v want %s", got["root_path"], cwd)
	}
	if got["workspace_hash"] != wantHash {
		t.Errorf("workspace_hash: got %v want %s", got["workspace_hash"], wantHash)
	}
}

func TestResolveWorkspace_DBError(t *testing.T) {
	q := &mockResolver{
		fn: func(_ context.Context, _ string) (sqlc.Workspace, error) {
			return sqlc.Workspace{}, errors.New("connection refused")
		},
	}
	c, _ := newResolveRequest(t, `{"path":"/tmp/x"}`)
	err := handlers.ResolveWorkspace(q, zerolog.Nop())(c)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected *echo.HTTPError, got %T: %v", err, err)
	}
	if he.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", he.Code)
	}
}

func TestResolveWorkspacePath_DirectCall(t *testing.T) {
	absPath, _ := filepath.Abs("/tmp/direct-call-test")
	wantHash, _ := storage.WorkspaceHash(absPath)
	q := &mockResolver{
		fn: func(_ context.Context, _ string) (sqlc.Workspace, error) {
			return sqlc.Workspace{}, sql.ErrNoRows
		},
	}
	resp, err := handlers.ResolveWorkspacePath(context.Background(), q, "/tmp/direct-call-test", zerolog.Nop())
	if err != nil {
		t.Fatalf("ResolveWorkspacePath: %v", err)
	}
	if resp.WorkspaceHash != wantHash {
		t.Errorf("hash: got %s want %s", resp.WorkspaceHash, wantHash)
	}
	if resp.RootPath != absPath {
		t.Errorf("root_path: got %s want %s", resp.RootPath, absPath)
	}
	if resp.Registered {
		t.Error("expected registered=false")
	}
}
