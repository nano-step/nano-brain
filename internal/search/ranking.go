package search

import (
	"path"
	"strings"
)

// ApplyCodeAwareBoost boosts results where query keywords match the source path
// or title. This surfaces code files higher when the query mentions function names,
// file paths, or module names.
//
// boostPathFactor is applied when query keywords appear in the source path.
// boostTitleFactor is applied when query keywords appear in the title.
// Both boosts are multiplicative on the existing score.
func ApplyCodeAwareBoost(results []Result, query string, boostPathFactor, boostTitleFactor float64) []Result {
	if len(results) == 0 || query == "" {
		return results
	}

	keywords := extractQueryKeywords(query)
	if len(keywords) == 0 {
		return results
	}

	for i := range results {
		boost := 1.0

		pathLower := strings.ToLower(results[i].SourcePath)
		baseName := strings.ToLower(path.Base(results[i].SourcePath))
		for _, kw := range keywords {
			if strings.Contains(pathLower, kw) || strings.Contains(baseName, kw) {
				boost *= boostPathFactor
				break
			}
		}

		titleLower := strings.ToLower(results[i].Title)
		for _, kw := range keywords {
			if strings.Contains(titleLower, kw) {
				boost *= boostTitleFactor
				break
			}
		}

		results[i].Score *= boost
	}

	return results
}

// extractQueryKeywords extracts meaningful keywords from a search query.
// It removes common stop words and short tokens, then lowercases.
func extractQueryKeywords(query string) []string {
	stopWords := map[string]bool{
		"a": true, "an": true, "the": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "shall": true, "can": true, "need": true,
		"dare": true, "ought": true, "used": true, "to": true, "of": true,
		"in": true, "for": true, "on": true, "with": true, "at": true,
		"by": true, "from": true, "as": true, "into": true, "through": true,
		"during": true, "before": true, "after": true, "above": true, "below": true,
		"between": true, "out": true, "off": true, "over": true, "under": true,
		"again": true, "further": true, "then": true, "once": true, "here": true,
		"there": true, "when": true, "where": true, "why": true, "how": true,
		"all": true, "both": true, "each": true, "few": true, "more": true,
		"most": true, "other": true, "some": true, "such": true, "no": true,
		"nor": true, "not": true, "only": true, "own": true, "same": true,
		"so": true, "than": true, "too": true, "very": true, "just": true,
		"don": true, "now": true, "and": true, "but": true, "or": true,
		"if": true, "while": true, "about": true, "up": true, "it": true,
		"its": true, "what": true, "which": true, "who": true, "whom": true,
		"this": true, "that": true, "these": true, "those": true, "i": true,
		"me": true, "my": true, "we": true, "our": true, "you": true,
		"your": true, "he": true, "him": true, "his": true, "she": true,
		"her": true, "they": true, "them": true, "their": true,
	}

	words := strings.Fields(strings.ToLower(query))
	keywords := make([]string, 0, len(words))
	seen := make(map[string]bool)

	for _, w := range words {
		// Strip punctuation
		w = strings.Trim(w, ".,;:!?\"'()[]{}<>|/\\@#$%^&*~`")
		if len(w) < 2 || stopWords[w] || seen[w] {
			continue
		}
		seen[w] = true
		keywords = append(keywords, w)
	}

	return keywords
}
