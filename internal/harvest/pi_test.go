package harvest

import (
	"bufio"
	"bytes"
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

// piHeaderLine returns a JSON line matching Pi's real session-header schema.
func piHeaderLine(id, ts, cwd string) string {
	line, _ := json.Marshal(map[string]any{
		"type":      "session",
		"version":   3,
		"id":        id,
		"timestamp": ts,
		"cwd":       cwd,
	})
	return string(line)
}

// piMessageLine returns a JSON line matching Pi's real message-event schema
// for the given role and content blocks.
func piMessageLine(ts, role string, blocks []map[string]any) string {
	content, _ := json.Marshal(blocks)
	msg, _ := json.Marshal(map[string]any{
		"role":    role,
		"content": json.RawMessage(content),
	})
	line, _ := json.Marshal(map[string]any{
		"type":      "message",
		"timestamp": ts,
		"message":   json.RawMessage(msg),
	})
	return string(line)
}

func piTextBlock(text string) map[string]any {
	return map[string]any{"type": "text", "text": text}
}

func piToolCallBlock(name string, args map[string]any) map[string]any {
	return map[string]any{"type": "toolCall", "name": name, "arguments": args}
}

func piThinkingBlock(text string) map[string]any {
	return map[string]any{"type": "thinking", "text": text}
}

func piImageBlock() map[string]any {
	return map[string]any{"type": "image"}
}

func piNoiseLine(kind string) string {
	line, _ := json.Marshal(map[string]any{"type": kind, "id": "abc"})
	return string(line)
}

func scanEvents(t *testing.T, lines []string) []piEvent {
	t.Helper()
	scanner := bufio.NewScanner(strings.NewReader(strings.Join(lines, "\n")))
	events, err := parsePiEvents(scanner)
	if err != nil {
		t.Fatalf("parsePiEvents: %v", err)
	}
	return events
}

// TestParsePiEvents_AllRolesAndBlockTypes proves every real role (user,
// assistant, toolResult) and every real content-block type (text, toolCall,
// thinking, image) survives parsing — a parser that only handled "text"
// blocks would silently drop about half of real transcript content.
func TestParsePiEvents_AllRolesAndBlockTypes(t *testing.T) {
	lines := []string{
		piMessageLine("2026-01-01T10:00:00Z", "user", []map[string]any{piTextBlock("what does this do?")}),
		piMessageLine("2026-01-01T10:00:01Z", "assistant", []map[string]any{
			piTextBlock("let me check"),
			piToolCallBlock("bash", map[string]any{"command": "ls -la"}),
			piThinkingBlock("the user wants a directory listing"),
			piImageBlock(),
		}),
		piMessageLine("2026-01-01T10:00:02Z", "toolResult", []map[string]any{piTextBlock("total 0\ndrwxr-xr-x")}),
		piNoiseLine("model_change"),
		piNoiseLine("thinking_level_change"),
	}

	events := scanEvents(t, lines)
	if len(events) != 3 {
		t.Fatalf("expected 3 message events (model_change/thinking_level_change filtered out), got %d", len(events))
	}
	if events[0].Message.Role != "user" || events[1].Message.Role != "assistant" || events[2].Message.Role != "toolResult" {
		t.Fatalf("unexpected role order: %q %q %q", events[0].Message.Role, events[1].Message.Role, events[2].Message.Role)
	}

	md := renderPiMarkdown("ses_test", events)
	for _, want := range []string{
		"what does this do?",
		"let me check",
		"Tool: bash",
		"command: ls -la",
		"Thinking:",
		"the user wants a directory listing",
		"[image omitted]",
		"Result: total 0\ndrwxr-xr-x",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("rendered markdown missing %q\n--- got ---\n%s", want, md)
		}
	}
}

// TestParsePiEvents_MalformedLineSkipsOnlyThatLine matches
// ClaudeCodeHarvester.parseJSONLFile's precedent exactly: one bad line is
// skipped, the rest of the session still renders — not discarded wholesale.
func TestParsePiEvents_MalformedLineSkipsOnlyThatLine(t *testing.T) {
	lines := []string{
		piMessageLine("2026-01-01T10:00:00Z", "user", []map[string]any{piTextBlock("first message")}),
		`{"type":"message","timestamp":"2026-01-01T10:00:01Z","message":{"role":"assistant","content":[{"type":"text"` + "\x00" + `TRUNCATED`,
		piMessageLine("2026-01-01T10:00:02Z", "assistant", []map[string]any{piTextBlock("second message survives")}),
	}
	events := scanEvents(t, lines)
	if len(events) != 2 {
		t.Fatalf("expected 2 valid events surrounding the malformed line, got %d", len(events))
	}
	md := renderPiMarkdown("ses_test", events)
	if !strings.Contains(md, "first message") || !strings.Contains(md, "second message survives") {
		t.Errorf("expected both valid messages in rendered output, got:\n%s", md)
	}
}

// TestParsePiEvents_TruncatedLastLine simulates a process killed mid-write:
// the final line has no closing brace/newline. It must be treated as a
// malformed line (skipped), not fatal to the session.
func TestParsePiEvents_TruncatedLastLine(t *testing.T) {
	lines := []string{
		piMessageLine("2026-01-01T10:00:00Z", "user", []map[string]any{piTextBlock("complete turn")}),
		`{"type":"message","timestamp":"2026-01-01T10:00:01Z","message":{"role":"assistant","content":[{"type":"tex`,
	}
	scanner := bufio.NewScanner(strings.NewReader(strings.Join(lines, "\n")))
	events, err := parsePiEvents(scanner)
	if err != nil {
		t.Fatalf("parsePiEvents on truncated last line: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected the one complete event, got %d", len(events))
	}
}

func TestRenderPiMarkdown_HashDeterminism(t *testing.T) {
	events := scanEvents(t, []string{
		piMessageLine("2026-01-01T10:00:00Z", "user", []map[string]any{piTextBlock("hello")}),
		piMessageLine("2026-01-01T10:00:01Z", "assistant", []map[string]any{
			piToolCallBlock("search", map[string]any{"query": "capital of France"}),
		}),
	})

	md1 := renderPiMarkdown("ses_test", events)
	md2 := renderPiMarkdown("ses_test", events)
	h1 := sha256.Sum256([]byte(md1))
	h2 := sha256.Sum256([]byte(md2))
	if hex.EncodeToString(h1[:]) != hex.EncodeToString(h2[:]) {
		t.Error("identical input must produce identical SHA-256 hash")
	}
}

func TestPiSourcePath(t *testing.T) {
	got := piSourcePath("019fdb51-2ce0-7085-a1d1-471be7c19602")
	want := "summary://pi/019fdb51-2ce0-7085-a1d1-471be7c19602"
	if got != want {
		t.Errorf("piSourcePath = %q, want %q", got, want)
	}
}

func TestPiHarvestAllEmptyDir(t *testing.T) {
	dir := t.TempDir()
	h := &PiHarvester{sessionDir: dir, logger: zerolog.Nop()}

	harvested, skipped, errCount := h.HarvestAll(context.Background(), nil)
	if harvested != 0 || skipped != 0 || errCount != 0 {
		t.Errorf("empty dir: got h=%d s=%d e=%d", harvested, skipped, errCount)
	}
}

func TestPiHarvestAllNonExistentDir(t *testing.T) {
	h := &PiHarvester{sessionDir: "/nonexistent/path/pi-sessions", logger: zerolog.Nop()}

	harvested, skipped, errCount := h.HarvestAll(context.Background(), nil)
	if harvested != 0 || skipped != 0 || errCount != 0 {
		t.Errorf("nonexistent dir: got h=%d s=%d e=%d", harvested, skipped, errCount)
	}
}

func TestPiHarvestAllSkipsNonJSONL(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not a session"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "data.json"), []byte("{}"), 0o644)

	h := &PiHarvester{sessionDir: dir, logger: zerolog.Nop()}

	harvested, skipped, errCount := h.HarvestAll(context.Background(), nil)
	if harvested != 0 || skipped != 0 || errCount != 0 {
		t.Errorf("non-JSONL files: got h=%d s=%d e=%d", harvested, skipped, errCount)
	}
}

// TestPiHarvestAll_MalformedHeaderCountsAsError proves a session file whose
// FIRST (header) line is unparseable is counted as an error — unlike a
// malformed line elsewhere in the file, there is no session ID to key a
// partial harvest on, so the whole session file is skipped-with-error rather
// than silently ignored.
func TestPiHarvestAll_MalformedHeaderCountsAsError(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	buf.WriteString("not json at all\n")
	if err := os.WriteFile(filepath.Join(dir, "bad-header.jsonl"), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	h := &PiHarvester{sessionDir: dir, logger: zerolog.Nop()}
	harvested, skipped, errCount := h.HarvestAll(context.Background(), nil)
	if harvested != 0 || skipped != 0 || errCount != 1 {
		t.Errorf("malformed header: got h=%d s=%d e=%d, want h=0 s=0 e=1", harvested, skipped, errCount)
	}
}
