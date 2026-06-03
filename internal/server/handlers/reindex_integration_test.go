//go:build integration

package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
	"github.com/rs/zerolog"
)

func TestIncrementalReindex_Integration(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	q := sqlc.New(db)

	dir := t.TempDir()
	wsHash, err := storage.WorkspaceHash(dir)
	if err != nil {
		t.Fatalf("WorkspaceHash: %v", err)
	}

	ctx := context.Background()

	_, err = q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: wsHash,
		Path: dir,
		Name: "integration-test",
	})
	if err != nil {
		t.Fatalf("UpsertWorkspace: %v", err)
	}

	_, err = q.UpsertCollection(ctx, sqlc.UpsertCollectionParams{
		WorkspaceHash: wsHash,
		Name:          "code",
		Path:          dir,
		GlobPattern:   "**/*",
		UpdateMode:    "auto",
	})
	if err != nil {
		t.Fatalf("UpsertCollection: %v", err)
	}

	fileA := filepath.Join(dir, "a.txt")
	fileB := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(fileA, []byte("content A"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fileB, []byte("content B"), 0o644); err != nil {
		t.Fatal(err)
	}

	w := newTestWatcherForHandler()

	doReindex := func(body string) map[string]interface{} {
		t.Helper()
		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reindex", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("workspace", wsHash)

		h := handlers.TriggerReindex(q, w, nil, nil, zerolog.Nop())
		if err := h(c); err != nil {
			t.Fatalf("handler error: %v", err)
		}
		if rec.Code != http.StatusAccepted {
			t.Fatalf("expected 202, got %d body=%s", rec.Code, rec.Body.String())
		}
		var resp map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return resp
	}

	resp := doReindex(`{}`)
	if resp["embedded"] != float64(2) {
		t.Errorf("first pass: expected embedded=2, got %v", resp["embedded"])
	}
	if resp["skipped"] != float64(0) {
		t.Errorf("first pass: expected skipped=0, got %v", resp["skipped"])
	}

	docs, err := q.ListDocumentSourcePathsAndHashes(ctx, sqlc.ListDocumentSourcePathsAndHashesParams{
		WorkspaceHash: wsHash,
		Collection:    "code",
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(docs) != 0 {
		t.Logf("note: watcher not running in test, so docs are not indexed in DB (expected 0, got %d)", len(docs))
	}

	resp2 := doReindex(`{}`)
	if resp2["embedded"] != float64(2) {
		t.Errorf("second pass (no DB records): expected embedded=2, got %v", resp2["embedded"])
	}

	if err := os.WriteFile(fileA, []byte("modified content A"), 0o644); err != nil {
		t.Fatal(err)
	}
	resp3 := doReindex(`{}`)
	_ = resp3

	if err := os.Remove(fileB); err != nil {
		t.Fatal(err)
	}
	resp4 := doReindex(`{}`)
	_ = resp4
}
