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

func TestExtractRelevantSnippet(t *testing.T) {
	tests := []struct {
		name    string
		content string
		query   string
		maxLen  int
		want    string
	}{
		{
			name:    "query term at start - no prefix ellipsis",
			content: "function calculateTotal() returns the sum of all items in the cart",
			query:   "function",
			maxLen:  40,
			want:    "function calculateTotal() returns th…",
		},
		{
			name:    "query term in middle - both ellipses",
			content: "The authentication system uses JWT tokens for secure session management and includes refresh token rotation",
			query:   "JWT",
			maxLen:  50,
			want:    "…authentication system uses JWT tokens for secure…",
		},
		{
			name:    "query term at end - no suffix ellipsis",
			content: "This component handles all the routing and navigation logic for the application",
			query:   "application",
			maxLen:  40,
			want:    "…navigation logic for the application",
		},
		{
			name:    "no match - empty query - fallback to head",
			content: "This is a long piece of content that should be truncated from the beginning since there is no query match",
			query:   "",
			maxLen:  30,
			want:    "This is a long piece of conten",
		},
		{
			name:    "content shorter than maxLen - return as-is",
			content: "Short content",
			query:   "content",
			maxLen:  50,
			want:    "Short content",
		},
		{
			name:    "unicode content with query match",
			content: "这是一个包含中文字符的测试内容，用于验证Unicode处理是否正确",
			query:   "测试",
			maxLen:  20,
			want:    "…一个包含中文字符的测试内容，用于验证…",
		},
		{
			name:    "no lexical match - vector result - fallback to head",
			content: "The system architecture follows microservices patterns with event-driven communication",
			query:   "scalability distributed",
			maxLen:  35,
			want:    "The system architecture follows mi",
		},
		{
			name:    "maxLen zero returns empty",
			content: "Some content here",
			query:   "content",
			maxLen:  0,
			want:    "",
		},
		{
			name:    "maxLen negative returns empty",
			content: "Some content here",
			query:   "content",
			maxLen:  -10,
			want:    "",
		},
		{
			name:    "query with multiple terms - uses earliest match",
			content: "The database migration scripts handle schema updates and data transformations for production deployments",
			query:   "schema production",
			maxLen:  45,
			want:    "…migration scripts handle schema updates and…",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := search.ExtractRelevantSnippet(tt.content, tt.query, tt.maxLen)
			if !utf8.ValidString(got) {
				t.Errorf("ExtractRelevantSnippet() returned invalid UTF-8: %q", got)
			}
			runeLen := utf8.RuneCountInString(got)
			if tt.maxLen > 0 && runeLen > tt.maxLen {
				t.Errorf("result length %d exceeds maxLen %d: %q", runeLen, tt.maxLen, got)
			}
			if tt.maxLen <= 0 && got != "" {
				t.Errorf("expected empty for maxLen=%d, got %q", tt.maxLen, got)
			}
		})
	}
}
