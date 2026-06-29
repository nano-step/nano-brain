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

// realUserLine returns a JSON line matching Claude Code's actual JSONL schema
// for a user turn.
func realUserLine(ts, text string) string {
	msg, _ := json.Marshal(map[string]any{
		"role":    "user",
		"content": text,
	})
	line, _ := json.Marshal(map[string]any{
		"type":      "user",
		"timestamp": ts,
		"message":   json.RawMessage(msg),
	})
	return string(line)
}

// realAssistantLine returns a JSON line matching Claude Code's actual JSONL
// schema for an assistant turn with a text block and an optional tool_use block.
func realAssistantLine(ts, text, toolName string, toolInput map[string]any) string {
	blocks := []map[string]any{}
	if text != "" {
		blocks = append(blocks, map[string]any{"type": "text", "text": text})
	}
	if toolName != "" {
		blocks = append(blocks, map[string]any{
			"type":  "tool_use",
			"name":  toolName,
			"input": toolInput,
		})
	}
	contentJSON, _ := json.Marshal(blocks)
	msg, _ := json.Marshal(map[string]any{
		"role":    "assistant",
		"content": json.RawMessage(contentJSON),
	})
	line, _ := json.Marshal(map[string]any{
		"type":      "assistant",
		"timestamp": ts,
		"message":   json.RawMessage(msg),
	})
	return string(line)
}

// modeLine returns the leading metadata record Claude Code writes before any
// user/assistant turns. It has no timestamp.
func modeLine(sessionID string) string {
	line, _ := json.Marshal(map[string]any{
		"type":      "mode",
		"mode":      "auto",
		"sessionId": sessionID,
	})
	return string(line)
}

// TestExtractText_UserContentArray covers the case where a user turn's
// message.content is an ARRAY of tool_result blocks (how Claude Code records
// tool output) rather than a plain string. extractText must fall through from
// the string path to the typed-block path and surface the result text.
func TestExtractText_UserContentArray(t *testing.T) {
	t.Run("tool_result with string content", func(t *testing.T) {
		content, _ := json.Marshal([]map[string]any{
			{"type": "tool_result", "content": "exit status 0\nbuild ok"},
		})
		msg, _ := json.Marshal(map[string]any{"role": "user", "content": json.RawMessage(content)})
		line, _ := json.Marshal(map[string]any{
			"type": "user", "timestamp": "2026-01-01T10:00:00Z", "message": json.RawMessage(msg),
		})
		var m claudeCodeMessage
		if err := json.Unmarshal(line, &m); err != nil {
			t.Fatal(err)
		}
		if got := m.extractText(); !strings.Contains(got, "build ok") {
			t.Errorf("extractText() = %q, want it to contain tool_result text 'build ok'", got)
		}
	})

	t.Run("tool_result with nested block content", func(t *testing.T) {
		inner, _ := json.Marshal([]map[string]any{{"type": "text", "text": "nested result"}})
		content, _ := json.Marshal([]map[string]any{
			{"type": "tool_result", "content": json.RawMessage(inner)},
		})
		msg, _ := json.Marshal(map[string]any{"role": "user", "content": json.RawMessage(content)})
		line, _ := json.Marshal(map[string]any{
			"type": "user", "timestamp": "2026-01-01T10:00:00Z", "message": json.RawMessage(msg),
		})
		var m claudeCodeMessage
		if err := json.Unmarshal(line, &m); err != nil {
			t.Fatal(err)
		}
		if got := m.extractText(); !strings.Contains(got, "nested result") {
			t.Errorf("extractText() = %q, want it to contain 'nested result'", got)
		}
	})
}

func TestParseJSONLFile(t *testing.T) {
	t.Run("valid messages with real schema", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "ses_abc.jsonl")
		writeLines(t, path, []string{
			modeLine("ses_abc"),
			realUserLine("2026-01-01T10:00:00Z", "hello"),
			realAssistantLine("2026-01-01T10:00:01Z", "world", "bash", map[string]any{"command": "ls"}),
		})

		msgs, err := parseJSONLFile(path)
		if err != nil {
			t.Fatal(err)
		}
		// All three lines have a non-empty type so all are kept by parseJSONLFile.
		if len(msgs) != 3 {
			t.Fatalf("got %d messages, want 3", len(msgs))
		}
		if msgs[0].Type != "mode" {
			t.Errorf("first message type = %q, want mode", msgs[0].Type)
		}
		if msgs[1].Type != "user" {
			t.Errorf("second message type = %q, want user", msgs[1].Type)
		}
		// Content is in the nested message envelope, not a top-level field.
		if got := msgs[1].extractText(); got != "hello" {
			t.Errorf("user extractText() = %q, want hello", got)
		}
		if msgs[2].Type != "assistant" {
			t.Errorf("third message type = %q, want assistant", msgs[2].Type)
		}
		// Assistant text + tool_use block should both appear.
		assistantText := msgs[2].extractText()
		if !strings.Contains(assistantText, "world") {
			t.Errorf("assistant extractText() missing text block: %q", assistantText)
		}
		if !strings.Contains(assistantText, "bash") {
			t.Errorf("assistant extractText() missing tool name: %q", assistantText)
		}
	})

	t.Run("skips invalid lines", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "ses_bad.jsonl")
		writeLines(t, path, []string{
			realUserLine("2026-01-01T10:00:00Z", "valid"),
			`{not json at all`,
			``,
			realUserLine("2026-01-01T10:00:01Z", "also valid"),
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
			`{"timestamp":"2026-01-01T10:00:00Z","message":{"role":"user","content":"no type field"}}`,
			realUserLine("2026-01-01T10:00:01Z", "has type"),
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
	t.Run("full session with real schema", func(t *testing.T) {
		// Build messages that match Claude Code's actual JSONL schema.
		userMsgContent, _ := json.Marshal("What is Go?")
		assistantBlocks, _ := json.Marshal([]map[string]any{
			{"type": "text", "text": "Go is a language."},
			{"type": "tool_use", "name": "bash", "input": map[string]any{"command": "go version"}},
		})
		msgs := []claudeCodeMessage{
			{
				Type:      "mode",
				Timestamp: "", // no timestamp on leading metadata line
			},
			{
				Type:      "user",
				Timestamp: "2026-01-01T10:00:00Z",
				Message: struct {
					Role    string          `json:"role"`
					Content json.RawMessage `json:"content"`
				}{Role: "user", Content: userMsgContent},
			},
			{
				Type:      "assistant",
				Timestamp: "2026-01-01T10:00:01Z",
				Message: struct {
					Role    string          `json:"role"`
					Content json.RawMessage `json:"content"`
				}{Role: "assistant", Content: assistantBlocks},
			},
		}

		md := renderClaudeCodeMarkdown("ses_test", msgs)

		if !strings.Contains(md, "session_id: ses_test") {
			t.Error("missing session_id in front-matter")
		}
		if !strings.Contains(md, "source: claude_code") {
			t.Error("missing source in front-matter")
		}
		// message_count counts user+assistant turns (not the leading mode line).
		if !strings.Contains(md, "message_count: 2") {
			t.Errorf("message_count should be 2 (user + assistant), got:\n%s", md)
		}
		// created_at must come from the first message that has a timestamp (the
		// user turn, not the mode line which has none).
		if !strings.Contains(md, "created_at: 2026-01-01T10:00:00Z") {
			t.Errorf("missing created_at from first timestamped message, got:\n%s", md)
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
		if !strings.Contains(md, "Go is a language.") {
			t.Error("missing assistant text block")
		}
		if !strings.Contains(md, "bash") {
			t.Error("missing tool name in assistant output")
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
		userContent, _ := json.Marshal("just a question")
		msgs := []claudeCodeMessage{
			{
				Type:      "user",
				Timestamp: "2026-01-01T10:00:00Z",
				Message: struct {
					Role    string          `json:"role"`
					Content json.RawMessage `json:"content"`
				}{Role: "user", Content: userContent},
			},
		}
		md := renderClaudeCodeMarkdown("ses_user", msgs)
		if !strings.Contains(md, "message_count: 1") {
			t.Error("message_count should be 1")
		}
		if !strings.Contains(md, "## human") {
			t.Error("missing human heading")
		}
		if !strings.Contains(md, "just a question") {
			t.Error("missing user content")
		}
	})

	t.Run("mode-line-first timestamp sourcing", func(t *testing.T) {
		// Reproduces the Date:0001-01-01 bug: mode line has no timestamp, the
		// first real timestamp is on the user line.
		userContent, _ := json.Marshal("hi")
		msgs := []claudeCodeMessage{
			{Type: "mode", Timestamp: ""},
			{
				Type:      "user",
				Timestamp: "2026-06-01T09:00:00Z",
				Message: struct {
					Role    string          `json:"role"`
					Content json.RawMessage `json:"content"`
				}{Role: "user", Content: userContent},
			},
		}
		md := renderClaudeCodeMarkdown("ses_ts", msgs)
		if !strings.Contains(md, "created_at: 2026-06-01T09:00:00Z") {
			t.Errorf("expected created_at from user line, got:\n%s", md)
		}
	})
}

func TestRenderClaudeCodeMarkdownHashStability(t *testing.T) {
	userContent, _ := json.Marshal("hello")
	msgs := []claudeCodeMessage{
		{
			Type:      "user",
			Timestamp: "2026-01-01T10:00:00Z",
			Message: struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			}{Role: "user", Content: userContent},
		},
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
	_ = os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not a session"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "data.json"), []byte("{}"), 0o644)

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
	// Assistant message with a tool_use block — rendered output must be deterministic.
	blocks, _ := json.Marshal([]map[string]any{
		{"type": "tool_use", "name": "bash", "input": map[string]any{
			"command": "ls -la",
			"timeout": 30,
			"cwd":     "/tmp",
		}},
	})
	msgs := []claudeCodeMessage{
		{
			Type:      "assistant",
			Timestamp: "2026-01-01T10:00:00Z",
			Message: struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			}{Role: "assistant", Content: blocks},
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
	userContent, _ := json.Marshal("What is the capital of France?")
	blocks, _ := json.Marshal([]map[string]any{
		{"type": "tool_use", "name": "search", "input": map[string]any{"query": "capital of France"}},
	})
	msgs := []claudeCodeMessage{
		{
			Type:      "user",
			Timestamp: "2026-01-01T10:00:00Z",
			Message: struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			}{Role: "user", Content: userContent},
		},
		{
			Type:      "assistant",
			Timestamp: "2026-01-01T10:00:01Z",
			Message: struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			}{Role: "assistant", Content: blocks},
		},
	}

	md1 := renderClaudeCodeMarkdown("ses_test", msgs)
	md2 := renderClaudeCodeMarkdown("ses_test", msgs)

	h1 := sha256.Sum256([]byte(md1))
	h2 := sha256.Sum256([]byte(md2))

	if hex.EncodeToString(h1[:]) != hex.EncodeToString(h2[:]) {
		t.Error("identical input must produce identical SHA-256 hash")
	}
}

func TestClaudeCodeHarvesterDedupHashChange(t *testing.T) {
	makeMsg := func(text string) []claudeCodeMessage {
		c, _ := json.Marshal(text)
		return []claudeCodeMessage{
			{
				Type:      "user",
				Timestamp: "2026-01-01T10:00:00Z",
				Message: struct {
					Role    string          `json:"role"`
					Content json.RawMessage `json:"content"`
				}{Role: "user", Content: c},
			},
		}
	}

	md1 := renderClaudeCodeMarkdown("ses_test", makeMsg("Original question"))
	md2 := renderClaudeCodeMarkdown("ses_test", makeMsg("Modified question"))

	h1 := sha256.Sum256([]byte(md1))
	h2 := sha256.Sum256([]byte(md2))

	if hex.EncodeToString(h1[:]) == hex.EncodeToString(h2[:]) {
		t.Error("different content must produce different SHA-256 hash")
	}
}
