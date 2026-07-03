package main

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nano-brain/nano-brain/internal/health/doctor"
)

// withOrchestratorHooks saves/overrides/restores every seam runInteractiveInit
// depends on, mirroring withRecoveryHooks (commands_test.go) and
// withServeHooks (init_serve_test.go). Callers customize the returned struct
// fields before calling runInteractiveInit; call counts are recorded on the
// struct so tests can assert on them.
type orchestratorHooks struct {
	stepDatabaseCalls  int
	stepEmbeddingCalls int
	doctorCalls        int
	stepServeCalls     int
	registerCalls      int
}

func withOrchestratorHooks(t *testing.T, h *orchestratorHooks) {
	t.Helper()

	origTTY := isTTYFn
	origStepDatabase := stepDatabaseFn
	origStepEmbedding := stepEmbeddingFn
	origRunDoctor := runDoctorChecksFn
	origStepServe := stepServeFn
	origRegister := registerWorkspaceFn

	isTTYFn = func() bool { return true }

	stepDatabaseFn = func(scanner *bufio.Scanner, defaultURL string) (string, bool) {
		h.stepDatabaseCalls++
		return defaultURL, true
	}
	stepEmbeddingFn = func(scanner *bufio.Scanner, notes io.Writer, defaultURL, defaultModel string) string {
		h.stepEmbeddingCalls++
		return "embedding:\n  provider: \"\"\n"
	}
	runDoctorChecksFn = func(configPath string) []doctor.Check {
		h.doctorCalls++
		return []doctor.Check{
			{Name: "PostgreSQL", Status: "ok", Detail: "localhost:5432"},
		}
	}
	stepServeFn = func(scanner *bufio.Scanner, checks []doctor.Check, configPath string) serveOutcome {
		h.stepServeCalls++
		return serveAlreadyRunning
	}
	registerWorkspaceFn = func(root, workspace string, jsonFlag bool) (initResult, error) {
		h.registerCalls++
		return initResult{Name: "proj", WorkspaceHash: "abc123", RootPath: root}, nil
	}

	t.Cleanup(func() {
		isTTYFn = origTTY
		stepDatabaseFn = origStepDatabase
		stepEmbeddingFn = origStepEmbedding
		runDoctorChecksFn = origRunDoctor
		stepServeFn = origStepServe
		registerWorkspaceFn = origRegister
	})
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = w

	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		done <- buf.String()
	}()

	fn()

	os.Stdout = orig
	_ = w.Close()
	out := <-done
	return out
}

func TestRunInteractiveInit_NonTTY(t *testing.T) {
	h := &orchestratorHooks{}
	withOrchestratorHooks(t, h)
	isTTYFn = func() bool { return false }

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")

	origStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = origStdin })
	// No input is ever written to w — if runInteractiveInit tries to read a
	// prompt, the scanner will block forever, so any Scan() call would hang
	// this test out. Close write end immediately so a read (if attempted)
	// returns EOF rather than hanging, but the real assertion is the call
	// counts below.
	_ = w.Close()

	captureStdout(t, func() {
		runInteractiveInit(configPath)
	})

	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Errorf("expected no config file written in non-TTY mode, stat err = %v", err)
	}
	if h.stepDatabaseCalls != 0 || h.stepEmbeddingCalls != 0 || h.stepServeCalls != 0 || h.registerCalls != 0 {
		t.Errorf("expected zero step-seam calls in non-TTY mode, got db=%d emb=%d serve=%d register=%d",
			h.stepDatabaseCalls, h.stepEmbeddingCalls, h.stepServeCalls, h.registerCalls)
	}
}

func TestRunInteractiveInit_KeepExisting(t *testing.T) {
	h := &orchestratorHooks{}
	withOrchestratorHooks(t, h)

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")
	sentinel := "server:\n  host: localhost\n  port: 3100\n\ndatabase:\n  url: postgres://sentinel\n"
	if err := os.WriteFile(configPath, []byte(sentinel), 0600); err != nil {
		t.Fatalf("write sentinel config: %v", err)
	}
	infoBefore, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("stat sentinel config: %v", err)
	}

	// Answer "k" to the keep/overwrite gate, then Enter (= default cwd) at
	// the register prompt so the D-03 keep→serve→register chaining is
	// observable via the stubbed seams below.
	origStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = origStdin })
	go func() {
		_, _ = w.WriteString("k\n\n")
		_ = w.Close()
	}()

	captureStdout(t, func() {
		runInteractiveInit(configPath)
	})

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config after keep: %v", err)
	}
	if string(data) != sentinel {
		t.Errorf("keep path rewrote the config file; got %q, want unchanged %q", string(data), sentinel)
	}
	infoAfter, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("stat config after keep: %v", err)
	}
	if infoAfter.ModTime() != infoBefore.ModTime() {
		t.Errorf("keep path modified the config file mtime")
	}
	if h.stepDatabaseCalls != 0 {
		t.Errorf("keep path invoked stepDatabaseFn %d times, want 0", h.stepDatabaseCalls)
	}
	if h.stepEmbeddingCalls != 0 {
		t.Errorf("keep path invoked stepEmbeddingFn %d times, want 0", h.stepEmbeddingCalls)
	}
	if h.doctorCalls == 0 {
		t.Error("keep path should still proceed to the doctor step, but runDoctorChecksFn was never called")
	}
	// D-03: keep must chain into the service steps, not return after doctor.
	if h.stepServeCalls != 1 {
		t.Errorf("keep path invoked stepServeFn %d times, want 1", h.stepServeCalls)
	}
	if h.registerCalls != 1 {
		t.Errorf("keep path invoked registerWorkspaceFn %d times, want 1 (Enter = default cwd)", h.registerCalls)
	}
}

func TestRunInteractiveInit_RegisterSkipOnN(t *testing.T) {
	// D-15 / review CR-02: typing "n" at the register prompt skips
	// registration — it must NOT be treated as a literal directory path.
	h := &orchestratorHooks{}
	withOrchestratorHooks(t, h)

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configPath, []byte("server:\n  host: localhost\n  port: 3100\n"), 0600); err != nil {
		t.Fatalf("write sentinel config: %v", err)
	}

	origStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = origStdin })
	go func() {
		// "k" keeps the existing config; "n" declines registration.
		_, _ = w.WriteString("k\nn\n")
		_ = w.Close()
	}()

	out := captureStdout(t, func() {
		runInteractiveInit(configPath)
	})

	if h.registerCalls != 0 {
		t.Errorf("registerWorkspaceFn called %d times after answering n, want 0", h.registerCalls)
	}
	if !strings.Contains(out, "Skipped registration") {
		t.Errorf("output %q does not contain the skip-registration notice", out)
	}
}

// TestRunInteractiveInit_ZeroQuestions is the D-01 acceptance test: on a
// fresh config (probe steps stubbed to their zero-question outcomes), the
// orchestrator itself must read exactly ONE consequential prompt — the
// register-workspace prompt — with no advanced-settings gate, no save
// confirmation, and no config-detail prompts left to answer. Answering only
// the register prompt (Enter = default cwd) must be enough to complete the
// whole run.
func TestRunInteractiveInit_ZeroQuestions(t *testing.T) {
	h := &orchestratorHooks{}
	withOrchestratorHooks(t, h)

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")

	// Only the register prompt remains; stepDatabaseFn/stepEmbeddingFn are
	// stubbed to their zero-question return values, so a single blank line
	// (Enter = default cwd) is all runInteractiveInit needs to complete.
	answers := "\n"
	origStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = origStdin })
	go func() {
		_, _ = w.WriteString(answers)
		_ = w.Close()
	}()

	out := captureStdout(t, func() {
		runInteractiveInit(configPath)
	})

	if h.stepDatabaseCalls != 1 {
		t.Errorf("stepDatabaseFn called %d times, want 1", h.stepDatabaseCalls)
	}
	if h.stepEmbeddingCalls != 1 {
		t.Errorf("stepEmbeddingFn called %d times, want 1", h.stepEmbeddingCalls)
	}
	if h.stepServeCalls != 1 {
		t.Errorf("stepServeFn called %d times, want 1", h.stepServeCalls)
	}
	if h.registerCalls != 1 {
		t.Errorf("registerWorkspaceFn called %d times, want 1", h.registerCalls)
	}
	if strings.Contains(out, "Advanced settings?") {
		t.Error("D-09: the advanced-settings gate must be removed entirely")
	}
	if strings.Contains(out, "Save this config?") {
		t.Error("D-01: the save-confirmation prompt must be removed — writing is silent once probes complete")
	}
	if strings.Contains(out, "Harvester (session indexing)") || strings.Contains(out, "Summarization (LLM session summaries)") {
		t.Error("D-09: config-detail prompt blocks (stepAdvanced) must be gone")
	}
}

// TestRunInteractiveInit_WritesCommentedTemplate verifies the written
// config is the D-03/D-04 commented template (not the old ad-hoc
// fmt.Sprintf assembly) by checking for a section only the template
// contains, and that Save-confirmation preview/prompt text is gone.
func TestRunInteractiveInit_WritesCommentedTemplate(t *testing.T) {
	h := &orchestratorHooks{}
	withOrchestratorHooks(t, h)

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")

	origStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = origStdin })
	go func() {
		_, _ = w.WriteString("\n")
		_ = w.Close()
	}()

	out := captureStdout(t, func() {
		runInteractiveInit(configPath)
	})

	if strings.Contains(out, "Config preview") {
		t.Error("the config preview step is gone — the commented template is self-documenting")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read written config: %v", err)
	}
	written := string(data)
	if !strings.Contains(written, "Advanced: disabled by default") {
		t.Errorf("written config missing the commented advanced-sections banner from the template:\n%s", written)
	}
	if !strings.Contains(written, "summarization:") || !strings.Contains(written, "enabled: false") {
		t.Errorf("written config missing the disabled summarization section:\n%s", written)
	}
}

func TestRunInteractiveInit_Summary(t *testing.T) {
	h := &orchestratorHooks{}
	withOrchestratorHooks(t, h)

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")

	answers := "\n" // register prompt only, Enter = default cwd
	origStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = origStdin })
	go func() {
		_, _ = w.WriteString(answers)
		_ = w.Close()
	}()

	out := captureStdout(t, func() {
		runInteractiveInit(configPath)
	})

	if !strings.Contains(out, "proj") {
		t.Errorf("summary output missing workspace name %q: %s", "proj", out)
	}
	if !strings.Contains(out, "abc123") {
		t.Errorf("summary output missing workspace hash %q: %s", "abc123", out)
	}
	if !strings.Contains(out, "restart") {
		t.Errorf("summary output missing restart-your-AI-client next action: %s", out)
	}
	if !strings.Contains(out, "http://") {
		t.Errorf("summary output missing server URL: %s", out)
	}
}

// TestRunNonInteractiveInit_ZeroPrompts is the D-08 acceptance test: with
// stepDatabaseFn/stepEmbeddingFn stubbed to their zero-question outcomes,
// runNonInteractiveInit must complete without ever touching os.Stdin (no
// pipe is set up at all — if it tried to read a prompt, the default
// bufio.Scanner over os.Stdin would either block or read unrelated test
// process input) and write a valid config.
func TestRunNonInteractiveInit_ZeroPrompts(t *testing.T) {
	h := &orchestratorHooks{}
	withOrchestratorHooks(t, h)

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")

	captureStdout(t, func() {
		runNonInteractiveInit(configPath)
	})

	if h.stepDatabaseCalls != 1 {
		t.Errorf("stepDatabaseFn called %d times, want 1", h.stepDatabaseCalls)
	}
	if h.stepEmbeddingCalls != 1 {
		t.Errorf("stepEmbeddingFn called %d times, want 1", h.stepEmbeddingCalls)
	}
	if h.doctorCalls != 1 {
		t.Errorf("runDoctorChecksFn called %d times, want 1", h.doctorCalls)
	}
	if h.stepServeCalls != 0 || h.registerCalls != 0 {
		t.Errorf("runNonInteractiveInit must not chain into serve/register, got serve=%d register=%d", h.stepServeCalls, h.registerCalls)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read written config: %v", err)
	}
	written := string(data)
	if !strings.Contains(written, "url: postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev") {
		t.Errorf("written config missing the default database URL:\n%s", written)
	}
}

// TestBuildRenderedConfig_DatabaseAbort verifies the shared probe helper's
// ok=false contract that both runInteractiveInit and runNonInteractiveInit
// rely on to avoid writing a config with no usable database.url: when
// stepDatabaseFn reports failure, buildRenderedConfig must return ok=false
// without calling stepEmbeddingFn at all.
func TestBuildRenderedConfig_DatabaseAbort(t *testing.T) {
	origStepDatabase := stepDatabaseFn
	origStepEmbedding := stepEmbeddingFn
	t.Cleanup(func() {
		stepDatabaseFn = origStepDatabase
		stepEmbeddingFn = origStepEmbedding
	})

	stepDatabaseFn = func(scanner *bufio.Scanner, defaultURL string) (string, bool) {
		return "", false
	}
	embeddingCalled := false
	stepEmbeddingFn = func(scanner *bufio.Scanner, notes io.Writer, defaultURL, defaultModel string) string {
		embeddingCalled = true
		return "embedding:\n  provider: \"\"\n"
	}

	scanner := bufio.NewScanner(strings.NewReader(""))
	yaml, _, ok := buildRenderedConfig(scanner)

	if ok {
		t.Error("buildRenderedConfig() ok = true, want false when stepDatabaseFn fails")
	}
	if yaml != "" {
		t.Errorf("buildRenderedConfig() yaml = %q, want empty on abort", yaml)
	}
	if embeddingCalled {
		t.Error("stepEmbeddingFn must not be called when the database step aborts")
	}
}
