package chunk

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
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

func maxChunkLen(chunks []Chunk) int {
	maxLen := 0
	for _, c := range chunks {
		if len(c.Content) > maxLen {
			maxLen = len(c.Content)
		}
	}
	return maxLen
}

func TestSplit_HardSplit_SingleLongLine(t *testing.T) {
	input := strings.Repeat("a", 10000)
	chunks := Split(input, DefaultConfig())
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks for 10k-char single line, got %d", len(chunks))
	}
	allowed := DefaultConfig().TargetSize + searchWindow/2
	if got := maxChunkLen(chunks); got > allowed {
		t.Errorf("chunk exceeds allowed size: got %d, max allowed %d", got, allowed)
	}
}

func TestSplit_HardSplit_FenceTrapped(t *testing.T) {
	input := "```go\n" + strings.Repeat("x", 8000)
	chunks := Split(input, DefaultConfig())
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks for fence-trapped 8k content, got %d", len(chunks))
	}
	allowed := DefaultConfig().TargetSize + searchWindow/2
	if got := maxChunkLen(chunks); got > allowed {
		t.Errorf("fence-trapped chunk exceeds allowed: got %d, max %d", got, allowed)
	}
}

func TestSplit_HardSplit_UTF8_CJK(t *testing.T) {
	input := strings.Repeat("漢", 2500)
	chunks := Split(input, DefaultConfig())
	allowed := DefaultConfig().TargetSize + searchWindow/2
	for i, c := range chunks {
		if !utf8.ValidString(c.Content) {
			t.Errorf("chunk %d not valid UTF-8", i)
		}
		if len(c.Content) > allowed {
			t.Errorf("chunk %d exceeds allowed: %d > %d", i, len(c.Content), allowed)
		}
	}
	var rebuilt strings.Builder
	for _, c := range chunks {
		rebuilt.WriteString(c.Content)
	}
	if rebuilt.String() != input {
		t.Errorf("CJK round-trip failed: got %d bytes, want %d", rebuilt.Len(), len(input))
	}
}

func TestSplit_HardSplit_UTF8_Emoji(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 60; i++ {
		b.WriteString(strings.Repeat("a", 100))
		b.WriteString("🚀")
	}
	input := b.String()
	chunks := Split(input, DefaultConfig())
	allowed := DefaultConfig().TargetSize + searchWindow/2
	for i, c := range chunks {
		if !utf8.ValidString(c.Content) {
			t.Errorf("chunk %d not valid UTF-8 (likely cut mid-emoji)", i)
		}
		if len(c.Content) > allowed {
			t.Errorf("chunk %d exceeds allowed: %d > %d", i, len(c.Content), allowed)
		}
	}
}

func TestSplit_HardSplit_Pathological(t *testing.T) {
	input := strings.Repeat("x", 1_000_000)
	chunks := Split(input, DefaultConfig())
	allowed := DefaultConfig().TargetSize + searchWindow/2
	if len(chunks) < 200 {
		t.Errorf("expected many chunks for 1MB input, got %d", len(chunks))
	}
	if got := maxChunkLen(chunks); got > allowed {
		t.Errorf("pathological chunk exceeds allowed: %d > %d", got, allowed)
	}
}

func TestSplit_HardSplit_PrefersSentenceBoundary(t *testing.T) {
	sentence := strings.Repeat("a", 80) + ". "
	input := strings.Repeat(sentence, 60)
	chunks := Split(input, DefaultConfig())
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for i := 0; i < len(chunks)-1; i++ {
		c := chunks[i].Content
		if len(c) < 2 {
			continue
		}
		tail := c[len(c)-2:]
		if tail != ". " && !strings.HasSuffix(c, "\n") && !strings.HasSuffix(c, " ") {
			t.Errorf("chunk %d does not end at sentence/whitespace boundary: tail=%q", i, tail)
		}
	}
}

func TestSplit_HardSplit_NormalContentUnchanged(t *testing.T) {
	input := strings.Repeat("# Section\n\nNormal paragraph of about 80 chars per line.\n\n", 150)
	chunks := Split(input, DefaultConfig())
	allowed := DefaultConfig().TargetSize + searchWindow/2
	if got := maxChunkLen(chunks); got > allowed {
		t.Errorf("normal content exceeds allowed: %d > %d", got, allowed)
	}
	if len(chunks) < 2 {
		t.Errorf("expected multiple chunks for normal long content, got %d", len(chunks))
	}
}

func TestFindHardBoundary_PrefersBlankLine(t *testing.T) {
	s := strings.Repeat("a", 2700) + "\n\n" + strings.Repeat("b", 1000)
	cut := findHardBoundary(s, 3600)
	if cut != 2702 {
		t.Errorf("expected cut after blank-line marker at 2702, got %d", cut)
	}
}

func TestFindHardBoundary_FallsBackToNewline(t *testing.T) {
	s := strings.Repeat("a", 3000) + "\n" + strings.Repeat("b", 1000)
	cut := findHardBoundary(s, 3600)
	if cut != 3001 {
		t.Errorf("expected cut after newline at 3001, got %d", cut)
	}
}

func TestFindHardBoundary_FallsBackToSentence(t *testing.T) {
	s := strings.Repeat("a", 3400) + ". " + strings.Repeat("b", 1000)
	cut := findHardBoundary(s, 3600)
	if cut != 3402 {
		t.Errorf("expected cut after sentence at 3402, got %d", cut)
	}
}

func TestFindHardBoundary_FallsBackToWhitespace(t *testing.T) {
	s := strings.Repeat("a", 3400) + " " + strings.Repeat("b", 1000)
	cut := findHardBoundary(s, 3600)
	if cut != 3401 {
		t.Errorf("expected cut after whitespace at 3401, got %d", cut)
	}
}

func TestFindHardBoundary_RuneBoundaryFallback(t *testing.T) {
	s := strings.Repeat("漢", 2000)
	cut := findHardBoundary(s, 3600)
	if cut <= 0 || cut > 3600 {
		t.Errorf("cut out of range: %d", cut)
	}
	if cut < len(s) && !utf8.RuneStart(s[cut]) {
		t.Errorf("cut at %d is not a UTF-8 rune start (byte %x)", cut, s[cut])
	}
}

func TestSplit_DefaultConfig_MatchesEmbedBudget(t *testing.T) {
	cfg := DefaultConfig()
	const defaultMaxEmbedChars = 3000
	got := cfg.TargetSize + searchWindow/2
	if got != defaultMaxEmbedChars {
		t.Errorf("contract violation: DefaultConfig max output is %d, but embed queue's defaultMaxEmbedChars is %d. These MUST match — see issue #300.", got, defaultMaxEmbedChars)
	}
}

func TestSplit_TraceJSON_NoOversize(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"events":[`)
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&b, `{"ts":%d,"data":"%s"},`, i, strings.Repeat("x", 200))
	}
	b.WriteString(`]}`)
	input := b.String()
	chunks := Split(input, DefaultConfig())
	const embedMaxChars = 3000
	for i, c := range chunks {
		if len(c.Content) > embedMaxChars {
			t.Errorf("chunk %d would trigger embed-queue truncation: len=%d > %d", i, len(c.Content), embedMaxChars)
		}
	}
	if len(chunks) < 2 {
		t.Errorf("expected multiple chunks for JSON of %d chars, got %d", len(input), len(chunks))
	}
}
