package links

import (
	"regexp"
	"strings"
)

// Kind distinguishes between ID-based and title-based wikilinks.
type Kind int

const (
	KindTitle Kind = iota
	KindID
)

// Link is one [[wikilink]] occurrence in source content.
type Link struct {
	Raw       string
	TargetRef string
	Kind      Kind
	Start     int
	End       int
}

var uuidRe = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

const maxContentLen = 200

// Parse scans content for [[wikilinks]] in a single pass. A backslash
// immediately before [[ escapes the wikilink (no link emitted, backslash
// consumed). Content inside brackets must be single-line and at most 200
// characters; otherwise the match is skipped. Duplicate TargetRefs are
// preserved; the caller is responsible for deduplication.
func Parse(content string) []Link {
	var out []Link
	i := 0
	n := len(content)
	for i < n-3 {
		if content[i] == '[' && content[i+1] == '[' {
			if i > 0 && content[i-1] == '\\' {
				i += 2
				continue
			}
			start := i
			j := i + 2
			for j < n-1 {
				if content[j] == '[' && content[j+1] == '[' {
					break
				}
				if content[j] == ']' && content[j+1] == ']' {
					inner := content[start+2 : j]
					if len(inner) > 0 && len(inner) <= maxContentLen && !strings.Contains(inner, "\n") {
						ref := strings.TrimSpace(inner)
						if ref != "" {
							kind := KindTitle
							if uuidRe.MatchString(ref) {
								kind = KindID
								ref = strings.ToLower(ref)
							}
							out = append(out, Link{
								Raw:       content[start : j+2],
								TargetRef: ref,
								Kind:      kind,
								Start:     start,
								End:       j + 2,
							})
						}
					}
					i = j + 2
					goto next
				}
				if content[j] == '\n' {
					break
				}
				j++
			}
			i = j
			continue
		}
		i++
		continue
	next:
	}
	return out
}
