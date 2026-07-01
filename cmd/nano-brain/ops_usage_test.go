package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

// dispatchedCommands must be kept in sync with the case labels in main()'s
// command-dispatch switch (main.go). This regression test exists because
// printUsage() previously drifted out of sync with that switch (8 live
// commands were undocumented) — see issue #527.
var dispatchedCommands = []string{
	"serve", "stop", "restart", "collection", "init", "write", "query",
	"search", "vsearch", "workspaces", "harvest", "reindex", "bench",
	"db:migrate", "logs", "docker", "status", "config", "doctor", "context",
	"code-impact", "detect-changes", "reset-embeddings", "backfill-summaries",
	"cleanup-stale-raw", "cleanup-orphan-workspaces", "wake-up", "get",
	"tags", "multi-get", "auth", "mcp-url", "version", "help",
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestPrintUsage_ListsAllDispatchedCommands(t *testing.T) {
	out := captureStdout(t, printUsage)

	for _, cmd := range dispatchedCommands {
		if !strings.Contains(out, cmd) {
			t.Errorf("printUsage() output missing dispatched command %q — main.go's dispatch switch and printUsage() have drifted out of sync", cmd)
		}
	}
}

func TestPrintUsage_MentionsHelpCommand(t *testing.T) {
	out := captureStdout(t, printUsage)
	if !strings.Contains(out, "nano-brain help") && !strings.Contains(out, "help") {
		t.Error("printUsage() should mention the help command")
	}
}
