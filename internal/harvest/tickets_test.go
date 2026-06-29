package harvest

import (
	"reflect"
	"testing"
)

func newExtractorOrFail(t *testing.T, patterns []string) *TicketExtractor {
	t.Helper()
	te, err := NewTicketExtractor(patterns)
	if err != nil {
		t.Fatalf("NewTicketExtractor(%v) error: %v", patterns, err)
	}
	return te
}

func TestExtract_ContentMatch(t *testing.T) {
	te := newExtractorOrFail(t, nil)
	content := "Working on DEV-1234 and fixing PROJ-99 as discussed."
	got := te.Extract(content, "", nil)
	want := []string{"DEV-1234", "PROJ-99"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Extract content match: got %v, want %v", got, want)
	}
}

func TestExtract_BranchMatch(t *testing.T) {
	te := newExtractorOrFail(t, nil)
	// No ticket in content; ticket derived entirely from branch name.
	got := te.Extract("some session content without any ticket id", "feat/DEV-4706-my-feature", nil)
	want := []string{"DEV-4706"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Extract branch match: got %v, want %v", got, want)
	}
}

func TestExtract_ParentInheritance(t *testing.T) {
	te := newExtractorOrFail(t, nil)
	// Child content mentions no ticket; parent tag carries DEV-999.
	got := te.Extract("no ticket here", "", []string{"ticket:DEV-999", "summary", "claude"})
	want := []string{"DEV-999"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Extract parent inheritance: got %v, want %v", got, want)
	}
}

func TestExtract_SubagentNoContent(t *testing.T) {
	te := newExtractorOrFail(t, nil)
	// Subagent: empty content, no branch, parent has ticket.
	got := te.Extract("", "", []string{"ticket:PROJ-42"})
	want := []string{"PROJ-42"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Extract subagent no content: got %v, want %v", got, want)
	}
}

func TestExtract_Deduplicate(t *testing.T) {
	te := newExtractorOrFail(t, nil)
	// Same ticket appears in content and branch — must appear once.
	got := te.Extract("fixing DEV-100 today", "feat/DEV-100-fix", nil)
	want := []string{"DEV-100"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Extract deduplicate: got %v, want %v", got, want)
	}
}

func TestExtract_CustomPattern(t *testing.T) {
	te := newExtractorOrFail(t, []string{`#\d+`})
	got := te.Extract("see issue #42 for context", "", nil)
	want := []string{"#42"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Extract custom pattern: got %v, want %v", got, want)
	}
}

func TestExtract_MarkdownHeadingsNotMatchedByHash(t *testing.T) {
	te := newExtractorOrFail(t, nil)
	// "#Introduction" or "# Heading" must not produce a ticket.
	content := "# Introduction\n## Section\nSee #42 in the body."
	got := te.Extract(content, "", nil)
	// Only #42 from the body should match; headings should be excluded.
	want := []string{"#42"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Extract headings excluded: got %v, want %v", got, want)
	}
}

func TestExtract_Empty(t *testing.T) {
	te := newExtractorOrFail(t, nil)
	got := te.Extract("", "", nil)
	if got != nil {
		t.Errorf("Extract empty: got %v, want nil", got)
	}
}

func TestAsTags(t *testing.T) {
	te := newExtractorOrFail(t, nil)
	got := te.AsTags([]string{"DEV-4706", "PROJ-42"})
	want := []string{"ticket:DEV-4706", "ticket:PROJ-42"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("AsTags: got %v, want %v", got, want)
	}
}

func TestNewTicketExtractor_InvalidPattern(t *testing.T) {
	_, err := NewTicketExtractor([]string{`[invalid`})
	if err == nil {
		t.Error("NewTicketExtractor with invalid pattern: expected error, got nil")
	}
}
