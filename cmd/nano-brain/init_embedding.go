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

// stepEmbedding implements the D-02.2 minimal embedding probe: Ollama
// reachable at defaultURL → silent default (zero questions), matching
// getDefaults()'s provider/url/model/concurrency exactly so this branch
// never diverges from the template's committed defaults. Ollama unreachable
// → exactly one prompt offering to point at a different Ollama-compatible
// URL; declining (or a closed stdin, EOF-means-decline per CR-01) degrades
// to BM25-only via the empty `provider: ""` YAML value (verified to
// correctly override the config default, see 13-RESEARCH.md). There is no
// provider-choice or concurrency prompt — voyage/cloud embedding setup is
// deferred to manual config-file editing (D-07). notes receives user-facing
// hints (the BM25-only note and the cloud-auth caveat) so callers/tests can
// assert on them independently of the returned YAML block.
func stepEmbedding(scanner *bufio.Scanner, notes io.Writer, defaultURL, defaultModel string) (embBlock string) {
	const embConcurrency = "3"

	if detectOllamaFn(defaultURL) {
		fmt.Printf("  ✓ Ollama detected at %s — using %s for embeddings\n", defaultURL, defaultModel)
		return fmt.Sprintf("embedding:\n  provider: ollama\n  url: %s\n  model: %s\n  concurrency: %s\n", defaultURL, defaultModel, embConcurrency)
	}

	fmt.Print("\n── Embedding ──\n")
	fmt.Println("  Ollama not found at the default URL — semantic search needs an embedding provider.")
	answer, ok := promptConsequential(scanner, "Ollama URL (blank for BM25 keyword search only)", "")
	if !ok || answer == "" {
		fmt.Fprintln(notes, "  BM25 keyword search only — re-run nano-brain init to enable embeddings later")
		return "embedding:\n  provider: \"\"\n"
	}

	printCloudCaveatIfRemote(notes, answer)
	return fmt.Sprintf("embedding:\n  provider: ollama\n  url: %s\n  model: %s\n  concurrency: %s\n", answer, defaultModel, embConcurrency)
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
