//go:build integration

package handlers_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
	"github.com/nano-brain/nano-brain/internal/watcher"
	"github.com/rs/zerolog"
)

func TestFreshWorkspaceReindexPersistsDocuments(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(t.TempDir(), "new-repo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "service.ts")
	if err := os.WriteFile(file, []byte("export function getPrice() { return 42 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	queries := sqlc.New(db)
	cfg := config.Config{
		Watcher: config.WatcherConfig{DebounceMs: 10, ReindexInterval: 3600},
		Storage: config.StorageConfig{MaxFileSize: 1 << 20},
	}
	w := watcher.New(db, queries, zerolog.Nop(), cfg)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("watcher stopped: %v", err)
		}
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/init",
		bytes.NewBufferString(`{"root_path":`+strconv.Quote(root)+`}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := handlers.InitWorkspace(queries, db, w, cfg.Watcher, zerolog.Nop())(c); err != nil {
		t.Fatalf("init: %v", err)
	}
	var initResp struct {
		WorkspaceHash string `json:"workspace_hash"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &initResp); err != nil {
		t.Fatal(err)
	}
	if initResp.WorkspaceHash == "" {
		t.Fatalf("init returned no workspace hash: %s", rec.Body.String())
	}

	reindexReq := httptest.NewRequest(http.MethodPost, "/api/v1/reindex", bytes.NewBufferString(`{}`))
	reindexReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	reindexRec := httptest.NewRecorder()
	reindexCtx := e.NewContext(reindexReq, reindexRec)
	reindexCtx.Set("workspace", initResp.WorkspaceHash)
	if err := handlers.TriggerReindex(queries, w, nil, nil, zerolog.Nop())(reindexCtx); err != nil {
		t.Fatalf("reindex: %v", err)
	}
	if reindexRec.Code != http.StatusAccepted {
		t.Fatalf("reindex returned %d: %s", reindexRec.Code, reindexRec.Body.String())
	}

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		doc, err := queries.GetDocumentBySourcePath(context.Background(), sqlc.GetDocumentBySourcePathParams{
			SourcePath: file, WorkspaceHash: initResp.WorkspaceHash,
		})
		if err == nil {
			var chunks int
			if err := db.QueryRow("SELECT count(*) FROM chunks WHERE document_id = $1", doc.ID).Scan(&chunks); err != nil {
				t.Fatal(err)
			}
			if chunks > 0 {
				return
			}
		} else if !errors.Is(err, sql.ErrNoRows) {
			t.Fatal(err)
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("reindex reported %s but document and chunks were never persisted", reindexRec.Body.String())
}
