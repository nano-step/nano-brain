package main

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

// withDetectOllamaFn overrides the package-level detectOllamaFn seam for the
// duration of a test and restores it on cleanup, so tests never hit the
// network (acceptance criteria: no real network call in tests).
func withDetectOllamaFn(t *testing.T, fn func(url string) bool) {
	t.Helper()
	orig := detectOllamaFn
	detectOllamaFn = fn
	t.Cleanup(func() { detectOllamaFn = orig })
}

// TestStepEmbedding_OllamaUp covers D-02.2's zero-question path: Ollama
// reachable at the default URL silently defaults to provider/url/model/
// concurrency with NO prompt read at all (the scanner has no input queued,
// so a Scan() call here would return false/empty and the test would still
// need to pass — the real assertion is that the well-known default values
// appear, matching getDefaults()).
func TestStepEmbedding_OllamaUp(t *testing.T) {
	withDetectOllamaFn(t, func(string) bool { return true })

	scanner := bufio.NewScanner(strings.NewReader(""))
	var notes bytes.Buffer
	block := stepEmbedding(scanner, &notes, "http://localhost:11434", "nomic-embed-text")

	if !strings.Contains(block, "provider: ollama") {
		t.Errorf("stepEmbedding() ollama-up block = %q, want provider: ollama", block)
	}
	if !strings.Contains(block, "http://localhost:11434") {
		t.Errorf("stepEmbedding() ollama-up block = %q, want the default URL", block)
	}
	if !strings.Contains(block, "nomic-embed-text") {
		t.Errorf("stepEmbedding() ollama-up block = %q, want the default model", block)
	}
	if !strings.Contains(block, "concurrency: 3") {
		t.Errorf("stepEmbedding() ollama-up block = %q, want concurrency: 3", block)
	}
	if notes.Len() != 0 {
		t.Errorf("stepEmbedding() ollama-up notes = %q, want empty (no caveat for the local default)", notes.String())
	}
}

// TestStepEmbedding_OllamaDownBlankAnswer covers D-02.2's down branch when
// the single prompt is declined (blank answer): degrades to BM25-only.
func TestStepEmbedding_OllamaDownBlankAnswer(t *testing.T) {
	withDetectOllamaFn(t, func(string) bool { return false })

	scanner := bufio.NewScanner(strings.NewReader("\n"))
	var notes bytes.Buffer
	block := stepEmbedding(scanner, &notes, "http://localhost:11434", "nomic-embed-text")

	if !strings.Contains(block, `provider: ""`) {
		t.Errorf("stepEmbedding() blank-answer block = %q, want it to contain `provider: \"\"`", block)
	}
	if !strings.Contains(notes.String(), "BM25") {
		t.Errorf("stepEmbedding() notes = %q, want a BM25-only note", notes.String())
	}
}

// TestStepEmbedding_OllamaDownEOF covers the EOF-means-decline convention
// (CR-01): a closed stdin at the single prompt must also degrade to
// BM25-only, never be treated as consent.
func TestStepEmbedding_OllamaDownEOF(t *testing.T) {
	withDetectOllamaFn(t, func(string) bool { return false })

	scanner := bufio.NewScanner(strings.NewReader(""))
	var notes bytes.Buffer
	block := stepEmbedding(scanner, &notes, "http://localhost:11434", "nomic-embed-text")

	if !strings.Contains(block, `provider: ""`) {
		t.Errorf("stepEmbedding() EOF block = %q, want it to contain `provider: \"\"`", block)
	}
}

// TestStepEmbedding_OllamaDownProvidedURL covers D-02.2's down branch when
// the user provides a URL: returns an ollama block using that URL and the
// default model, with no provider-choice or concurrency prompt.
func TestStepEmbedding_OllamaDownProvidedURL(t *testing.T) {
	withDetectOllamaFn(t, func(string) bool { return false })

	scanner := bufio.NewScanner(strings.NewReader("http://192.168.1.50:11434\n"))
	var notes bytes.Buffer
	block := stepEmbedding(scanner, &notes, "http://localhost:11434", "nomic-embed-text")

	if !strings.Contains(block, "provider: ollama") {
		t.Errorf("stepEmbedding() provided-URL block = %q, want provider: ollama", block)
	}
	if !strings.Contains(block, "http://192.168.1.50:11434") {
		t.Errorf("stepEmbedding() provided-URL block = %q, want the entered URL", block)
	}
	if !strings.Contains(block, "nomic-embed-text") {
		t.Errorf("stepEmbedding() provided-URL block = %q, want the default model (no model prompt)", block)
	}
}

// TestStepEmbedding_CloudCaveat covers Pitfall 6: entering a non-local
// Ollama URL at the single down-branch prompt must still print a cloud-auth
// caveat hint (no new auth code).
func TestStepEmbedding_CloudCaveat(t *testing.T) {
	withDetectOllamaFn(t, func(string) bool { return false })

	scanner := bufio.NewScanner(strings.NewReader("https://ollama.com\n"))
	var notes bytes.Buffer
	block := stepEmbedding(scanner, &notes, "http://localhost:11434", "nomic-embed-text")

	if !strings.Contains(block, "https://ollama.com") {
		t.Errorf("stepEmbedding() cloud block = %q, want the entered cloud URL", block)
	}
	if !strings.Contains(notes.String(), "API key") {
		t.Errorf("stepEmbedding() notes = %q, want a cloud-auth caveat mentioning an API key", notes.String())
	}
}
