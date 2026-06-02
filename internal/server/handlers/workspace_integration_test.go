//go:build integration

package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
	"github.com/rs/zerolog"
)

func newQueries(t *testing.T) *sqlc.Queries {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	return sqlc.New(db)
}

func doInit(t *testing.T, q *sqlc.Queries, rootPath string) map[string]interface{} {
	t.Helper()
	e := echo.New()
	body, _ := json.Marshal(map[string]string{"root_path": rootPath})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/init", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := handlers.InitWorkspace(q, nil, nil, config.WatcherConfig{}, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("InitWorkspace handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func TestInitWorkspaceE2E(t *testing.T) {
	q := newQueries(t)
	resp := doInit(t, q, "/tmp/e2e-test-project")

	hash, ok := resp["workspace_hash"].(string)
	if !ok || hash == "" {
		t.Fatalf("expected non-empty workspace_hash, got %v", resp["workspace_hash"])
	}

	expectedHash, err := storage.WorkspaceHash("/tmp/e2e-test-project")
	if err != nil {
		t.Fatalf("WorkspaceHash: %v", err)
	}
	if hash != expectedHash {
		t.Errorf("expected hash %q, got %q", expectedHash, hash)
	}

	ws, err := q.GetWorkspaceByHash(context.Background(), hash)
	if err != nil {
		t.Fatalf("GetWorkspaceByHash: %v", err)
	}
	if ws.Hash != hash {
		t.Errorf("expected hash %q in DB, got %q", hash, ws.Hash)
	}
}

func TestInitWorkspaceIdempotent(t *testing.T) {
	q := newQueries(t)
	resp1 := doInit(t, q, "/tmp/idempotent-project")
	resp2 := doInit(t, q, "/tmp/idempotent-project")

	hash1 := resp1["workspace_hash"].(string)
	hash2 := resp2["workspace_hash"].(string)
	if hash1 != hash2 {
		t.Errorf("expected same hash on second init, got %q and %q", hash1, hash2)
	}

	workspaces, err := q.ListWorkspaces(context.Background())
	if err != nil {
		t.Fatalf("ListWorkspaces: %v", err)
	}

	count := 0
	for _, ws := range workspaces {
		if ws.Hash == hash1 {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 workspace row, found %d", count)
	}
}

func TestListWorkspacesE2E(t *testing.T) {
	q := newQueries(t)
	doInit(t, q, "/tmp/list-test-project")

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := handlers.ListWorkspaces(q, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("ListWorkspaces handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp struct {
		Workspaces []map[string]interface{} `json:"workspaces"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Workspaces) == 0 {
		t.Error("expected at least one workspace in list")
	}

	expectedHash, err := storage.WorkspaceHash("/tmp/list-test-project")
	if err != nil {
		t.Fatalf("WorkspaceHash: %v", err)
	}
	found := false
	for _, item := range resp.Workspaces {
		if item["hash"] == expectedHash {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("workspace with hash %q not found in list", expectedHash)
	}
}
