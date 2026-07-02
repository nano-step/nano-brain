package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/url"
)

// detectOllamaFn is a test seam over detectOllama so tests never perform a
// real network call.
var detectOllamaFn = detectOllama

// stepEmbedding implements the D-11/D-12 embedding wizard step: it first
// asks whether to enable semantic embeddings at all (D-11); declining
// degrades to BM25-only via an empty `provider: ""` YAML value (verified to
// correctly override the config default, see 13-RESEARCH.md). Accepting
// auto-detects a local Ollama instance at defaultURL (D-12); if found it
// confirms the URL + model, otherwise it prompts for a provider (ollama or
// voyage). notes receives user-facing hints (the BM25-only note and the
// cloud-auth caveat) so callers/tests can assert on them independently of
// the returned YAML block.
func stepEmbedding(scanner *bufio.Scanner, notes io.Writer, defaultURL, defaultModel string) (embBlock string) {
	fmt.Print("\n── Embedding ──\n")
	fmt.Println("  Converts text chunks into vectors for semantic search.")

	enable := promptWithDefault(scanner, "Enable semantic embeddings?", "Y")
	if !isAffirmative(enable) {
		fmt.Fprintln(notes, "  BM25 keyword search only — re-run nano-brain init to enable embeddings later")
		return "embedding:\n  provider: \"\"\n"
	}

	fmt.Println("  Providers: ollama (local, free) | voyage (cloud, higher quality)")

	if detectOllamaFn(defaultURL) {
		fmt.Printf("  ✓ Ollama detected at %s\n", defaultURL)
		embURL := promptWithDefault(scanner, "Ollama URL", defaultURL)
		model := promptWithDefault(scanner, "Embedding model", defaultModel)
		printCloudCaveatIfRemote(notes, embURL)
		embConcurrency := "3"
		return fmt.Sprintf("embedding:\n  provider: ollama\n  url: %s\n  model: %s\n  concurrency: %s\n", embURL, model, embConcurrency)
	}

	fmt.Println("  Ollama not found at default URL.")
	provider := promptWithDefault(scanner, "Embedding provider (ollama/voyage)", "ollama")

	if provider == "voyage" {
		model := promptWithDefault(scanner, "Embedding model", "voyage-3")
		embConcurrency := "3"
		return fmt.Sprintf("embedding:\n  provider: voyage\n  model: %s\n  concurrency: %s\n", model, embConcurrency)
	}

	fmt.Println("  Enter any Ollama-compatible URL (local or remote).")
	embURL := promptWithDefault(scanner, "Ollama URL", defaultURL)
	fmt.Println("  Recommended embed models: nomic-embed-text (fast), mxbai-embed-large (better quality)")
	model := promptWithDefault(scanner, "Embedding model", defaultModel)
	printCloudCaveatIfRemote(notes, embURL)
	embConcurrency := "3"
	return fmt.Sprintf("embedding:\n  provider: ollama\n  url: %s\n  model: %s\n  concurrency: %s\n", embURL, model, embConcurrency)
}

// printCloudCaveatIfRemote prints a hint (Pitfall 6) when embURL is not a
// local/private-network address: hosted/cloud Ollama-compatible endpoints
// typically require an API key that nano-brain does not currently send.
// This is a printed hint only — no auth code is added.
func printCloudCaveatIfRemote(notes io.Writer, embURL string) {
	if isLocalOrPrivateURL(embURL) {
		return
	}
	fmt.Fprintln(notes, "  Note: hosted/cloud Ollama-compatible endpoints often require an API key;")
	fmt.Fprintln(notes, "  nano-brain does not currently send an Authorization header — self-hosted")
	fmt.Fprintln(notes, "  or local Ollama endpoints are the tested path.")
}

// isLocalOrPrivateURL reports whether embURL's host is localhost, a loopback
// address, or a private (RFC1918/RFC4193) address. Unparseable URLs are
// treated as non-local so the caveat is shown (safer default).
func isLocalOrPrivateURL(embURL string) bool {
	parsed, err := url.Parse(embURL)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	if host == "" {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate()
}
