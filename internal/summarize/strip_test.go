package summarize

import (
	"strings"
	"testing"
)

func TestStripOpenCode_RemovesSystemPrompt(t *testing.T) {
	input := "---\nsession_id: abc\n---\n\n## system (2026-05-26T10:00:00Z)\n\nYou are OpenCode, the best coding agent.\nSkills list: skill1, skill2, skill3...\nMore system prompt content here that is very long.\n\n## user (2026-05-26T10:01:00Z)\n\nhello\n"

	got := StripOpenCode(input)

	if strings.Contains(got, "best coding agent") {
		t.Error("system prompt content should be removed")
	}
	if strings.Contains(got, "Skills list") {
		t.Error("system prompt content should be removed")
	}
	if !strings.Contains(got, "## user") {
		t.Error("user section should be preserved")
	}
	if !strings.Contains(got, "hello") {
		t.Error("user message should be preserved")
	}
}

func TestStripOpenCode_KeepsShortToolOutput(t *testing.T) {
	input := "## assistant (2026-05-26T10:00:00Z)\n\n**Tool result** (read):\nshort output here\n\n## user (2026-05-26T10:01:00Z)\n\nthanks\n"

	got := StripOpenCode(input)

	if !strings.Contains(got, "short output here") {
		t.Error("short tool output should be preserved")
	}
}

func TestStripOpenCode_ReplacesLargeToolOutput(t *testing.T) {
	longBody := strings.Repeat("line of file content number xyz\n", 20)
	input := "## assistant (2026-05-26T10:00:00Z)\n\n**Tool result** (read):\n```\n" + longBody + "```\n\n## user (2026-05-26T10:01:00Z)\n\nthanks\n"

	got := StripOpenCode(input)

	if strings.Contains(got, "line of file content") {
		t.Error("large tool output body should be replaced")
	}
	if !strings.Contains(got, "[tool: read,") {
		t.Error("should contain tool placeholder with name")
	}
	if !strings.Contains(got, "lines]") {
		t.Error("should contain line count in placeholder")
	}
}

func TestStripOpenCode_CollapsesLargeCodeBlock(t *testing.T) {
	var lines []string
	for i := 0; i < 30; i++ {
		lines = append(lines, "    fmt.Println(\"hello\")")
	}
	codeContent := strings.Join(lines, "\n") + "\n"
	input := "## assistant (2026-05-26T10:00:00Z)\n\nHere is the code:\n\n```go\n" + codeContent + "```\n\n## user (2026-05-26T10:01:00Z)\n\nthanks\n"

	got := StripOpenCode(input)

	if strings.Contains(got, "fmt.Println") {
		t.Error("large code block content should be collapsed")
	}
	if !strings.Contains(got, "[code block: 30 lines, go]") {
		t.Errorf("should contain collapse placeholder, got:\n%s", got)
	}
}

func TestStripOpenCode_KeepsSmallCodeBlock(t *testing.T) {
	var lines []string
	for i := 0; i < 10; i++ {
		lines = append(lines, "    fmt.Println(\"hello\")")
	}
	codeContent := strings.Join(lines, "\n") + "\n"
	input := "```go\n" + codeContent + "```\n"

	got := StripOpenCode(input)

	if !strings.Contains(got, "fmt.Println") {
		t.Error("small code block content should be preserved")
	}
	if strings.Contains(got, "[code block:") {
		t.Error("small code block should not be collapsed")
	}
}

func TestStripOpenCode_CollapsesUnlabeledCodeBlock(t *testing.T) {
	var lines []string
	for i := 0; i < 25; i++ {
		lines = append(lines, "some content line")
	}
	codeContent := strings.Join(lines, "\n") + "\n"
	input := "```\n" + codeContent + "```\n"

	got := StripOpenCode(input)

	if strings.Contains(got, "some content line") {
		t.Error("large unlabeled code block should be collapsed")
	}
	if !strings.Contains(got, "[code block: 25 lines, unknown]") {
		t.Errorf("should contain unknown language placeholder, got:\n%s", got)
	}
}

func TestStripOpenCode_DedupesRepeatedErrors(t *testing.T) {
	input := "Error: connection refused\nError: connection refused\nError: connection refused\nError: connection refused\nError: timeout\n"

	got := StripOpenCode(input)

	if strings.Count(got, "Error: connection refused") != 1 {
		t.Errorf("should have exactly 1 occurrence of the repeated error, got:\n%s", got)
	}
	if !strings.Contains(got, "(repeated 3 more times)") {
		t.Errorf("should have repeat count annotation, got:\n%s", got)
	}
	if !strings.Contains(got, "Error: timeout") {
		t.Error("unique errors should be preserved")
	}
}

func TestStripOpenCode_RemovesBase64(t *testing.T) {
	b64 := strings.Repeat("ABCDEFGHIJKLMNOPabcdefghijklmnop0123456789+/", 10) + "=="
	input := "Here is an image: " + b64 + "\nAnd more text.\n"

	got := StripOpenCode(input)

	if strings.Contains(got, "ABCDEFGHIJKLMNOP") {
		t.Error("base64 data should be removed")
	}
	if !strings.Contains(got, "[base64 data removed]") {
		t.Error("should contain base64 removal placeholder")
	}
	if !strings.Contains(got, "And more text.") {
		t.Error("surrounding text should be preserved")
	}
}

func TestStripOpenCode_PreservesUserAndAssistantText(t *testing.T) {
	input := "---\nsession_id: abc\n---\n\n## user (2026-05-26T10:00:00Z)\n\nhow do I fix this bug?\n\n## assistant (2026-05-26T10:01:00Z)\n\nLet me check the file. The issue is in the error handling on line 42.\n"

	got := StripOpenCode(input)

	if got != input {
		t.Errorf("plain user/assistant text should be unchanged.\nwant:\n%s\ngot:\n%s", input, got)
	}
}

func TestStripOpenCode_PreservesFilePaths(t *testing.T) {
	input := "## assistant (2026-05-26T10:00:00Z)\n\nThe file `/Users/foo/bar.go` has the bug at `internal/server/handler.go:42`.\n"

	got := StripOpenCode(input)

	if !strings.Contains(got, "/Users/foo/bar.go") {
		t.Error("file paths should be preserved")
	}
	if !strings.Contains(got, "internal/server/handler.go:42") {
		t.Error("file path references should be preserved")
	}
}

func TestStripClaude_LongCommand(t *testing.T) {
	var cmdLines []string
	for i := 0; i < 8; i++ {
		cmdLines = append(cmdLines, "  some long command argument line")
	}
	cmdBody := strings.Join(cmdLines, "\n") + "\n"
	input := "## assistant (2026-05-26T10:00:00Z)\n\n**tool_use** Bash:\n```\n" + cmdBody + "```\n\n## human (2026-05-26T10:01:00Z)\n\nok\n"

	got := StripClaude(input)

	if strings.Contains(got, "some long command argument line") {
		t.Error("long tool_use command body should be replaced")
	}
	if !strings.Contains(got, "[command:") {
		t.Errorf("should contain command placeholder, got:\n%s", got)
	}
}

func TestStripClaude_LongToolResult(t *testing.T) {
	longOutput := strings.Repeat("output line with various content data\n", 15)
	input := "## tool_result (2026-05-26T10:00:00Z)\n\n" + longOutput + "\n## human (2026-05-26T10:01:00Z)\n\nthanks\n"

	got := StripClaude(input)

	if strings.Contains(got, "output line with various") {
		t.Error("long tool_result body should be replaced")
	}
	if !strings.Contains(got, "[tool: tool_result,") {
		t.Errorf("should contain tool_result placeholder, got:\n%s", got)
	}
}

func TestStripClaude_ShortToolResult(t *testing.T) {
	input := "## tool_result (2026-05-26T10:00:00Z)\n\nshort output\n\n## human (2026-05-26T10:01:00Z)\n\nthanks\n"

	got := StripClaude(input)

	if !strings.Contains(got, "short output") {
		t.Error("short tool_result should be preserved")
	}
}

func TestStripClaude_RemovesBase64(t *testing.T) {
	b64 := strings.Repeat("ABCDEFGHIJKLMNOPabcdefghijklmnop0123456789+/", 10) + "=="
	input := "## human (2026-05-26T10:00:00Z)\n\nHere is data: " + b64 + "\n"

	got := StripClaude(input)

	if strings.Contains(got, "ABCDEFGHIJKLMNOP") {
		t.Error("base64 data should be removed")
	}
	if !strings.Contains(got, "[base64 data removed]") {
		t.Error("should contain base64 removal placeholder")
	}
}
