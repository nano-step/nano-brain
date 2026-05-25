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
	"time"

	"github.com/rs/zerolog"
)

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParseSessionFile(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "ses_abc.json")
		writeJSON(t, path, map[string]any{
			"id":        "ses_abc",
			"title":     "Test Session",
			"projectID": "proj123",
			"directory": "/some/dir",
			"time":      map[string]any{"created": 1700000000000, "updated": 1700000100000},
		})

		s, err := parseSessionFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if s.ID != "ses_abc" {
			t.Errorf("got ID %q, want ses_abc", s.ID)
		}
		if s.Title != "Test Session" {
			t.Errorf("got Title %q, want Test Session", s.Title)
		}
		if s.Time.Created != 1700000000000 {
			t.Errorf("got Created %d, want 1700000000000", s.Time.Created)
		}
	})

	t.Run("missing id", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "bad.json")
		writeJSON(t, path, map[string]any{"title": "no id"})

		_, err := parseSessionFile(path)
		if err == nil {
			t.Fatal("expected error for missing id")
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "bad.json")
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = os.WriteFile(path, []byte("{not json"), 0o644)

		_, err := parseSessionFile(path)
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := parseSessionFile("/nonexistent/path.json")
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})
}

func TestParseMessageFile(t *testing.T) {
	t.Run("valid user message", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "msg_001.json")
		writeJSON(t, path, map[string]any{
			"id":        "msg_001",
			"sessionID": "ses_abc",
			"role":      "user",
			"time":      map[string]any{"created": 1700000000000},
		})

		m, err := parseMessageFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if m.Role != "user" {
			t.Errorf("got role %q, want user", m.Role)
		}
		if m.CreatedAt.IsZero() {
			t.Error("CreatedAt should not be zero")
		}
		expected := time.UnixMilli(1700000000000)
		if !m.CreatedAt.Equal(expected) {
			t.Errorf("got CreatedAt %v, want %v", m.CreatedAt, expected)
		}
	})

	t.Run("missing id", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "msg.json")
		writeJSON(t, path, map[string]any{"role": "user"})

		_, err := parseMessageFile(path)
		if err == nil {
			t.Fatal("expected error for missing id")
		}
	})
}

func TestParsePartFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prt_001.json")
	writeJSON(t, path, map[string]any{
		"id":        "prt_001",
		"type":      "text",
		"text":      "Hello world",
		"messageID": "msg_001",
	})

	p, err := parsePartFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Type != "text" {
		t.Errorf("got type %q, want text", p.Type)
	}
	if p.Text != "Hello world" {
		t.Errorf("got text %q, want Hello world", p.Text)
	}
}

func TestRenderMarkdown(t *testing.T) {
	sess := &sessionFile{
		ID:    "ses_test",
		Title: "My Session",
	}
	sess.Time.Created = 1700000000000
	sess.Directory = "/work/project"

	ts1 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	ts2 := time.Date(2026, 1, 1, 10, 0, 5, 0, time.UTC)

	msgs := []renderedMessage{
		{Role: "user", Content: "What is Go?", CreatedAt: ts1},
		{Role: "assistant", Content: "Go is a programming language.", CreatedAt: ts2},
	}

	md := renderMarkdown(sess, msgs)

	if !strings.Contains(md, "session_id: ses_test") {
		t.Error("missing session_id in front-matter")
	}
	if !strings.Contains(md, "source: opencode") {
		t.Error("missing source in front-matter")
	}
	if !strings.Contains(md, "message_count: 2") {
		t.Error("missing or wrong message_count")
	}
	if !strings.Contains(md, `title: "My Session"`) {
		t.Error("missing title in front-matter")
	}
	if !strings.Contains(md, `directory: "/work/project"`) {
		t.Error("missing directory in front-matter")
	}
	if !strings.Contains(md, "## user (2026-01-01T10:00:00Z)") {
		t.Error("missing user message heading")
	}
	if !strings.Contains(md, "## assistant (2026-01-01T10:00:05Z)") {
		t.Error("missing assistant message heading")
	}
	if !strings.Contains(md, "What is Go?") {
		t.Error("missing user message content")
	}
	if !strings.Contains(md, "Go is a programming language.") {
		t.Error("missing assistant message content")
	}
	if !strings.HasPrefix(md, "---\n") {
		t.Error("front-matter should start with ---")
	}
}

func TestRenderMarkdownEmpty(t *testing.T) {
	sess := &sessionFile{ID: "ses_empty"}
	sess.Time.Created = 1700000000000

	md := renderMarkdown(sess, nil)
	if !strings.Contains(md, "message_count: 0") {
		t.Error("empty messages should show count 0")
	}
}

func TestRenderMarkdownHashStability(t *testing.T) {
	sess := &sessionFile{ID: "ses_hash", Title: "Hash Test"}
	sess.Time.Created = 1700000000000
	msgs := []renderedMessage{
		{Role: "user", Content: "hello", CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
	}

	md1 := renderMarkdown(sess, msgs)
	md2 := renderMarkdown(sess, msgs)

	h1 := sha256.Sum256([]byte(md1))
	h2 := sha256.Sum256([]byte(md2))

	if hex.EncodeToString(h1[:]) != hex.EncodeToString(h2[:]) {
		t.Error("same input should produce same hash")
	}
}

func TestLoadTextParts(t *testing.T) {
	dir := t.TempDir()
	h := &OpenCodeHarvester{sessionDir: dir}

	msgID := "msg_test"
	partDir := filepath.Join(dir, "part", msgID)

	writeJSON(t, filepath.Join(partDir, "prt_001.json"), map[string]any{
		"id": "prt_001", "type": "text", "text": "First part", "messageID": msgID,
	})
	writeJSON(t, filepath.Join(partDir, "prt_002.json"), map[string]any{
		"id": "prt_002", "type": "step-start", "messageID": msgID,
	})
	writeJSON(t, filepath.Join(partDir, "prt_003.json"), map[string]any{
		"id": "prt_003", "type": "text", "text": "Second part", "messageID": msgID,
	})

	text, err := h.loadTextParts(msgID)
	if err != nil {
		t.Fatal(err)
	}
	if text != "First part\n\nSecond part" {
		t.Errorf("got %q, want concatenated text parts", text)
	}
}

func TestLoadTextPartsNoDir(t *testing.T) {
	h := &OpenCodeHarvester{sessionDir: t.TempDir()}

	text, err := h.loadTextParts("msg_nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if text != "" {
		t.Errorf("expected empty string for missing part dir, got %q", text)
	}
}

func TestLoadMessages(t *testing.T) {
	dir := t.TempDir()
	h := &OpenCodeHarvester{
		sessionDir: dir,
		logger:     zerolog.Nop(),
	}

	sesID := "ses_loadtest"
	msgDir := filepath.Join(dir, "message", sesID)

	writeJSON(t, filepath.Join(msgDir, "msg_a.json"), map[string]any{
		"id": "msg_a", "sessionID": sesID, "role": "user",
		"time": map[string]any{"created": 1700000002000},
	})
	writeJSON(t, filepath.Join(msgDir, "msg_b.json"), map[string]any{
		"id": "msg_b", "sessionID": sesID, "role": "assistant",
		"time": map[string]any{"created": 1700000003000},
	})
	writeJSON(t, filepath.Join(msgDir, "msg_c.json"), map[string]any{
		"id": "msg_c", "sessionID": sesID, "role": "system",
		"time": map[string]any{"created": 1700000001000},
	})

	partDirA := filepath.Join(dir, "part", "msg_a")
	writeJSON(t, filepath.Join(partDirA, "prt_01.json"), map[string]any{
		"id": "prt_01", "type": "text", "text": "user question",
	})
	partDirB := filepath.Join(dir, "part", "msg_b")
	writeJSON(t, filepath.Join(partDirB, "prt_02.json"), map[string]any{
		"id": "prt_02", "type": "text", "text": "assistant answer",
	})

	msgs, err := h.loadMessages(sesID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2 (system should be filtered)", len(msgs))
	}
	if msgs[0].Role != "user" {
		t.Errorf("first message should be user (earlier timestamp), got %s", msgs[0].Role)
	}
	if msgs[1].Role != "assistant" {
		t.Errorf("second message should be assistant, got %s", msgs[1].Role)
	}
}

func TestLoadMessagesNoDir(t *testing.T) {
	h := &OpenCodeHarvester{
		sessionDir: t.TempDir(),
		logger:     zerolog.Nop(),
	}

	msgs, err := h.loadMessages("ses_nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages for missing dir, got %d", len(msgs))
	}
}

func TestHarvestAllEmptyDir(t *testing.T) {
	dir := t.TempDir()
	h := &OpenCodeHarvester{
		sessionDir: dir,
		logger:     zerolog.Nop(),
	}

	harvested, skipped, errCount := h.HarvestAll(context.Background(), nil)
	if harvested != 0 || skipped != 0 || errCount != 0 {
		t.Errorf("empty dir: got h=%d s=%d e=%d", harvested, skipped, errCount)
	}
}

func TestHarvestAllNoSessionSubdir(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "session"), 0o755)
	h := &OpenCodeHarvester{
		sessionDir: dir,
		logger:     zerolog.Nop(),
	}

	harvested, skipped, errCount := h.HarvestAll(context.Background(), nil)
	if harvested != 0 || skipped != 0 || errCount != 0 {
		t.Errorf("no session subdir: got h=%d s=%d e=%d", harvested, skipped, errCount)
	}
}

func TestOpenCodeHarvesterDedupHashDeterminism(t *testing.T) {
	sess := &sessionFile{
		ID:        "ses_dedup",
		Title:     "Dedup Test",
		Directory: "/work",
	}
	sess.Time.Created = 1700000000000

	msg := renderedMessage{
		Role:      "user",
		Content:   "Test content",
		CreatedAt: time.UnixMilli(1700000000000),
	}

	md1 := renderMarkdown(sess, []renderedMessage{msg})
	md2 := renderMarkdown(sess, []renderedMessage{msg})

	h1 := sha256.Sum256([]byte(md1))
	h2 := sha256.Sum256([]byte(md2))

	hash1 := hex.EncodeToString(h1[:])
	hash2 := hex.EncodeToString(h2[:])

	if hash1 != hash2 {
		t.Error("identical input must produce identical SHA-256 hash")
	}
}

func TestOpenCodeHarvesterDedupHashChange(t *testing.T) {
	sess := &sessionFile{
		ID:        "ses_dedup",
		Title:     "Dedup Test",
		Directory: "/work",
	}
	sess.Time.Created = 1700000000000

	msg1 := renderedMessage{
		Role:      "user",
		Content:   "Original content",
		CreatedAt: time.UnixMilli(1700000000000),
	}

	msg2 := renderedMessage{
		Role:      "user",
		Content:   "Modified content",
		CreatedAt: time.UnixMilli(1700000000000),
	}

	md1 := renderMarkdown(sess, []renderedMessage{msg1})
	md2 := renderMarkdown(sess, []renderedMessage{msg2})

	h1 := sha256.Sum256([]byte(md1))
	h2 := sha256.Sum256([]byte(md2))

	hash1 := hex.EncodeToString(h1[:])
	hash2 := hex.EncodeToString(h2[:])

	if hash1 == hash2 {
		t.Error("different content must produce different SHA-256 hash")
	}
}
