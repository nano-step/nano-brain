package summarize

import (
	"strings"
	"unicode"
)

const (
	maxSlugLen      = 80
	defaultSlugStub = "untitled-session"
)

// slugify converts an arbitrary session title into a filesystem-safe identifier
// suitable for use as part of a filename. Rules (issue #258):
//   1. Lowercase
//   2. Each rune that is not [a-z0-9] becomes '-'
//   3. Consecutive '-' collapse to single '-'
//   4. Leading/trailing '-' trimmed
//   5. Truncated to 80 chars (at any boundary, no word-aware logic)
//   6. Empty result → "untitled-session"
//
// Unicode handling: non-ASCII letters (e.g. Vietnamese diacritics) are treated
// as non-alphanumeric and replaced with '-'. This is deliberate — filesystem
// portability across macOS, Linux, and Obsidian-on-mobile matters more than
// preserving original glyphs. The session title itself remains intact in the
// markdown body's frontmatter-equivalent header.
func Slugify(title string) string {
	var b strings.Builder
	b.Grow(len(title))

	for _, r := range strings.ToLower(title) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if unicode.IsSpace(r) || r == '-' || r == '_' || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			b.WriteByte('-')
		} else {
			b.WriteByte('-')
		}
	}

	out := collapseDashes(b.String())
	out = strings.Trim(out, "-")

	if len(out) > maxSlugLen {
		out = strings.TrimRight(out[:maxSlugLen], "-")
	}

	if out == "" {
		return defaultSlugStub
	}
	return out
}

// collapseDashes replaces runs of '-' with a single '-'.
func collapseDashes(s string) string {
	if !strings.Contains(s, "--") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	prevDash := false
	for _, r := range s {
		if r == '-' {
			if prevDash {
				continue
			}
			prevDash = true
		} else {
			prevDash = false
		}
		b.WriteRune(r)
	}
	return b.String()
}
