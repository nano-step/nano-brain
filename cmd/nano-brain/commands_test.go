package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetBaseURL_Defaults(t *testing.T) {
	t.Setenv("NANO_BRAIN_HOST", "")
	t.Setenv("NANO_BRAIN_PORT", "")
	got := getBaseURL()
	if got != "http://localhost:3100" {
		t.Errorf("getBaseURL() = %q, want %q", got, "http://localhost:3100")
	}
}

func TestGetBaseURL_EnvOverride(t *testing.T) {
	t.Setenv("NANO_BRAIN_HOST", "myhost")
	t.Setenv("NANO_BRAIN_PORT", "9090")
	got := getBaseURL()
	if got != "http://myhost:9090" {
		t.Errorf("getBaseURL() = %q, want %q", got, "http://myhost:9090")
	}
}

func TestGetBaseURL_PartialOverride(t *testing.T) {
	t.Setenv("NANO_BRAIN_HOST", "custom")
	t.Setenv("NANO_BRAIN_PORT", "")
	got := getBaseURL()
	if got != "http://custom:3100" {
		t.Errorf("getBaseURL() = %q, want %q", got, "http://custom:3100")
	}
}

func TestDoRequest_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer ts.Close()

	data, _, err := doRequest("GET", ts.URL+"/test", nil)
	if err != nil {
		t.Fatalf("doRequest() error = %v", err)
	}
	if !strings.Contains(string(data), "ok") {
		t.Errorf("expected response to contain 'ok', got %q", string(data))
	}
}

func TestDoRequest_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"fail"}`))
	}))
	defer ts.Close()

	data, _, err := doRequest("GET", ts.URL+"/test", nil)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "server returned 500") {
		t.Errorf("error = %q, want it to contain 'server returned 500'", err.Error())
	}
	if data == nil {
		t.Error("expected body data even on error")
	}
}

func TestDoRequest_ConnectionRefused(t *testing.T) {
	t.Setenv("NANO_BRAIN_HOST", "localhost")
	t.Setenv("NANO_BRAIN_PORT", "19999")
	_, _, err := doRequest("GET", "http://localhost:19999/test", nil)
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
	if !strings.Contains(err.Error(), "cannot connect to nano-brain server") &&
		!strings.Contains(err.Error(), "request failed") {
		t.Errorf("error = %q, expected connection error message", err.Error())
	}
}

func TestDoRequest_NotFound_ReturnsBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not_found"}`))
	}))
	defer ts.Close()

	data, _, err := doRequest("POST", ts.URL+"/api/v1/query", strings.NewReader(`{}`))
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "server returned 404") {
		t.Errorf("error = %q, want 'server returned 404'", err.Error())
	}
	if !strings.Contains(string(data), "not_found") {
		t.Errorf("body = %q, want 'not_found'", string(data))
	}
}

func TestDoRequest_SetsContentType(t *testing.T) {
	var gotCT string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	_, _, _ = doRequest("POST", ts.URL+"/test", strings.NewReader(`{"foo":"bar"}`))
	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q, want %q", gotCT, "application/json")
	}
}

func TestDoRequest_NoContentTypeWithoutBody(t *testing.T) {
	var gotCT string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	_, _, _ = doRequest("GET", ts.URL+"/test", nil)
	if gotCT != "" {
		t.Errorf("Content-Type = %q, want empty for nil body", gotCT)
	}
}

func TestGetBaseURL_EnvVarsFromOS(t *testing.T) {
	t.Setenv("NANO_BRAIN_HOST", "check")
	t.Setenv("NANO_BRAIN_PORT", "4444")

	got := getBaseURL()
	if got != "http://check:4444" {
		t.Errorf("getBaseURL() = %q, want %q", got, "http://check:4444")
	}
}

func TestInitCmdBuildsCorrectBody(t *testing.T) {
	var capturedBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"workspace_hash": "hash123",
			"root_path":      "/test/path",
			"agents_snippet": "",
		})
	}))
	defer ts.Close()

	t.Setenv("NANO_BRAIN_HOST", strings.TrimPrefix(strings.TrimPrefix(ts.URL, "http://"), ":"))
	parts := strings.Split(strings.TrimPrefix(ts.URL, "http://"), ":")
	if len(parts) > 1 {
		t.Setenv("NANO_BRAIN_PORT", parts[1])
	}

	resp, _, err := doRequest("POST", ts.URL+"/api/v1/init",
		bytes.NewReader([]byte(`{"root_path":"/test/path","workspace":"myhash"}`)))
	if err != nil {
		t.Fatalf("doRequest failed: %v", err)
	}

	var result map[string]string
	_ = json.Unmarshal(resp, &result)
	if result["workspace_hash"] != "hash123" {
		t.Errorf("response workspace_hash = %s, want hash123", result["workspace_hash"])
	}
}

func TestWriteCmdWithTagsBuildsCorrectBody(t *testing.T) {
	var capturedBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "doc123"})
	}))
	defer ts.Close()

	writeBody := map[string]interface{}{
		"content":   "test content",
		"workspace": "ws123",
		"tags":      []string{"tag1", "tag2"},
	}
	data, _ := json.Marshal(writeBody)

	resp, _, err := doRequest("POST", ts.URL+"/api/v1/write", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("doRequest failed: %v", err)
	}

	if !strings.Contains(string(resp), "doc123") {
		t.Errorf("response = %s, want to contain doc123", string(resp))
	}
}

func TestStubCmdHandles404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not_found"}`))
	}))
	defer ts.Close()

	_, _, err := doRequest("POST", ts.URL+"/api/v1/query",
		bytes.NewReader([]byte(`{"query":"test","workspace":"ws123"}`)))
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "server returned 404") {
		t.Errorf("error = %s, want to contain 'server returned 404'", err.Error())
	}
}

func TestIsNpxLaunched(t *testing.T) {
	cases := []struct {
		name        string
		execpath    string
		packageName string
		want        bool
	}{
		{"both unset", "", "", false},
		{"npm_execpath set", "/path/to/npx-cli.js", "", true},
		{"npm_package_name set", "", "@nano-step/nano-brain", true},
		{"both set", "/path/to/npx-cli.js", "@nano-step/nano-brain", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("npm_execpath", tc.execpath)
			t.Setenv("npm_package_name", tc.packageName)
			if got := isNpxLaunched(); got != tc.want {
				t.Errorf("isNpxLaunched() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSuggestStartCommand(t *testing.T) {
	cases := []struct {
		name        string
		execpath    string
		packageName string
		want        string
	}{
		{"binary", "", "", "nano-brain serve -d"},
		{"npx via execpath", "/path/to/npx-cli.js", "", "npx @nano-step/nano-brain@beta serve -d"},
		{"npx via package_name", "", "@nano-step/nano-brain", "npx @nano-step/nano-brain@beta serve -d"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("npm_execpath", tc.execpath)
			t.Setenv("npm_package_name", tc.packageName)
			if got := suggestStartCommand(); got != tc.want {
				t.Errorf("suggestStartCommand() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFormatConnectError_Structure(t *testing.T) {
	t.Setenv("npm_execpath", "")
	t.Setenv("npm_package_name", "")

	got := formatConnectError("localhost", 3100)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), got)
	}
	if lines[0] != "Error: cannot connect to nano-brain server at localhost:3100" {
		t.Errorf("line 1 = %q", lines[0])
	}
	if lines[1] != "The server does not appear to be running." {
		t.Errorf("line 2 = %q", lines[1])
	}
	if lines[2] != "Run this to start it: nano-brain serve -d" {
		t.Errorf("line 3 = %q", lines[2])
	}
}

func TestFormatConnectError_CustomHostPort(t *testing.T) {
	t.Setenv("npm_execpath", "")
	t.Setenv("npm_package_name", "")

	got := formatConnectError("my-host", 9999)
	if !strings.Contains(got, "my-host:9999") {
		t.Errorf("expected host:port in output, got %q", got)
	}
}

func TestFormatConnectError_NpxSuggestion(t *testing.T) {
	t.Setenv("npm_execpath", "/path/to/npx-cli.js")

	got := formatConnectError("localhost", 3100)
	if !strings.Contains(got, "npx @nano-step/nano-brain@beta serve -d") {
		t.Errorf("expected npx suggestion, got %q", got)
	}
}

func TestIsTTY_NonTTYStdin(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	if isTTY() {
		t.Error("isTTY() = true when stdin is a pipe, want false")
	}
}

func TestIsTTY_NonTTYStderr(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	origStderr := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = origStderr }()

	if isTTY() {
		t.Error("isTTY() = true when stderr is a pipe, want false")
	}
}

func TestIsCharDevice_NilFile(t *testing.T) {
	if isCharDevice(nil) {
		t.Error("isCharDevice(nil) = true, want false")
	}
}

func TestIsCharDevice_RegularFile(t *testing.T) {
	tmp, err := os.CreateTemp("", "nano-brain-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	if isCharDevice(tmp) {
		t.Error("isCharDevice(regular file) = true, want false")
	}
}

func pointHTTPClientAt(t *testing.T, ts *httptest.Server) {
	t.Helper()
	u := strings.TrimPrefix(ts.URL, "http://")
	parts := strings.Split(u, ":")
	t.Setenv("NANO_BRAIN_HOST", parts[0])
	if len(parts) > 1 {
		t.Setenv("NANO_BRAIN_PORT", parts[1])
	}
}

func TestWaitForServerHealthy_BecomesHealthy(t *testing.T) {
	start := time.Now()
	var hits int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if time.Since(start) < 400*time.Millisecond {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	pointHTTPClientAt(t, ts)

	if err := waitForServerHealthy(3 * time.Second); err != nil {
		t.Fatalf("waitForServerHealthy() error = %v, want nil", err)
	}
	if atomic.LoadInt32(&hits) < 2 {
		t.Errorf("expected at least 2 polls, got %d", hits)
	}
}

func TestWaitForServerHealthy_Timeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()
	pointHTTPClientAt(t, ts)

	start := time.Now()
	err := waitForServerHealthy(500 * time.Millisecond)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "did not become healthy") {
		t.Errorf("error = %q, expected 'did not become healthy'", err.Error())
	}
	if elapsed < 500*time.Millisecond {
		t.Errorf("returned too early: %s", elapsed)
	}
	if elapsed > 2*time.Second {
		t.Errorf("returned too late: %s", elapsed)
	}
}

func TestWaitForServerHealthy_UnreachableHost(t *testing.T) {
	t.Setenv("NANO_BRAIN_HOST", "127.0.0.1")
	t.Setenv("NANO_BRAIN_PORT", "19998")

	start := time.Now()
	err := waitForServerHealthy(400 * time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if time.Since(start) < 400*time.Millisecond {
		t.Errorf("returned too early")
	}
}

func TestPromptStartServer(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"empty input (Enter)", "\n", true},
		{"capital Y", "Y\n", true},
		{"lowercase y", "y\n", true},
		{"yes-with-trailing-text", "yes\n", true},
		{"capital N", "N\n", false},
		{"lowercase n", "n\n", false},
		{"no", "no\n", false},
		{"garbage", "abort\n", false},
		{"whitespace only", "   \n", true},
		{"eof", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader := bytes.NewBufferString(tc.input)
			writer := &bytes.Buffer{}
			got := promptStartServer(reader, writer)
			if got != tc.want {
				t.Errorf("promptStartServer(%q) = %v, want %v", tc.input, got, tc.want)
			}
			if !strings.Contains(writer.String(), "Start server now? [Y/n]:") {
				t.Errorf("expected prompt text in output, got %q", writer.String())
			}
		})
	}
}

func withRecoveryHooks(t *testing.T, isTTYReturn bool, accept bool, daemon func()) {
	t.Helper()
	origIsTTY := isTTYFn
	origReader := promptReader
	origWriter := promptWriter
	origDaemon := runServeDaemonFn

	isTTYFn = func() bool { return isTTYReturn }
	if accept {
		promptReader = bytes.NewBufferString("Y\n")
	} else {
		promptReader = bytes.NewBufferString("n\n")
	}
	promptWriter = &bytes.Buffer{}
	if daemon == nil {
		daemon = func() {}
	}
	runServeDaemonFn = func(string) { daemon() }

	t.Cleanup(func() {
		isTTYFn = origIsTTY
		promptReader = origReader
		promptWriter = origWriter
		runServeDaemonFn = origDaemon
	})
}

func TestDoRequest_ConnectionRefused_NoTTY_NoPrompt(t *testing.T) {
	t.Setenv("NANO_BRAIN_HOST", "127.0.0.1")
	t.Setenv("NANO_BRAIN_PORT", "19997")
	t.Setenv("NANO_BRAIN_NO_AUTO_START", "")

	daemonCalled := false
	withRecoveryHooks(t, false, true, func() { daemonCalled = true })

	_, _, err := doRequest("GET", "http://127.0.0.1:19997/test", nil)
	if err == nil {
		t.Fatal("expected connection refused error")
	}
	if daemonCalled {
		t.Error("daemon should NOT be called when TTY is false")
	}
}

func TestDoRequest_ConnectionRefused_NoAutoStartEnv(t *testing.T) {
	t.Setenv("NANO_BRAIN_HOST", "127.0.0.1")
	t.Setenv("NANO_BRAIN_PORT", "19996")
	t.Setenv("NANO_BRAIN_NO_AUTO_START", "1")

	daemonCalled := false
	withRecoveryHooks(t, true, true, func() { daemonCalled = true })

	_, _, err := doRequest("GET", "http://127.0.0.1:19996/test", nil)
	if err == nil {
		t.Fatal("expected connection refused error")
	}
	if daemonCalled {
		t.Error("daemon should NOT be called when NANO_BRAIN_NO_AUTO_START=1")
	}
}

func TestDoRequest_ConnectionRefused_UserDeclines(t *testing.T) {
	t.Setenv("NANO_BRAIN_HOST", "127.0.0.1")
	t.Setenv("NANO_BRAIN_PORT", "19995")
	t.Setenv("NANO_BRAIN_NO_AUTO_START", "")

	daemonCalled := false
	withRecoveryHooks(t, true, false, func() { daemonCalled = true })

	_, _, err := doRequest("GET", "http://127.0.0.1:19995/test", nil)
	if err == nil {
		t.Fatal("expected connection refused error")
	}
	if daemonCalled {
		t.Error("daemon should NOT be called when user declines")
	}
}

func TestDoRequest_ConnectionRefused_UserAcceptsTriggersRecovery(t *testing.T) {
	t.Setenv("NANO_BRAIN_NO_AUTO_START", "")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/status" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	pointHTTPClientAt(t, ts)

	bad := "http://127.0.0.1:19994/test"

	daemonCalled := false
	withRecoveryHooks(t, true, true, func() {
		daemonCalled = true
	})

	_, _, err := doRequest("GET", bad, nil)
	if err == nil {
		t.Fatal("expected retry to fail since original URL is still unreachable")
	}
	if !daemonCalled {
		t.Error("daemon should have been called after user accepted")
	}
	if !strings.Contains(err.Error(), "after auto-start") && !strings.Contains(err.Error(), "request failed") {
		t.Errorf("expected retry-stage error, got %q", err.Error())
	}
}

func TestDoRequest_RetrySucceeds_HappyPath(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/status" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"retried":true}`))
	}))
	defer ts.Close()
	pointHTTPClientAt(t, ts)

	target := ts.URL + "/api/v1/echo"

	daemonCalled := false
	withRecoveryHooks(t, true, true, func() { daemonCalled = true })

	t.Setenv("NANO_BRAIN_NO_AUTO_START", "")
	data, status, err := doRequest("GET", target, nil)
	if err != nil {
		t.Fatalf("happy path doRequest error = %v", err)
	}
	if status != http.StatusOK {
		t.Errorf("status = %d", status)
	}
	if !strings.Contains(string(data), "retried") {
		t.Errorf("body = %q", data)
	}
	if daemonCalled {
		t.Error("daemon should NOT be called when server is reachable")
	}
}

func TestDoRequest_BodyBufferedForReplay(t *testing.T) {
	var bodies []string
	var mu sync.Mutex

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(b))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()
	pointHTTPClientAt(t, ts)

	body := bytes.NewReader([]byte(`{"hello":"world"}`))
	_, _, err := doRequest("POST", ts.URL+"/test", body)
	if err != nil {
		t.Fatalf("happy path failed: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(bodies) != 1 || bodies[0] != `{"hello":"world"}` {
		t.Errorf("expected body forwarded once intact, got %v", bodies)
	}
}
