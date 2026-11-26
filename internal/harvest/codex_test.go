package harvest

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

// codexMetaLine returns a session_meta rollout line matching the real Codex
// schema (payload carries id/session_id/cwd/timestamp).
func codexMetaLine(id, cwd, ts string) string {
	line, _ := json.Marshal(map[string]any{
		"timestamp": ts,
		"type":      "session_meta",
		"payload": map[string]any{
			"id":         id,
			"session_id": id,
			"cwd":        cwd,
			"timestamp":  ts,
		},
	})
	return string(line)
}

func codexItemLine(ts, payloadType string, payload any) string {
	raw, _ := json.Marshal(payload)
	line, _ := json.Marshal(map[string]any{
		"timestamp": ts,
		"type":      "response_item",
		"payload":   json.RawMessage(raw),
	})
	return string(line)
}

func codexRecordLine(ts, recordType string, payload any) string {
	raw, _ := json.Marshal(payload)
	line, _ := json.Marshal(map[string]any{
		"timestamp": ts,
		"type":      recordType,
		"payload":   json.RawMessage(raw),
	})
	return string(line)
}

func codexWriteRollout(t *testing.T, dir, name string, lines ...string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCollectCodexRollouts_NestedAndArchived(t *testing.T) {
	root := t.TempDir()
	sessions := filepath.Join(root, "sessions")
	archived := filepath.Join(root, "archived_sessions")

	codexWriteRollout(t, sessions, "2026/11/20/rollout-a.jsonl", codexMetaLine("a", "/w", "2026-11-20T00:00:00Z"))
	codexWriteRollout(t, sessions, "2026/11/20/rollout-b.jsonl.zst", "{}")
	codexWriteRollout(t, sessions, "2026/11/20/notes.txt", "x")
	codexWriteRollout(t, archived, "rollout-c.jsonl", codexMetaLine("c", "/w", "2026-11-01T00:00:00Z"))
	codexWriteRollout(t, archived, "rollout-d.jsonl.zst", "{}")

	files, err := collectCodexRollouts(sessions, archived)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 rollout files, got %d: %v", len(files), files)
	}
	for _, f := range files {
		if strings.HasSuffix(f, ".zst") || strings.HasSuffix(f, ".txt") {
			t.Fatalf("unexpected file collected: %s", f)
		}
	}
}

func TestCollectCodexRollouts_MissingDirs(t *testing.T) {
	files, err := collectCodexRollouts(filepath.Join(t.TempDir(), "nope"), filepath.Join(t.TempDir(), "alsonope"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("expected 0 files, got %v", files)
	}
}

func TestCodexWorkspaceCtx_Resolve(t *testing.T) {
	ws := []sqlc.Workspace{
		{Hash: "hash-root", Path: "/data/ws"},
		{Hash: "hash-nested", Path: "/data/ws/nested"},
	}
	ctx := newCodexWorkspaceCtx(ws)

	cases := []struct {
		name string
		cwd  string
		want string
		ok   bool
	}{
		{"exact", "/data/ws", "hash-root", true},
		{"exact trailing slash", "/data/ws/", "hash-root", true},
		{"subdirectory picks longest ancestor", "/data/ws/nested/deep", "hash-nested", true},
		{"unregistered", "/elsewhere", "", false},
		{"empty", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ctx.resolve(tc.cwd)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("resolve(%q) = (%q,%v), want (%q,%v)", tc.cwd, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestScanCodexSessionMeta(t *testing.T) {
	t.Run("meta on first line", func(t *testing.T) {
		content := codexMetaLine("sess-1", "/w", "2026-01-01T00:00:00Z") + "\n" + codexRecordLine("t", "event_msg", map[string]any{"type": "task_started"}) + "\n"
		sc := bufio.NewScanner(strings.NewReader(content))
		meta, found, err := scanCodexSessionMeta(sc)
		if err != nil || !found {
			t.Fatalf("found=%v err=%v", found, err)
		}
		if meta.ID != "sess-1" || meta.Cwd != "/w" {
			t.Fatalf("meta = %+v", meta)
		}
	})

	t.Run("meta after other records within scan window", func(t *testing.T) {
		var lines []string
		for i := 0; i < 10; i++ {
			lines = append(lines, codexRecordLine("t", "event_msg", map[string]any{"type": "x"}))
		}
		lines = append(lines, codexMetaLine("sess-2", "/w2", "2026-01-01T00:00:00Z"))
		sc := bufio.NewScanner(strings.NewReader(strings.Join(lines, "\n") + "\n"))
		meta, found, err := scanCodexSessionMeta(sc)
		if err != nil || !found || meta.ID != "sess-2" {
			t.Fatalf("found=%v meta=%+v err=%v", found, meta, err)
		}
	})

	t.Run("meta beyond scan window not found", func(t *testing.T) {
		var lines []string
		for i := 0; i < codexMetaScanLines+5; i++ {
			lines = append(lines, codexRecordLine("t", "event_msg", map[string]any{"type": "x"}))
		}
		sc := bufio.NewScanner(strings.NewReader(strings.Join(lines, "\n") + "\n"))
		_, found, err := scanCodexSessionMeta(sc)
		if err != nil {
			t.Fatal(err)
		}
		if found {
			t.Fatal("expected meta not found beyond scan window")
		}
	})

	t.Run("session_id fallback when id empty", func(t *testing.T) {
		line, _ := json.Marshal(map[string]any{
			"type": "session_meta",
			"payload": map[string]any{
				"id": "", "session_id": "sess-fallback", "cwd": "/w",
			},
		})
		sc := bufio.NewScanner(strings.NewReader(string(line) + "\n"))
		meta, found, err := scanCodexSessionMeta(sc)
		if err != nil || !found || meta.ID != "sess-fallback" {
			t.Fatalf("found=%v meta=%+v err=%v", found, meta, err)
		}
	})

	t.Run("malformed lines skipped", func(t *testing.T) {
		content := "not json\n" + codexMetaLine("sess-3", "/w", "t") + "\n{bad\n"
		sc := bufio.NewScanner(strings.NewReader(content))
		meta, found, err := scanCodexSessionMeta(sc)
		if err != nil || !found || meta.ID != "sess-3" {
			t.Fatalf("found=%v meta=%+v err=%v", found, meta, err)
		}
	})
}

func TestParseCodexResponseItems(t *testing.T) {
	lines := []string{
		codexItemLine("2026-01-01T00:00:01Z", "message", map[string]any{
			"type": "message", "role": "user",
			"content": []map[string]any{{"type": "input_text", "text": "fix the bug"}},
		}),
		codexItemLine("2026-01-01T00:00:02Z", "message", map[string]any{
			"type": "message", "role": "assistant",
			"content": []map[string]any{{"type": "output_text", "text": "on it"}},
		}),
		codexItemLine("2026-01-01T00:00:03Z", "function_call", map[string]any{
			"type": "function_call", "call_id": "call-1", "name": "shell",
			"arguments": `{"cmd":"ls"}`,
		}),
		codexItemLine("2026-01-01T00:00:04Z", "function_call_output", map[string]any{
			"type": "function_call_output", "call_id": "call-1", "output": "file1.go\nfile2.go",
		}),
		codexItemLine("2026-01-01T00:00:05Z", "custom_tool_call", map[string]any{
			"type": "custom_tool_call", "call_id": "call-2", "name": "exec",
			"input": "text(tools)",
		}),
		codexItemLine("2026-01-01T00:00:06Z", "custom_tool_call_output", map[string]any{
			"type": "custom_tool_call_output", "call_id": "call-2",
			"output": []map[string]any{{"type": "input_text", "text": "done"}},
		}),
		codexItemLine("2026-01-01T00:00:07Z", "web_search_call", map[string]any{
			"type": "web_search_call", "call_id": "ws-1",
			"action": map[string]any{"type": "search", "query": "pgvector"},
		}),
		// Excluded payload types:
		codexItemLine("t", "message", map[string]any{
			"type": "message", "role": "developer",
			"content": []map[string]any{{"type": "input_text", "text": "system prompt"}},
		}),
		codexItemLine("t", "message", map[string]any{
			"type": "message", "role": "agent_message",
			"content": []map[string]any{{"type": "output_text", "text": "subagent"}},
		}),
		codexItemLine("t", "reasoning", map[string]any{
			"type": "reasoning", "encrypted_content": "gAAAA...",
		}),
		codexItemLine("t", "additional_tools", map[string]any{"type": "additional_tools"}),
		// Non-response_item records:
		codexRecordLine("t", "event_msg", map[string]any{"type": "task_started"}),
		codexRecordLine("t", "token_usage_record", map[string]any{"total_tokens": 1}),
		// Malformed + truncated:
		"{not json",
		`{"timestamp":"t","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"last`,
	}

	sc := bufio.NewScanner(strings.NewReader(strings.Join(lines, "\n") + "\n"))
	items, err := parseCodexResponseItems(sc)
	if err != nil {
		t.Fatal(err)
	}

	var kinds []string
	for _, it := range items {
		kinds = append(kinds, it.Kind+":"+it.Name)
	}
	want := []string{
		"user:", "assistant:", "tool:shell", "tool:exec", "tool:web_search",
	}
	if strings.Join(kinds, ",") != strings.Join(want, ",") {
		t.Fatalf("kinds = %v, want %v", kinds, want)
	}

	// Tool outputs paired into their Tool: blocks.
	if !strings.Contains(items[2].Text, "Result: file1.go") {
		t.Fatalf("function_call output not paired: %q", items[2].Text)
	}
	if !strings.Contains(items[3].Text, "Result: done") {
		t.Fatalf("custom_tool_call output not paired: %q", items[3].Text)
	}

	// No excluded content leaked.
	for _, it := range items {
		for _, banned := range []string{"system prompt", "subagent", "gAAAA"} {
			if strings.Contains(it.Text, banned) {
				t.Fatalf("excluded content %q leaked into item %+v", banned, it)
			}
		}
	}
}

func TestParseCodexResponseItems_UnpairedOutputStandalone(t *testing.T) {
	lines := []string{
		codexItemLine("t", "function_call_output", map[string]any{
			"type": "function_call_output", "call_id": "orphan", "output": "orphan result",
		}),
	}
	sc := bufio.NewScanner(strings.NewReader(strings.Join(lines, "\n") + "\n"))
	items, err := parseCodexResponseItems(sc)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Kind != "result" || items[0].Text != "orphan result" {
		t.Fatalf("items = %+v", items)
	}
}

func TestRenderCodexMarkdown(t *testing.T) {
	meta := codexSessionMeta{ID: "sess-x", Cwd: "/w", Timestamp: "2026-01-01T00:00:00Z"}
	items := []codexItem{
		{Kind: "user", Role: "user", Timestamp: "2026-01-01T00:00:01Z", Text: "hello"},
		{Kind: "assistant", Role: "assistant", Timestamp: "2026-01-01T00:00:02Z", Text: "hi there"},
		{Kind: "tool", Name: "shell", Timestamp: "t", Text: `{"cmd":"ls"}` + "\nResult: out.txt"},
	}
	md := renderCodexMarkdown("sess-x", meta, items)

	for _, want := range []string{
		"session_id: sess-x", "source: codex", "cwd: /w",
		"## user (2026-01-01T00:00:01Z)", "hello",
		"## assistant (2026-01-01T00:00:02Z)", "hi there",
		"Tool: shell", `{"cmd":"ls"}`, "Result: out.txt",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("rendered markdown missing %q:\n%s", want, md)
		}
	}
}

func TestCodexSourcePath(t *testing.T) {
	if got := codexSourcePath("abc-123"); got != "summary://codex/abc-123" {
		t.Fatalf("codexSourcePath = %q", got)
	}
}

func TestCodexHarvestAllNoDirs(t *testing.T) {
	h := NewCodexHarvester(nil, zerolog.Nop(), filepath.Join(t.TempDir(), "empty-home"))
	harvested, skipped, errs := h.HarvestAll(t.Context(), nil)
	if harvested != 0 || skipped != 0 || errs != 0 {
		t.Fatalf("got (%d,%d,%d), want (0,0,0)", harvested, skipped, errs)
	}
}

func TestCodexHarvestAllEmptySessionsDir(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "sessions"), 0o755); err != nil {
		t.Fatal(err)
	}
	h := NewCodexHarvester(nil, zerolog.Nop(), home)
	harvested, skipped, errs := h.HarvestAll(t.Context(), nil)
	if harvested != 0 || skipped != 0 || errs != 0 {
		t.Fatalf("got (%d,%d,%d), want (0,0,0)", harvested, skipped, errs)
	}
}
