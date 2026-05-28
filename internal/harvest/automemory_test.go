package harvest

import (
	"strings"
	"testing"
)

func containsStr2(s, sub string) bool {
	return strings.Contains(s, sub)
}

func TestExtractMemories_DecisionLine(t *testing.T) {
	content := "## Session\n\nDecision: Use PostgreSQL for storage\nDecision: Avoid CGO\n"
	memories := extractMemories(content)
	if len(memories) == 0 {
		t.Fatal("expected memories, got 0")
	}
	found := false
	for _, m := range memories {
		if m.kind == kindDecision && containsStr2(m.content, "PostgreSQL") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected decision about PostgreSQL, got: %+v", memories)
	}
}

func TestExtractMemories_LessonLine(t *testing.T) {
	content := "Lesson: Always write tests before implementation\n"
	memories := extractMemories(content)
	if len(memories) == 0 {
		t.Fatal("expected memories, got 0")
	}
	if memories[0].kind != kindLesson {
		t.Errorf("expected lesson kind, got %s", memories[0].kind)
	}
}

func TestExtractMemories_Empty(t *testing.T) {
	content := "## Regular session\n\nJust talking about stuff.\n"
	memories := extractMemories(content)
	if len(memories) != 0 {
		t.Errorf("expected 0 memories from plain content, got %d: %+v", len(memories), memories)
	}
}

func TestExtractMemories_Dedup(t *testing.T) {
	content := "Decision: Use Go for this project\nDecision: Use Go for this project\n"
	memories := extractMemories(content)
	if len(memories) != 1 {
		t.Errorf("expected 1 deduplicated memory, got %d", len(memories))
	}
}

func TestExtractMemories_DecisionHeading(t *testing.T) {
	content := "## Key Decisions\n\n- Use PostgreSQL\n- Avoid CGO\n\n## Other Section\n\nUnrelated content.\n"
	memories := extractMemories(content)
	if len(memories) == 0 {
		t.Fatal("expected memories from decision heading section")
	}
	for _, m := range memories {
		if m.kind != kindDecision {
			t.Errorf("expected decision kind, got %s", m.kind)
		}
	}
}

func TestTitleFromContent_Short(t *testing.T) {
	title := titleFromContent("Short decision")
	if title != "Short decision" {
		t.Errorf("unexpected title: %s", title)
	}
}

func TestTitleFromContent_Long(t *testing.T) {
	long := "This is a very long decision that exceeds eighty characters in total and should be truncated properly"
	title := titleFromContent(long)
	if len(title) > 80 {
		t.Errorf("title too long: %d chars", len(title))
	}
	if title[len(title)-3:] != "..." {
		t.Errorf("title should end with ..., got: %s", title)
	}
}
