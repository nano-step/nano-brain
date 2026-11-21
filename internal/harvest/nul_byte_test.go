package harvest

import (
	"encoding/json"
	"strings"
	"testing"
)

// Postgres TEXT rejects 0x00, so no renderer may let a NUL from tool output
// through to the document that gets persisted (#648).
func TestRenderers_StripNULBytes(t *testing.T) {
	const text = "before\x00after"

	claudeContent, _ := json.Marshal(text)
	piContent, _ := json.Marshal([]map[string]any{{"type": "text", "text": text}})

	var claudeMsg claudeCodeMessage
	claudeMsg.Type = "user"
	claudeMsg.Message.Role = "user"
	claudeMsg.Message.Content = claudeContent

	tests := []struct {
		name string
		md   string
	}{
		{"engine", RenderMarkdown(NormalizedSession{SessionID: "s", Messages: []NormalizedMessage{{Role: "user", Content: text}}})},
		{"opencode sqlite", renderSQLiteMarkdown(SqSession{ID: "s"}, []sqMessage{{role: "user", content: text}})},
		{"opencode json", renderMarkdown(&sessionFile{ID: "s"}, []renderedMessage{{Role: "user", Content: text}})},
		{"claude code", renderClaudeCodeMarkdown("s", []claudeCodeMessage{claudeMsg})},
		{"pi", renderPiMarkdown("s", []piEvent{{Type: "message", Message: piMessage{Role: "toolResult", Content: piContent}}})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if strings.Contains(tt.md, "\x00") {
				t.Errorf("rendered markdown still contains a NUL byte")
			}
			if !strings.Contains(tt.md, "beforeafter") {
				t.Errorf("rendered markdown lost the text around the NUL byte: %q", tt.md)
			}
		})
	}
}
