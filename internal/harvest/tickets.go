package harvest

import (
	"regexp"
	"sort"
	"strings"
)

// defaultTicketPatterns are the built-in ticket ID patterns used when no
// patterns are configured. The first pattern matches Linear/JIRA-style IDs
// (e.g. DEV-1234, PROJ-42) with word boundaries so it does not match inside
// larger tokens. The second matches GitHub issue references (#42). Content is
// pre-screened to strip markdown headings before applying #\d+ so lines like
// "# Introduction" do not produce false positives.
//
// The JIRA pattern uses \b boundaries (Go RE2 supports ASCII word boundaries)
// so technical strings like "UTF-8" or "SHA-256" embedded in prose are not
// matched as ticket IDs. A denylist (nonTicketPrefixes) provides a second line
// of defense against well-known non-ticket prefixes that share the shape.
var defaultTicketPatterns = []string{
	`\b[A-Z][A-Z0-9]+-\d+\b`,
	`#\d+`,
}

// nonTicketPrefixes is a denylist of project-key prefixes that match the
// JIRA-style ticket shape (PREFIX-NUMBER) but are well-known non-ticket
// technical identifiers (encodings, hashes, RFCs, protocols, CVEs). Matches
// whose prefix (the part before the first '-') is in this set are discarded.
// Keys are uppercase; lookups uppercase the candidate first.
var nonTicketPrefixes = map[string]struct{}{
	"UTF":    {},
	"UTF8":   {},
	"UTF16":  {},
	"SHA":    {},
	"MD5":    {},
	"ISO":    {},
	"RFC":    {},
	"TLS":    {},
	"SSL":    {},
	"HTTP":   {},
	"HTTPS":  {},
	"CVE":    {},
	"BASE64": {},
	"IPV4":   {},
	"IPV6":   {},
	"X86":    {},
	"ARM64":  {},
}

// isNonTicket reports whether a JIRA-shaped match (e.g. "UTF-8", "DEV-4706")
// should be discarded because its prefix is a known non-ticket identifier.
// The GitHub "#NN" form has no prefix and is never filtered here.
func isNonTicket(match string) bool {
	prefix, _, found := strings.Cut(match, "-")
	if !found {
		return false
	}
	_, denied := nonTicketPrefixes[strings.ToUpper(prefix)]
	return denied
}

// TicketExtractor extracts ticket IDs from content, branch names, and parent
// session tags. Patterns are compiled at construction time for efficiency.
type TicketExtractor struct {
	patterns []*regexp.Regexp
}

// NewTicketExtractor compiles the supplied regex patterns and returns a
// TicketExtractor. If patterns is nil or empty the default patterns
// ([A-Z]+-\d+ and #\d+) are used. Returns an error if any pattern fails to
// compile.
func NewTicketExtractor(patterns []string) (*TicketExtractor, error) {
	if len(patterns) == 0 {
		patterns = defaultTicketPatterns
	}
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, err
		}
		compiled = append(compiled, re)
	}
	return &TicketExtractor{patterns: compiled}, nil
}

// Extract derives ticket IDs from three sources and returns them as a sorted,
// deduplicated slice of bare IDs (e.g. ["DEV-4706", "PROJ-42"]).
//
//   - content: full session markdown; markdown headings (lines starting with #)
//     are excluded before applying the #\d+ pattern to avoid false positives.
//   - branch: git branch name (e.g. "feat/DEV-4706-my-feature").
//   - parentTags: tags from the parent session document; any tag with the
//     prefix "ticket:" is stripped and inherited by the child.
func (e *TicketExtractor) Extract(content, branch string, parentTags []string) []string {
	seen := make(map[string]struct{})

	// addMatches scans src with each pattern and records matches, discarding
	// known non-ticket identifiers (UTF-8, SHA-256, etc.) via the denylist.
	addMatches := func(src string) {
		for _, re := range e.patterns {
			for _, match := range re.FindAllString(src, -1) {
				if isNonTicket(match) {
					continue
				}
				seen[strings.ToUpper(match)] = struct{}{}
			}
		}
	}

	// Scan content with markdown headings stripped so "# My Heading" does not
	// match the #\d+ pattern as a spurious ticket.
	addMatches(stripMarkdownHeadings(content))

	// Scan branch name directly — headings don't appear in branch names.
	addMatches(branch)

	// Inherit tickets from parent tags.
	for _, tag := range parentTags {
		if id, ok := strings.CutPrefix(tag, "ticket:"); ok {
			seen[strings.ToUpper(id)] = struct{}{}
		}
	}

	if len(seen) == 0 {
		return nil
	}

	result := make([]string, 0, len(seen))
	for id := range seen {
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}

// AsTags converts bare ticket IDs into "ticket:<ID>" tag strings.
func (e *TicketExtractor) AsTags(tickets []string) []string {
	tags := make([]string, len(tickets))
	for i, t := range tickets {
		tags[i] = "ticket:" + t
	}
	return tags
}

// stripMarkdownHeadings removes lines that start with one or more '#' followed
// by a space (ATX-style headings) from the text. This prevents the #\d+
// pattern from matching heading anchors like "# Introduction".
func stripMarkdownHeadings(text string) string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, "#")
		if len(trimmed) < len(line) && (len(trimmed) == 0 || trimmed[0] == ' ') {
			// This line is a markdown heading — skip it.
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
