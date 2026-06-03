package search_test

import (
	"testing"
	"unicode/utf8"

	"github.com/nano-brain/nano-brain/internal/search"
)

func TestTruncateSnippet(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxChars int
		want     string
		checkUTF bool
	}{
		{
			name:     "empty string",
			input:    "",
			maxChars: 10,
			want:     "",
			checkUTF: true,
		},
		{
			name:     "short string unchanged",
			input:    "hello",
			maxChars: 10,
			want:     "hello",
			checkUTF: true,
		},
		{
			name:     "exact boundary ASCII",
			input:    "hello",
			maxChars: 5,
			want:     "hello",
			checkUTF: true,
		},
		{
			name:     "truncate ASCII to 3 runes",
			input:    "hello world",
			maxChars: 3,
			want:     "hel",
			checkUTF: true,
		},
		{
			name:     "maxChars zero returns empty",
			input:    "hello",
			maxChars: 0,
			want:     "",
			checkUTF: true,
		},
		{
			name:     "maxChars negative returns empty",
			input:    "hello",
			maxChars: -1,
			want:     "",
			checkUTF: true,
		},
		{
			name:     "unicode accented character at boundary",
			input:    "café",
			maxChars: 3,
			want:     "caf",
			checkUTF: true,
		},
		{
			name:     "unicode accented character exact",
			input:    "café",
			maxChars: 4,
			want:     "café",
			checkUTF: true,
		},
		{
			name:     "chinese characters truncate",
			input:    "你好世界",
			maxChars: 2,
			want:     "你好",
			checkUTF: true,
		},
		{
			name:     "mixed ASCII and unicode",
			input:    "hello世界",
			maxChars: 7,
			want:     "hello世界",
			checkUTF: true,
		},
		{
			name:     "mixed ASCII and unicode truncate mid-unicode",
			input:    "hello世界",
			maxChars: 6,
			want:     "hello世",
			checkUTF: true,
		},
		{
			name:     "emoji boundary",
			input:    "hello🎉world",
			maxChars: 6,
			want:     "hello🎉",
			checkUTF: true,
		},
		{
			name:     "emoji truncate before",
			input:    "hello🎉world",
			maxChars: 5,
			want:     "hello",
			checkUTF: true,
		},
		{
			name:     "multiple emoji",
			input:    "🎉🎊🎈",
			maxChars: 2,
			want:     "🎉🎊",
			checkUTF: true,
		},
		{
			name:     "very long ASCII truncate",
			input:    "abcdefghijklmnopqrstuvwxyz",
			maxChars: 10,
			want:     "abcdefghij",
			checkUTF: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := search.TruncateSnippet(tt.input, tt.maxChars)
			if got != tt.want {
				t.Errorf("TruncateSnippet(%q, %d) = %q, want %q", tt.input, tt.maxChars, got, tt.want)
			}
			if tt.checkUTF && !utf8.ValidString(got) {
				t.Errorf("TruncateSnippet(%q, %d) returned invalid UTF-8: %q", tt.input, tt.maxChars, got)
			}
		})
	}
}

func TestTruncateSnippetRuneCount(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		maxChars   int
		wantRunes  int
	}{
		{
			name:       "return value has exactly maxChars runes when truncated",
			input:      "abcdefghij",
			maxChars:   5,
			wantRunes:  5,
		},
		{
			name:       "unicode: return value has exactly maxChars runes",
			input:      "你好世界朋友",
			maxChars:   3,
			wantRunes:  3,
		},
		{
			name:       "mixed: return value respects maxChars runes",
			input:      "hello世界world",
			maxChars:   7,
			wantRunes:  7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := search.TruncateSnippet(tt.input, tt.maxChars)
			gotRunes := utf8.RuneCountInString(got)
			if gotRunes != tt.wantRunes {
				t.Errorf("TruncateSnippet(%q, %d) returned %d runes, want %d (value: %q)", 
					tt.input, tt.maxChars, gotRunes, tt.wantRunes, got)
			}
		})
	}
}
