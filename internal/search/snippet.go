package search

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
