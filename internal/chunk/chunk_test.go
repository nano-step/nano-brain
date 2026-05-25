package chunk

import (
	"strings"
	"testing"
)

func TestChunkEmptyInput(t *testing.T) {
	chunks := Split("", DefaultConfig())
	if len(chunks) != 0 {
		t.Fatalf("expected 0 chunks for empty input, got %d", len(chunks))
	}
}

func TestChunkOnlyWhitespace(t *testing.T) {
	chunks := Split("   \n\t\n  ", DefaultConfig())
	if len(chunks) != 0 {
		t.Fatalf("expected 0 chunks for whitespace-only input, got %d", len(chunks))
	}
}

func TestChunkSingleShortDoc(t *testing.T) {
	input := "# Hello\n\nShort paragraph.\n"
	chunks := Split(input, DefaultConfig())
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Content != input {
		t.Errorf("content mismatch:\ngot:  %q\nwant: %q", chunks[0].Content, input)
	}
	if chunks[0].Sequence != 0 {
		t.Errorf("expected Sequence=0, got %d", chunks[0].Sequence)
	}
	if chunks[0].StartLine != 1 {
		t.Errorf("expected StartLine=1, got %d", chunks[0].StartLine)
	}
	if chunks[0].EndLine != 3 {
		t.Errorf("expected EndLine=3, got %d", chunks[0].EndLine)
	}
	if chunks[0].Hash == "" {
		t.Error("expected non-empty hash")
	}
}

func TestChunkBasicSplit(t *testing.T) {
	cfg := DefaultConfig()
	// Build a ~10,000 char doc with headings every ~1000 chars.
	var b strings.Builder
	for i := 0; i < 10; i++ {
		b.WriteString("## Section " + string(rune('A'+i)) + "\n\n")
		b.WriteString(strings.Repeat("Lorem ipsum dolor sit amet. ", 35)) // ~980 chars
		b.WriteString("\n\n")
	}
	input := b.String()
	if len(input) < 9000 {
		t.Fatalf("test doc too short: %d chars", len(input))
	}

	chunks := Split(input, cfg)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if len(c.Content) < cfg.MinSize && i < len(chunks)-1 {
			t.Errorf("chunk %d is shorter than MinSize: %d chars", i, len(c.Content))
		}
		if c.Sequence != i {
			t.Errorf("chunk %d has Sequence=%d", i, c.Sequence)
		}
		if c.Hash == "" {
			t.Errorf("chunk %d has empty hash", i)
		}
	}
}

func TestChunkBreakPointPriority(t *testing.T) {
	cfg := Config{TargetSize: 100, Overlap: 0, MinSize: 10}
	// Place an H1 and a blank line near the target boundary.
	// The H1 should be preferred (score 100 > 50).
	input := strings.Repeat("x", 80) + "\n\n" + // blank line at ~81 chars
		"# Heading One\n" + // H1 at ~83 chars
		strings.Repeat("y", 200) + "\n"

	chunks := Split(input, cfg)
	if len(chunks) < 2 {
		t.Fatalf("expected >=2 chunks, got %d", len(chunks))
	}
	// Second chunk should start with the H1.
	if !strings.HasPrefix(chunks[1].Content, "# Heading One") {
		t.Errorf("expected chunk 1 to start with H1, got: %q", truncate(chunks[1].Content, 40))
	}
}

func TestChunkCodeFenceNoCut(t *testing.T) {
	cfg := Config{TargetSize: 100, Overlap: 0, MinSize: 10}
	// Code block spans the target boundary — break must land outside.
	input := "Some intro text\n\n" +
		"```go\n" +
		strings.Repeat("code line\n", 20) + // ~200 chars inside fence
		"```\n\n" +
		"After the fence.\n"

	chunks := Split(input, cfg)
	for i, c := range chunks {
		opens := strings.Count(c.Content, "```")
		if opens%2 != 0 {
			t.Errorf("chunk %d has unbalanced code fences (count=%d):\n%s",
				i, opens, truncate(c.Content, 200))
		}
	}
}

func TestChunkDeterminism(t *testing.T) {
	cfg := DefaultConfig()
	input := strings.Repeat("# Heading\n\nParagraph text goes here.\n\n", 50)
	a := Split(input, cfg)
	b := Split(input, cfg)
	if len(a) != len(b) {
		t.Fatalf("non-deterministic chunk count: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Content != b[i].Content {
			t.Errorf("chunk %d content differs between runs", i)
		}
		if a[i].Hash != b[i].Hash {
			t.Errorf("chunk %d hash differs between runs", i)
		}
	}
}

func TestChunkMinLength(t *testing.T) {
	cfg := DefaultConfig()
	input := strings.Repeat("word ", 2000) // ~10,000 chars, no headings
	chunks := Split(input, cfg)
	for i, c := range chunks {
		if len(c.Content) < cfg.MinSize {
			t.Errorf("chunk %d length %d < MinSize %d", i, len(c.Content), cfg.MinSize)
		}
	}
}

func TestChunkPositionTracking(t *testing.T) {
	cfg := Config{TargetSize: 50, Overlap: 0, MinSize: 10}
	input := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\n"
	chunks := Split(input, cfg)
	if len(chunks) < 2 {
		t.Fatalf("expected >=2 chunks, got %d", len(chunks))
	}
	if chunks[0].StartLine != 1 {
		t.Errorf("first chunk StartLine=%d, want 1", chunks[0].StartLine)
	}
	last := chunks[len(chunks)-1]
	if last.EndLine != 10 {
		t.Errorf("last chunk EndLine=%d, want 10", last.EndLine)
	}
	for i, c := range chunks {
		if c.StartLine < 1 || c.EndLine < c.StartLine {
			t.Errorf("chunk %d has invalid lines: Start=%d End=%d", i, c.StartLine, c.EndLine)
		}
	}
}

func TestChunkOverlap(t *testing.T) {
	cfg := Config{TargetSize: 100, Overlap: 20, MinSize: 10}
	var b strings.Builder
	for i := 0; i < 10; i++ {
		b.WriteString("## Section\n")
		b.WriteString(strings.Repeat("a", 80) + "\n\n")
	}
	chunks := Split(b.String(), cfg)
	if len(chunks) < 3 {
		t.Fatalf("expected >=3 chunks, got %d", len(chunks))
	}
	// Check that consecutive chunks share some content.
	shared := 0
	for i := 1; i < len(chunks); i++ {
		prev := chunks[i-1].Content
		curr := chunks[i].Content
		// The beginning of curr should overlap with the end of prev.
		overlapLen := findOverlap(prev, curr)
		if overlapLen > 0 {
			shared++
		}
	}
	if shared == 0 {
		t.Error("no overlap detected between any consecutive chunks")
	}
}

func TestChunkNoHeadings(t *testing.T) {
	cfg := DefaultConfig()
	var b strings.Builder
	for i := 0; i < 300; i++ {
		b.WriteString("Plain text without any markdown headings.\n")
	}
	input := b.String()
	chunks := Split(input, cfg)
	if len(chunks) < 2 {
		t.Fatalf("expected >=2 chunks for %d char input, got %d", len(input), len(chunks))
	}
	for i, c := range chunks {
		if len(c.Content) < cfg.MinSize {
			t.Errorf("chunk %d length %d < MinSize", i, len(c.Content))
		}
	}
}

func FuzzChunker(f *testing.F) {
	f.Add("# Hello\n\nSome text")
	f.Add("")
	f.Add(strings.Repeat("x", 10000))
	f.Add("```\ncode\n```\n")
	f.Add("---\n***\n___\n")
	f.Fuzz(func(t *testing.T, input string) {
		cfg := DefaultConfig()
		chunks := Split(input, cfg)
		nonBlank := strings.TrimSpace(input) != ""
		if nonBlank && len(chunks) == 0 {
			t.Errorf("non-whitespace input produced 0 chunks")
		}
		for i, c := range chunks {
			if c.Sequence != i {
				t.Errorf("chunk %d has Sequence=%d", i, c.Sequence)
			}
			if c.Hash == "" {
				t.Errorf("chunk %d has empty hash", i)
			}
			if c.StartLine < 1 {
				t.Errorf("chunk %d StartLine=%d < 1", i, c.StartLine)
			}
			if c.EndLine < c.StartLine {
				t.Errorf("chunk %d EndLine=%d < StartLine=%d", i, c.EndLine, c.StartLine)
			}
		}
	})
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func findOverlap(a, b string) int {
	maxCheck := len(a)
	if maxCheck > len(b) {
		maxCheck = len(b)
	}
	for size := maxCheck; size > 0; size-- {
		if a[len(a)-size:] == b[:size] {
			return size
		}
	}
	return 0
}
