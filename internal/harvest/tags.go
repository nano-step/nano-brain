package harvest

import (
	"regexp"
	"strings"
)

// Pre-compiled regex patterns for semantic tag inference
var (
	// Bug fix patterns
	bugFixPattern = regexp.MustCompile(`(?i)\b(?:fix|bug|patch|resolve|hotfix|regression|broken|crash)\b`)

	// Feature patterns
	featurePattern = regexp.MustCompile(`(?i)\b(?:feat|feature|implement|add|create|new|introduce)\b`)

	// Refactor patterns
	refactorPattern = regexp.MustCompile(`(?i)\b(?:refactor|restructure|reorganize|clean.?up|simplif\w*)\b`)

	// Documentation patterns
	docsPattern = regexp.MustCompile(`(?i)\b(?:docs?|documentation|readme|comment)\b`)

	// Chore patterns
	chorePattern = regexp.MustCompile(`(?i)\b(?:chore|bump|upgrade|dependency|ci|cd|config|infra)\b`)
)

// InferSemanticTags analyzes content and title to extract semantic tags.
// Returns a slice of inferred tags (may be empty).
func InferSemanticTags(content, title string) []string {
	combined := strings.ToLower(title + " " + content)
	tags := make([]string, 0)

	if bugFixPattern.MatchString(combined) {
		tags = append(tags, "bug-fix")
	}

	if featurePattern.MatchString(combined) {
		tags = append(tags, "feature")
	}

	if refactorPattern.MatchString(combined) {
		tags = append(tags, "refactor")
	}

	if docsPattern.MatchString(combined) {
		tags = append(tags, "docs")
	}

	if chorePattern.MatchString(combined) {
		tags = append(tags, "chore")
	}

	return tags
}
