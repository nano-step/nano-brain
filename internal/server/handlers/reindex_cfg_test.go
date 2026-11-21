package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type mockReindexCFGQuerier struct {
	collections   []sqlc.Collection
	documents     []sqlc.ListDocumentsByWorkspaceRow
	deleteCount   int
	upsertCount   int
	deleteErr     error
	upsertErr     error
}

func (m *mockReindexCFGQuerier) ListCollections(_ context.Context, _ string) ([]sqlc.Collection, error) {
	if m.collections != nil {
		return m.collections, nil
	}
	return nil, nil
}

func (m *mockReindexCFGQuerier) ListDocumentsByWorkspace(_ context.Context, _ string) ([]sqlc.ListDocumentsByWorkspaceRow, error) {
	if m.documents != nil {
		return m.documents, nil
	}
	return nil, nil
}

func (m *mockReindexCFGQuerier) DeleteFunctionFlowchartsByFile(_ context.Context, _ sqlc.DeleteFunctionFlowchartsByFileParams) error {
	m.deleteCount++
	return m.deleteErr
}

func (m *mockReindexCFGQuerier) DeleteAllFunctionFlowcharts(_ context.Context, _ string) error {
	return nil
}

func (m *mockReindexCFGQuerier) UpsertFunctionFlowchart(_ context.Context, _ sqlc.UpsertFunctionFlowchartParams) error {
	m.upsertCount++
	return m.upsertErr
}

func TestReindexCFG_NoJSTSFiles(t *testing.T) {
	e := echo.New()
	body := `{"workspace":"ws123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reindex-cfg", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws123")

	mq := &mockReindexCFGQuerier{
		collections: []sqlc.Collection{{Name: "code", Path: t.TempDir()}},
		documents: []sqlc.ListDocumentsByWorkspaceRow{
			{Collection: "code", SourcePath: filepath.Join(t.TempDir(), "readme.txt")},
		},
	}
	reg := graph.NewRegistry()

	h := handlers.ReindexCFG(mq, reg, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if mq.deleteCount != 0 {
		t.Errorf("expected 0 deletes, got %d", mq.deleteCount)
	}
	if mq.upsertCount != 0 {
		t.Errorf("expected 0 upserts, got %d", mq.upsertCount)
	}
}

func TestReindexCFG_WithJSFile(t *testing.T) {
	dir := t.TempDir()
	jsFile := filepath.Join(dir, "app.js")
	if err := os.WriteFile(jsFile, []byte("function hello() { return 1; }"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := echo.New()
	body := `{"workspace":"ws123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reindex-cfg", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws123")

	mq := &mockReindexCFGQuerier{
		collections: []sqlc.Collection{{Name: "code", Path: dir}},
		documents: []sqlc.ListDocumentsByWorkspaceRow{
			{Collection: "code", SourcePath: jsFile},
		},
	}
	reg := graph.NewRegistry()

	h := handlers.ReindexCFG(mq, reg, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if mq.deleteCount != 1 {
		t.Errorf("expected 1 delete, got %d", mq.deleteCount)
	}
}

func TestReindexCFG_NilGraphRegistry(t *testing.T) {
	e := echo.New()
	body := `{"workspace":"ws123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reindex-cfg", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws123")

	mq := &mockReindexCFGQuerier{}

	h := handlers.ReindexCFG(mq, nil, zerolog.Nop())
	err := h(c)
	if err == nil {
		t.Fatal("expected error for nil graph registry")
	}
	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected HTTPError, got %T", err)
	}
	if httpErr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", httpErr.Code)
	}
}

// TestReindexCFG_SkipsSymlinksNotResolvingToAdmittedFiles: both the incremental
// (document list) and the full (filesystem walk) paths must not read a symlink
// that leaves the code root or resolves to an excluded file (issue #655). With
// an empty registry every file that IS read costs exactly one
// DeleteFunctionFlowchartsByFile, so the delete count is the number of files
// read.
func TestReindexCFG_SkipsSymlinksNotResolvingToAdmittedFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests need privileges on Windows")
	}

	cases := []struct {
		name string
		body string
	}{
		{"incremental", `{"workspace":"ws123"}`},
		{"full walk", `{"workspace":"ws123","full":true}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "ws")
			outside := filepath.Join(base, "outside")
			for _, dir := range []string{root, outside} {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			write := func(path string) {
				t.Helper()
				if err := os.WriteFile(path, []byte("function hello() { return 1; }"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			write(filepath.Join(root, "app.js"))
			write(filepath.Join(outside, "secret.js"))
			write(filepath.Join(root, "prod.env"))
			links := map[string]string{
				"leak.js":   filepath.Join(outside, "secret.js"), // leaves the root
				"env.js":    "prod.env",                          // excluded target
				"broken.js": "missing.js",                        // unresolvable
				"alias.js":  "app.js",                            // fine
			}
			for name, target := range links {
				if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
					t.Fatal(err)
				}
			}

			mq := &mockReindexCFGQuerier{collections: []sqlc.Collection{{Name: "code", Path: root}}}
			for _, name := range []string{"app.js", "leak.js", "env.js", "broken.js", "alias.js"} {
				mq.documents = append(mq.documents, sqlc.ListDocumentsByWorkspaceRow{Collection: "code", SourcePath: filepath.Join(root, name)})
			}

			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/reindex-cfg", strings.NewReader(tc.body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.Set("workspace", "ws123")

			if err := handlers.ReindexCFG(mq, graph.NewRegistry(), zerolog.Nop())(c); err != nil {
				t.Fatalf("handler error: %v", err)
			}
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
			}
			if mq.deleteCount != 2 {
				t.Errorf("files read = %d, want 2 (app.js and alias.js only)", mq.deleteCount)
			}
		})
	}
}
