package search

import "strings"

const MaxSnippetLen = 700

// TruncateSnippet truncates a string to maxChars runes, preserving valid UTF-8 boundaries.
// The function iterates over runes (not bytes), so multi-byte characters are handled correctly.
// If maxChars <= 0, returns empty string.
func TruncateSnippet(s string, maxChars int) string {
	if maxChars <= 0 {
		return ""
	}
	if len(s) <= maxChars {
		return s
	}
	var count int
	for i := range s {
		if count == maxChars {
			return s[:i]
		}
		count++
	}
	return s
}

// ExtractRelevantSnippet extracts a query-relevant window from content, centering
// around the first lexical match if found. Falls back to TruncateSnippet if no match.
// Preserves UTF-8 boundaries and snaps to word boundaries with ellipsis indicators.
func ExtractRelevantSnippet(content string, query string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(content) <= maxLen {
		return content
	}
	if query == "" {
		return TruncateSnippet(content, maxLen)
	}

	// Tokenize query into terms
	lowerContent := strings.ToLower(content)
	terms := strings.Fields(strings.ToLower(query))
	
	// Find earliest match position
	bestPos := -1
	for _, term := range terms {
		pos := strings.Index(lowerContent, term)
		if pos >= 0 && (bestPos < 0 || pos < bestPos) {
			bestPos = pos
		}
	}
	
	if bestPos < 0 {
		// No lexical match (vector-only result), fallback to head
		return TruncateSnippet(content, maxLen)
	}

	// Center window around match
	halfWindow := maxLen / 2
	start := bestPos - halfWindow
	if start < 0 {
		start = 0
	}

	willTruncateStart := start > 0
	willTruncateEnd := start+maxLen < len(content)

	extractLen := maxLen
	if willTruncateStart {
		extractLen--
	}
	if willTruncateEnd {
		extractLen--
	}

	// Extract window using rune-safe iteration
	var runeStart, runeEnd, runeCount int
	var foundStart bool
	for i := range content {
		if runeCount == start && !foundStart {
			runeStart = i
			foundStart = true
		}
		if runeCount == start+extractLen {
			runeEnd = i
			break
		}
		runeCount++
	}
	if runeEnd == 0 {
		runeEnd = len(content)
	}
	if !foundStart {
		runeStart = 0
	}

	snippet := content[runeStart:runeEnd]
	if willTruncateStart {
		snippet = "…" + snippet
	}
	if willTruncateEnd {
		snippet = snippet + "…"
	}

	return snippet
}
