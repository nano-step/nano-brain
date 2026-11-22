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
// by opening the FIFO's write end; fifo may be empty when the blocking target
// cannot be released (e.g. /dev/zero), in which case the wait is bounded.
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
	deadline := time.After(3 * time.Second)
	for {
		if fifo != "" {
			if f, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
				f.Close()
			}
		}
		select {
		case <-done:
			t.FailNow()
		case <-deadline:
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
	// Embedding is queued asynchronously now, so the changed files are reported
	// as files_pending_ingest and embedded stays 0.
	if resp["files_pending_ingest"] != float64(3) || resp["embedded"] != float64(0) {
		t.Errorf("files_pending_ingest = %v, embedded = %v; want 3 and 0 (the regular files and the alias of one)", resp["files_pending_ingest"], resp["embedded"])
	}
}

// The full walk compiles nested .gitignore/.nano-brainignore files on the way
// down. A FIFO there blocks the walk on open; a symlink to /dev/zero — which
// git can create from a cloned repo — reads without bound (#675).
func TestReindexCFG_FullWalk_SkipsNonRegularIgnoreFiles(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, sub string) (fifo string)
	}{
		{"fifo .gitignore", func(t *testing.T, sub string) string {
			fifo := filepath.Join(sub, ".gitignore")
			if err := syscall.Mkfifo(fifo, 0o644); err != nil {
				t.Skipf("mkfifo not available here: %v", err)
			}
			return fifo
		}},
		{"fifo .nano-brainignore", func(t *testing.T, sub string) string {
			fifo := filepath.Join(sub, ".nano-brainignore")
			if err := syscall.Mkfifo(fifo, 0o644); err != nil {
				t.Skipf("mkfifo not available here: %v", err)
			}
			return fifo
		}},
		{".gitignore -> /dev/zero", func(t *testing.T, sub string) string {
			if err := os.Symlink("/dev/zero", filepath.Join(sub, ".gitignore")); err != nil {
				t.Fatal(err)
			}
			return ""
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := fifoTree(t)
			fifo := tc.setup(t, filepath.Join(root, "sub"))

			mq := &mockReindexCFGQuerier{collections: []sqlc.Collection{{Name: "code", Path: root}}}

			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/reindex-cfg", strings.NewReader(`{"workspace":"ws123","full":true}`))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.Set("workspace", "ws123")

			serveOrFailOnBlock(t, fifo, handlers.ReindexCFG(mq, graph.NewRegistry(), zerolog.Nop()), c)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
			}
		})
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
