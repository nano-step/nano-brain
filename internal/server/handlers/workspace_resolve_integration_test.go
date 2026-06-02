//go:build integration

package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage"
	"github.com/rs/zerolog"
)

func doResolve(t *testing.T, h echo.HandlerFunc, path string) (int, map[string]interface{}) {
	t.Helper()
	e := echo.New()
	body, _ := json.Marshal(map[string]string{"path": path})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/resolve", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h(c); err != nil {
		if he, ok := err.(*echo.HTTPError); ok {
			return he.Code, nil
		}
		t.Fatalf("resolve handler: %v", err)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode resolve response: %v", err)
	}
	return rec.Code, resp
}

func TestResolveWorkspaceE2E_RegisteredAfterInit(t *testing.T) {
	q := newQueries(t)
	const projectPath = "/tmp/e2e-resolve-registered"
	doInit(t, q, projectPath)

	resolveHandler := handlers.ResolveWorkspace(q, zerolog.Nop())
	code, resp := doResolve(t, resolveHandler, projectPath)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}

	wantHash, err := storage.WorkspaceHash(projectPath)
	if err != nil {
		t.Fatalf("WorkspaceHash: %v", err)
	}
	if resp["workspace_hash"] != wantHash {
		t.Errorf("workspace_hash: got %v want %s", resp["workspace_hash"], wantHash)
	}
	if resp["registered"] != true {
		t.Errorf("registered: got %v want true", resp["registered"])
	}
	if resp["name"] != filepath.Base(projectPath) {
		t.Errorf("name: got %v want %s", resp["name"], filepath.Base(projectPath))
	}
	if resp["root_path"] != projectPath {
		t.Errorf("root_path: got %v want %s", resp["root_path"], projectPath)
	}

	ctx := context.Background()
	rowsAfter, err := q.ListWorkspacesWithStats(ctx)
	if err != nil {
		t.Fatalf("ListWorkspacesWithStats: %v", err)
	}

	for i := 0; i < 3; i++ {
		_, _ = doResolve(t, resolveHandler, projectPath)
	}
	rowsAfterResolve, err := q.ListWorkspacesWithStats(ctx)
	if err != nil {
		t.Fatalf("ListWorkspacesWithStats: %v", err)
	}
	if len(rowsAfter) != len(rowsAfterResolve) {
		t.Errorf("resolve must be read-only: count changed from %d to %d after 3 resolves", len(rowsAfter), len(rowsAfterResolve))
	}
}

func TestResolveWorkspaceE2E_UnregisteredPath(t *testing.T) {
	q := newQueries(t)
	resolveHandler := handlers.ResolveWorkspace(q, zerolog.Nop())

	const unregistered = "/tmp/e2e-resolve-never-init-xyz"

	ctx := context.Background()
	rowsBefore, err := q.ListWorkspacesWithStats(ctx)
	if err != nil {
		t.Fatalf("ListWorkspacesWithStats baseline: %v", err)
	}

	code, resp := doResolve(t, resolveHandler, unregistered)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}

	wantHash, _ := storage.WorkspaceHash(unregistered)
	if resp["workspace_hash"] != wantHash {
		t.Errorf("workspace_hash: got %v want %s", resp["workspace_hash"], wantHash)
	}
	if resp["registered"] != false {
		t.Errorf("registered: got %v want false", resp["registered"])
	}

	rowsAfter, err := q.ListWorkspacesWithStats(ctx)
	if err != nil {
		t.Fatalf("ListWorkspacesWithStats post-resolve: %v", err)
	}
	if len(rowsBefore) != len(rowsAfter) {
		t.Errorf("resolve must be read-only: count changed from %d to %d", len(rowsBefore), len(rowsAfter))
	}
	for _, r := range rowsAfter {
		if r.Hash == wantHash {
			t.Errorf("resolve should NOT have inserted hash %s", wantHash)
		}
	}
}

func TestResolveWorkspaceE2E_HashDeterministic(t *testing.T) {
	q := newQueries(t)
	resolveHandler := handlers.ResolveWorkspace(q, zerolog.Nop())

	const path = "/tmp/e2e-resolve-determinism"
	_, r1 := doResolve(t, resolveHandler, path)
	_, r2 := doResolve(t, resolveHandler, path)
	if r1["workspace_hash"] != r2["workspace_hash"] {
		t.Errorf("hash should be deterministic across calls: got %v then %v", r1["workspace_hash"], r2["workspace_hash"])
	}
}
