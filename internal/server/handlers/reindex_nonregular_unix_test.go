//go:build !windows

package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

// fifoTree builds a root with two regular .js files, a FIFO, a symlink to the
// FIFO and a symlink to a regular file. Opening a FIFO for reading blocks
// until a writer appears (issue #675).
func fifoTree(t *testing.T) (root, fifo string) {
	t.Helper()
	root = filepath.Join(t.TempDir(), "ws")
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"app.js", filepath.Join("sub", "lib.js")} {
		if err := os.WriteFile(filepath.Join(root, rel), []byte("function hello() { return 1; }"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fifo = filepath.Join(root, "sub", "pipe.js")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Skipf("mkfifo not available here: %v", err)
	}
	for link, target := range map[string]string{"pipe-alias.js": filepath.Join("sub", "pipe.js"), "alias.js": "app.js"} {
		if err := os.Symlink(target, filepath.Join(root, link)); err != nil {
			t.Fatal(err)
		}
	}
	return root, fifo
}

// serveOrFailOnBlock runs the handler and fails the test, instead of hanging
// it, when the handler is still running after 5s. A blocked reader is released
// by opening the FIFO's write end.
func serveOrFailOnBlock(t *testing.T, fifo string, h echo.HandlerFunc, c echo.Context) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- h(c) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
		return
	case <-time.After(5 * time.Second):
	}
	t.Errorf("handler blocked on a non-regular file")
	for {
		if f, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			f.Close()
		}
		select {
		case <-done:
			t.FailNow()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func TestIncrementalReindex_SkipsNonRegularFiles(t *testing.T) {
	root, fifo := fifoTree(t)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reindex", strings.NewReader(`{"workspace":"ws123"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws123")

	mq := &mockReindexQuerier{
		collections: []sqlc.Collection{{Name: "code", Path: root}},
		indexedDocs: map[string][]sqlc.ListDocumentSourcePathsAndHashesRow{"code": {}},
	}
	serveOrFailOnBlock(t, fifo, handlers.TriggerReindex(mq, newTestWatcherForHandler(), nil, nil, zerolog.Nop()), c)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	// app.js, sub/lib.js and alias.js; not the FIFO, not the link to it.
	if resp["embedded"] != float64(3) {
		t.Errorf("embedded = %v, want 3 (the regular files and the alias of one)", resp["embedded"])
	}
}

// With an empty registry every file that IS read costs exactly one
// DeleteFunctionFlowchartsByFile, so the delete count is the number of files
// read.
func TestReindexCFG_SkipsNonRegularFiles(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"incremental", `{"workspace":"ws123"}`},
		{"full walk", `{"workspace":"ws123","full":true}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, fifo := fifoTree(t)
			mq := &mockReindexCFGQuerier{collections: []sqlc.Collection{{Name: "code", Path: root}}}
			for _, rel := range []string{"app.js", filepath.Join("sub", "lib.js"), filepath.Join("sub", "pipe.js"), "pipe-alias.js", "alias.js"} {
				mq.documents = append(mq.documents, sqlc.ListDocumentsByWorkspaceRow{Collection: "code", SourcePath: filepath.Join(root, rel)})
			}

			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/reindex-cfg", strings.NewReader(tc.body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.Set("workspace", "ws123")

			serveOrFailOnBlock(t, fifo, handlers.ReindexCFG(mq, graph.NewRegistry(), zerolog.Nop()), c)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
			}
			if mq.deleteCount != 3 {
				t.Errorf("files read = %d, want 3 (app.js, sub/lib.js and alias.js)", mq.deleteCount)
			}
		})
	}
}
