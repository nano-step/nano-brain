// Package chunk handles document chunking and segmentation.
package chunk

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// Chunk represents a segment of a document.
type Chunk struct {
	Content   string // text content of the chunk
	Sequence  int    // 0-indexed position in document
	StartLine int    // 1-indexed start line in original document
	EndLine   int    // 1-indexed end line in original document
	Hash      string // SHA-256 hex of chunk content
}

// Config controls the chunking behaviour.
type Config struct {
	TargetSize int // target chunk size in chars (default 3600)
	Overlap    int // overlap between consecutive chunks in chars (default 200)
	MinSize    int // minimum chunk size; shorter trailing chunks are merged (default 200)
}

// DefaultConfig returns the standard chunking configuration.
func DefaultConfig() Config {
	return Config{TargetSize: 3600, Overlap: 200, MinSize: 200}
}

// lineInfo holds metadata about a single line in the source document.
type lineInfo struct {
	text        string // line text including trailing newline (if any)
	startOffset int    // char offset of line start in original content
	endOffset   int    // char offset one past line end
	lineNum     int    // 1-indexed line number
	breakScore  int    // score for breaking BEFORE this line
	fenceOpen   bool   // true if a code fence is open AFTER this line
}

const (
	scoreH1   = 100
	scoreH2   = 90
	scoreH3   = 80
	scoreH4H6 = 70
	scoreHR   = 60
	scoreBlank = 50
	scoreList  = 40
	scoreNL    = 10
)

const searchWindow = 800

// Split splits content into chunks according to cfg.
func Split(content string, cfg Config) []Chunk {
	if isBlank(content) {
		return nil
	}

	lines := parseLines(content)
	if len(lines) == 0 {
		return nil
	}

	splits := findSplitPoints(lines, cfg)
	chunks := buildChunks(content, lines, splits, cfg)

	if len(chunks) > 1 && len(chunks[len(chunks)-1].Content) < cfg.MinSize {
		last := len(chunks) - 1
		chunks[last-1].Content += chunks[last].Content
		chunks[last-1].EndLine = chunks[last].EndLine
		chunks[last-1].Hash = hashContent(chunks[last-1].Content)
		chunks = chunks[:last]
	}

	for i := range chunks {
		chunks[i].Sequence = i
		chunks[i].Hash = hashContent(chunks[i].Content)
	}

	return chunks
}

// parseLines splits content into lineInfo records.
func parseLines(content string) []lineInfo {
	raw := strings.SplitAfter(content, "\n")
	if len(raw) > 0 && raw[len(raw)-1] == "" {
		raw = raw[:len(raw)-1]
	}

	lines := make([]lineInfo, len(raw))
	offset := 0
	fenceOpen := false

	for i, text := range raw {
		li := lineInfo{
			text:        text,
			startOffset: offset,
			endOffset:   offset + len(text),
			lineNum:     i + 1,
			breakScore:  scoreLine(text),
		}

		if isCodeFence(text) {
			fenceOpen = !fenceOpen
		}
		li.fenceOpen = fenceOpen

		lines[i] = li
		offset += len(text)
	}

	return lines
}

// scoreLine returns the break-point score for breaking BEFORE a line
// with the given text content.
func scoreLine(text string) int {
	trimmed := strings.TrimLeft(text, " \t")

	switch {
	case strings.HasPrefix(trimmed, "# ") || trimmed == "#\n" || trimmed == "#":
		return scoreH1
	case strings.HasPrefix(trimmed, "## ") || trimmed == "##\n" || trimmed == "##":
		return scoreH2
	case strings.HasPrefix(trimmed, "### ") || trimmed == "###\n" || trimmed == "###":
		return scoreH3
	case isCodeFence(text):
		return scoreH3
	case isHeading4Plus(trimmed):
		return scoreH4H6
	case isHorizontalRule(trimmed):
		return scoreHR
	case isBlankLine(text):
		return scoreBlank
	case isListItem(trimmed):
		return scoreList
	default:
		return scoreNL
	}
}

// isCodeFence returns true if the line is a code fence delimiter (``` ...).
func isCodeFence(text string) bool {
	trimmed := strings.TrimLeft(text, " \t")
	return strings.HasPrefix(trimmed, "```")
}

// isHeading4Plus matches ####, #####, ###### headings.
func isHeading4Plus(trimmed string) bool {
	if !strings.HasPrefix(trimmed, "####") {
		return false
	}
	rest := strings.TrimLeft(trimmed, "#")
	return len(rest) == 0 || rest[0] == ' ' || rest[0] == '\t' || rest[0] == '\n'
}

// isHorizontalRule matches ---, ***, ___ (with optional spaces).
func isHorizontalRule(trimmed string) bool {
	clean := strings.TrimRight(trimmed, " \t\n\r")
	if len(clean) < 3 {
		return false
	}
	ch := clean[0]
	if ch != '-' && ch != '*' && ch != '_' {
		return false
	}
	for _, c := range clean {
		if c != rune(ch) && c != ' ' {
			return false
		}
	}
	return true
}

// isBlankLine returns true if the line contains only whitespace.
func isBlankLine(text string) bool {
	return strings.TrimSpace(text) == ""
}

// isListItem matches unordered (- , * ) and ordered (1. ) list items.
func isListItem(trimmed string) bool {
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
		return true
	}
	for i, c := range trimmed {
		if c >= '0' && c <= '9' {
			continue
		}
		if c == '.' && i > 0 && i+1 < len(trimmed) && trimmed[i+1] == ' ' {
			return true
		}
		break
	}
	return false
}

// isBlank returns true if the entire string is empty or whitespace-only.
func isBlank(s string) bool {
	return strings.TrimSpace(s) == ""
}

// findSplitPoints returns the line indices where splits should occur.
// A split at index S means: the chunk boundary is BEFORE line S.
func findSplitPoints(lines []lineInfo, cfg Config) []int {
	if len(lines) == 0 {
		return nil
	}

	totalLen := lines[len(lines)-1].endOffset
	var splits []int
	chunkStartIdx := 0

	for {
		chunkStartOffset := lines[chunkStartIdx].startOffset
		remainingLen := totalLen - chunkStartOffset
		if remainingLen <= cfg.TargetSize {
			break
		}

		targetOffset := chunkStartOffset + cfg.TargetSize
		windowStart := targetOffset - searchWindow/2
		windowEnd := targetOffset + searchWindow/2

		bestLine := -1
		bestScore := -1

		for i := chunkStartIdx + 1; i < len(lines); i++ {
			off := lines[i].startOffset
			if off < windowStart {
				continue
			}
			if off > windowEnd {
				break
			}
			if i > 0 && lines[i-1].fenceOpen {
				continue
			}
			if lines[i].breakScore > bestScore {
				bestScore = lines[i].breakScore
				bestLine = i
			}
		}

		if bestLine < 0 {
			bestLine = findBreakAfterFence(lines, chunkStartIdx+1)
		}

		if bestLine < 0 || bestLine <= chunkStartIdx {
			break
		}

		splits = append(splits, bestLine)
		chunkStartIdx = bestLine
	}

	return splits
}

// findBreakAfterFence finds the first valid break point at or after lineIdx
// that is outside a code fence.
func findBreakAfterFence(lines []lineInfo, startIdx int) int {
	for i := startIdx; i < len(lines); i++ {
		if i == 0 {
			continue
		}
		if !lines[i-1].fenceOpen {
			return i
		}
	}
	return -1
}

// buildChunks constructs Chunk values from split points with overlap.
func buildChunks(content string, lines []lineInfo, splits []int, cfg Config) []Chunk {
	if len(lines) == 0 {
		return nil
	}

	type span struct{ start, end int }
	var spans []span

	prevStart := 0
	for i, splitLine := range splits {
		spans = append(spans, span{start: prevStart, end: splitLine - 1})

		nextStart := splitLine
		if cfg.Overlap > 0 && splitLine < len(lines) {
			overlapTarget := lines[splitLine].startOffset - cfg.Overlap
			if overlapTarget < 0 {
				overlapTarget = 0
			}
			for j := splitLine; j >= 0; j-- {
				if lines[j].startOffset <= overlapTarget {
					nextStart = j
					break
				}
			}
			if i > 0 && nextStart < splits[i-1] {
				nextStart = splits[i-1]
			} else if i == 0 && nextStart < 0 {
				nextStart = 0
			}
		}
		prevStart = nextStart
	}
	spans = append(spans, span{start: prevStart, end: len(lines) - 1})

	chunks := make([]Chunk, len(spans))
	for i, sp := range spans {
		startOff := lines[sp.start].startOffset
		endOff := lines[sp.end].endOffset
		chunks[i] = Chunk{
			Content:   content[startOff:endOff],
			Sequence:  i,
			StartLine: lines[sp.start].lineNum,
			EndLine:   lines[sp.end].lineNum,
		}
	}

	return chunks
}

// hashContent returns the SHA-256 hex digest of s.
func hashContent(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}
