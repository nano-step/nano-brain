package harvest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func writeLines(t *testing.T, path string, lines []string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParseJSONLFile(t *testing.T) {
	t.Run("valid messages", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "ses_abc.jsonl")
		writeLines(t, path, []string{
			`{"type":"user","timestamp":"2026-01-01T10:00:00Z","content":"hello"}`,
			`{"type":"tool_use","timestamp":"2026-01-01T10:00:01Z","tool_name":"bash","tool_input":{"command":"ls"}}`,
			`{"type":"tool_result","timestamp":"2026-01-01T10:00:02Z","tool_name":"bash","tool_output":{"output":"file.txt"}}`,
		})

		msgs, err := parseJSONLFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(msgs) != 3 {
			t.Fatalf("got %d messages, want 3", len(msgs))
		}
		if msgs[0].Type != "user" {
			t.Errorf("first message type = %q, want user", msgs[0].Type)
		}
		if msgs[0].Content != "hello" {
			t.Errorf("first message content = %q, want hello", msgs[0].Content)
		}
		if msgs[1].Type != "tool_use" {
			t.Errorf("second message type = %q, want tool_use", msgs[1].Type)
		}
		if msgs[1].ToolName != "bash" {
			t.Errorf("second message tool_name = %q, want bash", msgs[1].ToolName)
		}
	})

	t.Run("skips invalid lines", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "ses_bad.jsonl")
		writeLines(t, path, []string{
			`{"type":"user","timestamp":"2026-01-01T10:00:00Z","content":"valid"}`,
			`{not json at all`,
			``,
			`{"type":"user","timestamp":"2026-01-01T10:00:01Z","content":"also valid"}`,
		})

		msgs, err := parseJSONLFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(msgs) != 2 {
			t.Fatalf("got %d messages, want 2 (invalid lines skipped)", len(msgs))
		}
	})

	t.Run("empty file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "ses_empty.jsonl")
		writeLines(t, path, []string{})

		msgs, err := parseJSONLFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(msgs) != 0 {
			t.Fatalf("got %d messages, want 0", len(msgs))
		}
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := parseJSONLFile("/nonexistent/ses_nope.jsonl")
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("skips lines with empty type", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "ses_notype.jsonl")
		writeLines(t, path, []string{
			`{"timestamp":"2026-01-01T10:00:00Z","content":"no type field"}`,
			`{"type":"user","timestamp":"2026-01-01T10:00:01Z","content":"has type"}`,
		})

		msgs, err := parseJSONLFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(msgs) != 1 {
			t.Fatalf("got %d messages, want 1", len(msgs))
		}
	})
}

func TestRenderClaudeCodeMarkdown(t *testing.T) {
	t.Run("full session", func(t *testing.T) {
		msgs := []claudeCodeMessage{
			{Type: "user", Timestamp: "2026-01-01T10:00:00Z", Content: "What is Go?"},
			{Type: "tool_use", Timestamp: "2026-01-01T10:00:01Z", ToolName: "bash", ToolInput: []byte(`{"command":"go version"}`)},
			{Type: "tool_result", Timestamp: "2026-01-01T10:00:02Z", ToolName: "bash", ToolOutput: []byte(`{"output":"go1.23"}`)},
		}

		md := renderClaudeCodeMarkdown("ses_test", msgs)

		if !strings.Contains(md, "session_id: ses_test") {
			t.Error("missing session_id in front-matter")
		}
		if !strings.Contains(md, "source: claude_code") {
			t.Error("missing source in front-matter")
		}
		if !strings.Contains(md, "message_count: 2") {
			t.Error("message_count should be 2 (user + tool_use)")
		}
		if !strings.Contains(md, "created_at: 2026-01-01T10:00:00Z") {
			t.Error("missing created_at from first message")
		}
		if !strings.Contains(md, "## human (2026-01-01T10:00:00Z)") {
			t.Error("missing human heading")
		}
		if !strings.Contains(md, "What is Go?") {
			t.Error("missing user content")
		}
		if !strings.Contains(md, "## assistant (2026-01-01T10:00:01Z)") {
			t.Error("missing assistant heading")
		}
		if !strings.Contains(md, "Tool: bash") {
			t.Error("missing tool name")
		}
		if !strings.Contains(md, "## tool_result (2026-01-01T10:00:02Z)") {
			t.Error("missing tool_result heading")
		}
		if !strings.Contains(md, "go1.23") {
			t.Error("missing tool output")
		}
		if !strings.HasPrefix(md, "---\n") {
			t.Error("front-matter should start with ---")
		}
	})

	t.Run("empty messages", func(t *testing.T) {
		md := renderClaudeCodeMarkdown("ses_empty", nil)
		if !strings.Contains(md, "message_count: 0") {
			t.Error("empty messages should show count 0")
		}
		if !strings.Contains(md, "source: claude_code") {
			t.Error("missing source")
		}
	})

	t.Run("user only session", func(t *testing.T) {
		msgs := []claudeCodeMessage{
			{Type: "user", Timestamp: "2026-01-01T10:00:00Z", Content: "just a question"},
		}
		md := renderClaudeCodeMarkdown("ses_user", msgs)
		if !strings.Contains(md, "message_count: 1") {
			t.Error("message_count should be 1")
		}
		if !strings.Contains(md, "## human") {
			t.Error("missing human heading")
		}
	})
}

func TestRenderClaudeCodeMarkdownHashStability(t *testing.T) {
	msgs := []claudeCodeMessage{
		{Type: "user", Timestamp: "2026-01-01T10:00:00Z", Content: "hello"},
	}

	md1 := renderClaudeCodeMarkdown("ses_hash", msgs)
	md2 := renderClaudeCodeMarkdown("ses_hash", msgs)

	h1 := sha256.Sum256([]byte(md1))
	h2 := sha256.Sum256([]byte(md2))

	if hex.EncodeToString(h1[:]) != hex.EncodeToString(h2[:]) {
		t.Error("same input should produce same hash")
	}
}

func TestClaudeCodeHarvestAllEmptyDir(t *testing.T) {
	dir := t.TempDir()
	h := &ClaudeCodeHarvester{
		sessionDir: dir,
		logger:     zerolog.Nop(),
	}

	harvested, skipped, errCount := h.HarvestAll(context.Background(), nil)
	if harvested != 0 || skipped != 0 || errCount != 0 {
		t.Errorf("empty dir: got h=%d s=%d e=%d", harvested, skipped, errCount)
	}
}

func TestClaudeCodeHarvestAllNonExistentDir(t *testing.T) {
	h := &ClaudeCodeHarvester{
		sessionDir: "/nonexistent/path/sessions",
		logger:     zerolog.Nop(),
	}

	harvested, skipped, errCount := h.HarvestAll(context.Background(), nil)
	if harvested != 0 || skipped != 0 || errCount != 0 {
		t.Errorf("nonexistent dir: got h=%d s=%d e=%d", harvested, skipped, errCount)
	}
}

func TestClaudeCodeHarvestAllSkipsNonJSONL(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not a session"), 0o644)
	os.WriteFile(filepath.Join(dir, "data.json"), []byte("{}"), 0o644)

	h := &ClaudeCodeHarvester{
		sessionDir: dir,
		logger:     zerolog.Nop(),
	}

	harvested, skipped, errCount := h.HarvestAll(context.Background(), nil)
	if harvested != 0 || skipped != 0 || errCount != 0 {
		t.Errorf("non-JSONL files: got h=%d s=%d e=%d", harvested, skipped, errCount)
	}
}

func TestRenderClaudeCodeMarkdownToolUseHashStability(t *testing.T) {
	msgs := []claudeCodeMessage{
		{
			Type:      "tool_use",
			Timestamp: "2026-01-01T10:00:00Z",
			ToolName:  "bash",
			ToolInput: json.RawMessage(`{"command":"ls -la","timeout":30,"cwd":"/tmp"}`),
		},
	}
	first := renderClaudeCodeMarkdown("test-session", msgs)
	for i := 0; i < 50; i++ {
		got := renderClaudeCodeMarkdown("test-session", msgs)
		if got != first {
			t.Fatalf("iteration %d: non-deterministic output.\nfirst:\n%s\ngot:\n%s", i, first, got)
		}
	}
}

func TestClaudeCodeHarvesterDedupHashDeterminism(t *testing.T) {
	msgs := []claudeCodeMessage{
		{
			Type:      "user",
			Timestamp: "2026-01-01T10:00:00Z",
			Content:   "What is the capital of France?",
		},
		{
			Type:      "tool_use",
			Timestamp: "2026-01-01T10:00:01Z",
			ToolName:  "search",
			ToolInput: json.RawMessage(`{"query":"capital of France"}`),
		},
	}

	md1 := renderClaudeCodeMarkdown("ses_test", msgs)
	md2 := renderClaudeCodeMarkdown("ses_test", msgs)

	h1 := sha256.Sum256([]byte(md1))
	h2 := sha256.Sum256([]byte(md2))

	hash1 := hex.EncodeToString(h1[:])
	hash2 := hex.EncodeToString(h2[:])

	if hash1 != hash2 {
		t.Error("identical input must produce identical SHA-256 hash")
	}
}

func TestClaudeCodeHarvesterDedupHashChange(t *testing.T) {
	msgs1 := []claudeCodeMessage{
		{
			Type:      "user",
			Timestamp: "2026-01-01T10:00:00Z",
			Content:   "Original question",
		},
	}

	msgs2 := []claudeCodeMessage{
		{
			Type:      "user",
			Timestamp: "2026-01-01T10:00:00Z",
			Content:   "Modified question",
		},
	}

	md1 := renderClaudeCodeMarkdown("ses_test", msgs1)
	md2 := renderClaudeCodeMarkdown("ses_test", msgs2)

	h1 := sha256.Sum256([]byte(md1))
	h2 := sha256.Sum256([]byte(md2))

	hash1 := hex.EncodeToString(h1[:])
	hash2 := hex.EncodeToString(h2[:])

	if hash1 == hash2 {
		t.Error("different content must produce different SHA-256 hash")
	}
}
