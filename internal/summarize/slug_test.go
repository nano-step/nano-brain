package summarize

import (
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"simple title", "Oracle Verify Epic 9", "oracle-verify-epic-9"},
		{"with at mention", "Oracle Verify Epic 9 (@oracle)", "oracle-verify-epic-9-oracle"},
		{"special chars only", "@#$%", "untitled-session"},
		{"empty", "", "untitled-session"},
		{"single space", " ", "untitled-session"},
		{"already slug", "foo-bar-baz", "foo-bar-baz"},
		{"collapse dashes", "a---b", "a-b"},
		{"long collapse", "x------------y", "x-y"},
		{"leading dash", "-foo-bar", "foo-bar"},
		{"trailing dash", "foo-bar-", "foo-bar"},
		{"both ends", "---foo---", "foo"},
		{"multiple punctuation", "Foo!!! @bar ###", "foo-bar"},
		{"snake to dash", "foo_bar_baz", "foo-bar-baz"},
		{"numbers preserved", "Story 9.4 — implement REST", "story-9-4-implement-rest"},
		{"vietnamese diacritics", "Chào thế giới", "ch-o-th-gi-i"},
		{"only digits", "12345", "12345"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := slugify(c.input)
			if got != c.want {
				t.Errorf("slugify(%q) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}

func TestSlugify_LengthLimit(t *testing.T) {
	long := strings.Repeat("a", 200)
	got := slugify(long)
	if len(got) > maxSlugLen {
		t.Errorf("slugify produced %d chars, max is %d", len(got), maxSlugLen)
	}
	if got != strings.Repeat("a", maxSlugLen) {
		t.Errorf("expected 80 'a's, got %q", got)
	}
}

func TestSlugify_TrimsTrailingDashAfterTruncation(t *testing.T) {
	input := strings.Repeat("a", 79) + "----extra"
	got := slugify(input)
	if strings.HasSuffix(got, "-") {
		t.Errorf("slugify result %q ends with dash after truncation", got)
	}
	if len(got) > maxSlugLen {
		t.Errorf("slugify result length %d > %d", len(got), maxSlugLen)
	}
}

func TestSlugify_NonASCIIOnly(t *testing.T) {
	got := slugify("日本語のみ")
	if got != defaultSlugStub {
		t.Errorf("slugify(%q) = %q, want %q (all-non-alphanum should fallback)", "日本語のみ", got, defaultSlugStub)
	}
}

func TestSlugify_Idempotent(t *testing.T) {
	once := slugify("Oracle Verify Epic 9")
	twice := slugify(once)
	if once != twice {
		t.Errorf("slugify not idempotent: once=%q, twice=%q", once, twice)
	}
}

func TestCollapseDashes(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"a", "a"},
		{"a-b", "a-b"},
		{"a--b", "a-b"},
		{"a---b", "a-b"},
		{"---", "-"},
		{"", ""},
		{"abc", "abc"},
	}
	for _, c := range cases {
		got := collapseDashes(c.in)
		if got != c.want {
			t.Errorf("collapseDashes(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
