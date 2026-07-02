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
	mcpConfigCalls     int
}

func withOrchestratorHooks(t *testing.T, h *orchestratorHooks) {
	t.Helper()

	origTTY := isTTYFn
	origStepDatabase := stepDatabaseFn
	origStepEmbedding := stepEmbeddingFn
	origRunDoctor := runDoctorChecksFn
	origStepServe := stepServeFn
	origRegister := registerWorkspaceFn
	origMCPConfig := promptMCPClientConfigFn

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
	promptMCPClientConfigFn = func(scanner *bufio.Scanner, projectRoot, workspaceName string) {
		h.mcpConfigCalls++
	}

	t.Cleanup(func() {
		isTTYFn = origTTY
		stepDatabaseFn = origStepDatabase
		stepEmbeddingFn = origStepEmbedding
		runDoctorChecksFn = origRunDoctor
		stepServeFn = origStepServe
		registerWorkspaceFn = origRegister
		promptMCPClientConfigFn = origMCPConfig
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

	// Answer "k" to the keep/overwrite gate, then Enter for anything else
	// that might be read on the keep path (there should be none consumed).
	origStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = origStdin })
	go func() {
		_, _ = w.WriteString("k\n")
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
}

func TestRunInteractiveInit_QuestionBudget(t *testing.T) {
	h := &orchestratorHooks{}
	withOrchestratorHooks(t, h)

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")

	// Fresh config (no keep/overwrite gate consumed) → advanced gate (N,
	// default) → save confirm (Y, default) → register prompt (Y, default).
	// All step-internal prompts are stubbed out via the seams, so only the
	// orchestrator's own consequential prompts are counted here.
	answers := "N\nY\nY\n"
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

	captureStdout(t, func() {
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
}

func TestRunInteractiveInit_AdvancedGate(t *testing.T) {
	t.Run("default N skips advanced prompts", func(t *testing.T) {
		h := &orchestratorHooks{}
		withOrchestratorHooks(t, h)

		dir := t.TempDir()
		configPath := filepath.Join(dir, "config.yml")

		answers := "\nY\nY\n" // advanced=default(N), save=Y, register=Y
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

		if strings.Contains(out, "Harvester (session indexing)") {
			t.Error("default-N advanced gate should skip the harvester prompt block")
		}
		if strings.Contains(out, "Summarization (LLM session summaries)") {
			t.Error("default-N advanced gate should skip the summarization prompt block")
		}
	})

	t.Run("Y runs advanced prompts", func(t *testing.T) {
		h := &orchestratorHooks{}
		withOrchestratorHooks(t, h)

		dir := t.TempDir()
		configPath := filepath.Join(dir, "config.yml")

		// advanced=Y, then harvester/summarization/search/watcher/logging
		// answers all as blank (defaults), then save=Y, register=Y.
		answers := "Y\n" + strings.Repeat("\n", 14) + "Y\nY\n"
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

		if !strings.Contains(out, "Harvester (session indexing)") {
			t.Error("Y advanced gate should run the harvester prompt block")
		}
		if !strings.Contains(out, "Summarization (LLM session summaries)") {
			t.Error("Y advanced gate should run the summarization prompt block")
		}
	})
}

func TestRunInteractiveInit_Summary(t *testing.T) {
	h := &orchestratorHooks{}
	withOrchestratorHooks(t, h)

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")

	answers := "N\nY\nY\n"
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
