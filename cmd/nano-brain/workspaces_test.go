package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func setHostPort(t *testing.T, ts *httptest.Server) {
	t.Helper()
	u := strings.TrimPrefix(ts.URL, "http://")
	parts := strings.Split(u, ":")
	if len(parts) >= 1 {
		t.Setenv("NANO_BRAIN_HOST", parts[0])
	}
	if len(parts) >= 2 {
		t.Setenv("NANO_BRAIN_PORT", parts[1])
	}
}

func newWorkspacesServer(t *testing.T, payload interface{}, status int) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(payload)
	}))
	t.Cleanup(ts.Close)
	return ts
}

func TestWorkspacesList_DefaultTable(t *testing.T) {
	payload := map[string]interface{}{"workspaces": []map[string]interface{}{
		{
			"hash":                  "7f44356179aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"root_path":             "/Users/me/projects/nano-brain",
			"name":                  "nano-brain",
			"doc_count":             0,
			"chunk_count":           0,
			"last_document_updated": nil,
		},
		{
			"hash":                  "8a9c1d4f12bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			"root_path":             "/Users/me/projects/my-app",
			"name":                  "my-app",
			"doc_count":             42,
			"chunk_count":           250,
			"last_document_updated": "2026-05-25T10:30:00Z",
		},
	}}
	ts := newWorkspacesServer(t, payload, http.StatusOK)
	setHostPort(t, ts)

	var out, errOut bytes.Buffer
	code := runWorkspacesListWithIO(nil, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, errOut.String())
	}

	got := out.String()
	if !strings.Contains(got, "HASH") || !strings.Contains(got, "NAME") || !strings.Contains(got, "PATH") || !strings.Contains(got, "DOCS") || !strings.Contains(got, "LAST UPDATE") {
		t.Errorf("missing header columns; got:\n%s", got)
	}
	if !strings.Contains(got, "nano-brain") {
		t.Errorf("missing first row; got:\n%s", got)
	}
	if !strings.Contains(got, "my-app") {
		t.Errorf("missing second row; got:\n%s", got)
	}
	if !strings.Contains(got, "7f44356179..") {
		t.Errorf("hash not truncated; got:\n%s", got)
	}
	if !strings.Contains(got, "2026-05-25") {
		t.Errorf("date not formatted; got:\n%s", got)
	}
	if !strings.Contains(got, "never") {
		t.Errorf("null last_document_updated should show 'never'; got:\n%s", got)
	}
}

func TestWorkspacesList_Json(t *testing.T) {
	payload := map[string]interface{}{"workspaces": []map[string]interface{}{
		{
			"hash":      "abc123",
			"root_path": "/p",
			"name":      "x",
			"doc_count": 1,
		},
	}}
	ts := newWorkspacesServer(t, payload, http.StatusOK)
	setHostPort(t, ts)

	var out, errOut bytes.Buffer
	code := runWorkspacesListWithIO([]string{"--json"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, errOut.String())
	}

	body := strings.TrimRight(out.String(), "\n")
	var got struct {
		Workspaces []map[string]interface{} `json:"workspaces"`
	}
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, body)
	}
	if len(got.Workspaces) != 1 || got.Workspaces[0]["hash"] != "abc123" {
		t.Errorf("unexpected JSON body: %s", body)
	}
	if !strings.HasSuffix(out.String(), "\n") {
		t.Errorf("missing trailing newline")
	}
}

func TestWorkspacesList_Empty_Default(t *testing.T) {
	ts := newWorkspacesServer(t, map[string]interface{}{"workspaces": []map[string]interface{}{}}, http.StatusOK)
	setHostPort(t, ts)

	var out, errOut bytes.Buffer
	code := runWorkspacesListWithIO(nil, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if out.Len() != 0 {
		t.Errorf("stdout should be empty, got:\n%s", out.String())
	}
	if !strings.Contains(errOut.String(), "No workspaces registered.") {
		t.Errorf("missing empty notice; stderr=%q", errOut.String())
	}
}

func TestWorkspacesList_Empty_Json(t *testing.T) {
	ts := newWorkspacesServer(t, map[string]interface{}{"workspaces": []map[string]interface{}{}}, http.StatusOK)
	setHostPort(t, ts)

	var out, errOut bytes.Buffer
	code := runWorkspacesListWithIO([]string{"--json"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, errOut.String())
	}
	got := strings.TrimRight(out.String(), "\n")
	want := `{"workspaces":[]}`
	if got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestWorkspacesList_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	t.Cleanup(ts.Close)
	setHostPort(t, ts)

	var out, errOut bytes.Buffer
	code := runWorkspacesListWithIO(nil, &out, &errOut)
	if code == 0 {
		t.Fatalf("exit = 0, want non-zero")
	}
	if errOut.Len() == 0 {
		t.Errorf("expected error on stderr")
	}
}

func TestWorkspacesList_LongPathTruncation(t *testing.T) {
	longPath := "/Users/tamlh/workspaces/self/AI/Tools/nano-brain/very/deeply/nested/example/path/project"
	payload := map[string]interface{}{"workspaces": []map[string]interface{}{
		{
			"hash":      "deadbeef0011223344",
			"root_path": longPath,
			"name":      "deep",
			"doc_count": 0,
		},
	}}
	ts := newWorkspacesServer(t, payload, http.StatusOK)
	setHostPort(t, ts)

	var out, errOut bytes.Buffer
	code := runWorkspacesListWithIO(nil, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, errOut.String())
	}
	got := truncateLeft(longPath, 50)
	if !strings.HasPrefix(got, "..") {
		t.Fatalf("truncateLeft helper did not prefix '..'; got %q", got)
	}
	if !strings.Contains(out.String(), got) {
		t.Errorf("rendered table missing truncated path %q; got:\n%s", got, out.String())
	}
}

func TestWorkspacesList_NeverColumn(t *testing.T) {
	payload := map[string]interface{}{"workspaces": []map[string]interface{}{
		{
			"hash":                  "h1",
			"root_path":             "/p",
			"name":                  "n",
			"doc_count":             0,
			"last_document_updated": nil,
		},
	}}
	ts := newWorkspacesServer(t, payload, http.StatusOK)
	setHostPort(t, ts)

	var out, errOut bytes.Buffer
	code := runWorkspacesListWithIO(nil, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "never") {
		t.Errorf("expected 'never' in column; got:\n%s", out.String())
	}
}

func TestWorkspacesAliasLs(t *testing.T) {
	payload := map[string]interface{}{"workspaces": []map[string]interface{}{
		{"hash": "h", "root_path": "/p", "name": "n", "doc_count": 1},
	}}
	ts := newWorkspacesServer(t, payload, http.StatusOK)
	setHostPort(t, ts)

	var out, errOut bytes.Buffer
	codeList := runWorkspacesListWithIO(nil, &out, &errOut)
	listOut := out.String()
	out.Reset()
	errOut.Reset()

	codeLs := runWorkspacesListWithIO([]string{}, &out, &errOut)
	if codeList != 0 || codeLs != 0 {
		t.Fatalf("exit codes: list=%d ls=%d", codeList, codeLs)
	}
	if listOut != out.String() {
		t.Errorf("ls output differs from list:\nlist:\n%s\nls:\n%s", listOut, out.String())
	}
}

func TestWorkspacesNoArgsDefaultsToList(t *testing.T) {
	payload := map[string]interface{}{"workspaces": []map[string]interface{}{
		{"hash": "h", "root_path": "/p", "name": "n", "doc_count": 1},
	}}
	ts := newWorkspacesServer(t, payload, http.StatusOK)
	setHostPort(t, ts)

	var out, errOut bytes.Buffer
	code := runWorkspacesListWithIO([]string{}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, errOut.String())
	}
	if !strings.Contains(out.String(), "HASH") {
		t.Errorf("expected list output; got:\n%s", out.String())
	}
}

func TestWorkspacesFlagOnlyDefaultsToList(t *testing.T) {
	payload := map[string]interface{}{"workspaces": []map[string]interface{}{
		{"hash": "h", "root_path": "/p", "name": "n", "doc_count": 1},
	}}
	ts := newWorkspacesServer(t, payload, http.StatusOK)
	setHostPort(t, ts)

	var out, errOut bytes.Buffer
	code := runWorkspacesListWithIO([]string{"--json"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, errOut.String())
	}
	if !strings.HasPrefix(out.String(), "{") {
		t.Errorf("expected JSON object output; got:\n%s", out.String())
	}
}
