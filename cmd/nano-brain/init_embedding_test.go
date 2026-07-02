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

// TestStepEmbedding_Disabled covers D-11: declining the enable gate must
// return a block with the empty-provider rendering (provider: "") and print
// a BM25-only note, with no further prompts read.
func TestStepEmbedding_Disabled(t *testing.T) {
	withDetectOllamaFn(t, func(string) bool {
		t.Fatal("detectOllamaFn must not be called when embeddings are declined")
		return false
	})

	scanner := bufio.NewScanner(strings.NewReader("n\n"))
	var notes bytes.Buffer
	block := stepEmbedding(scanner, &notes, "http://localhost:11434", "nomic-embed-text")

	if !strings.Contains(block, `provider: ""`) {
		t.Errorf("stepEmbedding() disabled block = %q, want it to contain `provider: \"\"`", block)
	}
	if strings.Contains(block, "provider: none") {
		t.Errorf("stepEmbedding() disabled block = %q, must not use a `none` sentinel", block)
	}
	if !strings.Contains(notes.String(), "BM25") {
		t.Errorf("stepEmbedding() notes = %q, want a BM25-only note", notes.String())
	}
}

// TestStepEmbedding_OllamaDetected covers D-12's found branch: an injected
// detectOllamaFn returning true should confirm the default URL + model and
// return an ollama block containing both.
func TestStepEmbedding_OllamaDetected(t *testing.T) {
	withDetectOllamaFn(t, func(url string) bool { return true })

	// y (enable) then Enter/Enter to accept the confirmed URL + model defaults.
	scanner := bufio.NewScanner(strings.NewReader("y\n\n\n"))
	var notes bytes.Buffer
	block := stepEmbedding(scanner, &notes, "http://localhost:11434", "nomic-embed-text")

	if !strings.Contains(block, "provider: ollama") {
		t.Errorf("stepEmbedding() ollama-detected block = %q, want provider: ollama", block)
	}
	if !strings.Contains(block, "http://localhost:11434") {
		t.Errorf("stepEmbedding() ollama-detected block = %q, want the default URL", block)
	}
	if !strings.Contains(block, "nomic-embed-text") {
		t.Errorf("stepEmbedding() ollama-detected block = %q, want the default model", block)
	}
}

// TestStepEmbedding_OllamaManual covers D-12's not-found branch: user
// selects provider "ollama" and enters a URL manually.
func TestStepEmbedding_OllamaManual(t *testing.T) {
	withDetectOllamaFn(t, func(url string) bool { return false })

	// y (enable), "ollama" (provider), URL, model (accept default).
	scanner := bufio.NewScanner(strings.NewReader("y\nollama\nhttp://192.168.1.50:11434\n\n"))
	var notes bytes.Buffer
	block := stepEmbedding(scanner, &notes, "http://localhost:11434", "nomic-embed-text")

	if !strings.Contains(block, "provider: ollama") {
		t.Errorf("stepEmbedding() ollama-manual block = %q, want provider: ollama", block)
	}
	if !strings.Contains(block, "http://192.168.1.50:11434") {
		t.Errorf("stepEmbedding() ollama-manual block = %q, want the entered URL", block)
	}
}

// TestStepEmbedding_Voyage covers D-12's voyage branch.
func TestStepEmbedding_Voyage(t *testing.T) {
	withDetectOllamaFn(t, func(url string) bool { return false })

	// y (enable), "voyage" (provider), model (accept default).
	scanner := bufio.NewScanner(strings.NewReader("y\nvoyage\n\n"))
	var notes bytes.Buffer
	block := stepEmbedding(scanner, &notes, "http://localhost:11434", "nomic-embed-text")

	if !strings.Contains(block, "provider: voyage") {
		t.Errorf("stepEmbedding() voyage block = %q, want provider: voyage", block)
	}
	if !strings.Contains(block, "voyage-3") {
		t.Errorf("stepEmbedding() voyage block = %q, want the default voyage-3 model", block)
	}
}

// TestStepEmbedding_CloudCaveat covers Pitfall 6: entering a non-local
// Ollama URL must print a cloud-auth caveat hint (no new auth code).
func TestStepEmbedding_CloudCaveat(t *testing.T) {
	withDetectOllamaFn(t, func(url string) bool { return false })

	// y (enable), "ollama" (provider), a non-local cloud URL, model (accept default).
	scanner := bufio.NewScanner(strings.NewReader("y\nollama\nhttps://ollama.com\n\n"))
	var notes bytes.Buffer
	block := stepEmbedding(scanner, &notes, "http://localhost:11434", "nomic-embed-text")

	if !strings.Contains(block, "https://ollama.com") {
		t.Errorf("stepEmbedding() cloud block = %q, want the entered cloud URL", block)
	}
	if !strings.Contains(notes.String(), "API key") {
		t.Errorf("stepEmbedding() notes = %q, want a cloud-auth caveat mentioning an API key", notes.String())
	}
}
