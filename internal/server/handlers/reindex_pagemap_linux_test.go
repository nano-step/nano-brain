//go:build linux

package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

// /proc/self/pagemap is a regular file that reports size 0 yet reads for
// hundreds of GiB. A committed ignore-file or content-file symlink to it
// must not stall a reindex (#675). Linux-only: there is no equivalent file
// on macOS.
func pagemapAvailable(t *testing.T) string {
	t.Helper()
	const pagemap = "/proc/self/pagemap"
	info, err := os.Stat(pagemap)
	if err != nil || !info.Mode().IsRegular() {
		t.Skipf("%s unavailable: %v", pagemap, err)
	}
	return pagemap
}

func TestReindexCFG_FullWalk_PagemapIgnoreFiles(t *testing.T) {
	pagemap := pagemapAvailable(t)
	for _, rel := range []string{".gitignore", ".nano-brainignore", filepath.Join("sub", ".gitignore"), filepath.Join("sub", ".nano-brainignore")} {
		t.Run(rel, func(t *testing.T) {
			root, _ := fifoTree(t)
			if err := os.Symlink(pagemap, filepath.Join(root, rel)); err != nil {
				t.Fatal(err)
			}

			mq := &mockReindexCFGQuerier{collections: []sqlc.Collection{{Name: "code", Path: root}}}

			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/reindex-cfg", strings.NewReader(`{"workspace":"ws123","full":true}`))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.Set("workspace", "ws123")

			serveOrFailOnBlock(t, "", handlers.ReindexCFG(mq, graph.NewRegistry(), zerolog.Nop()), c)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestIncrementalReindex_PagemapFile(t *testing.T) {
	pagemap := pagemapAvailable(t)
	root, _ := fifoTree(t)
	if err := os.Symlink(pagemap, filepath.Join(root, "map.md")); err != nil {
		t.Fatal(err)
	}

	mq := &mockReindexQuerier{
		collections: []sqlc.Collection{{Name: "code", Path: root}},
		indexedDocs: map[string][]sqlc.ListDocumentSourcePathsAndHashesRow{"code": {}},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reindex", strings.NewReader(`{"workspace":"ws123"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws123")

	// Without the bounded hash this call reads pagemap for ~15 minutes.
	serveOrFailOnBlock(t, "", handlers.TriggerReindex(mq, newTestWatcherForHandler(), nil, nil, zerolog.Nop()), c)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", rec.Code, rec.Body.String())
	}
}
