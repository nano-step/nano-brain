package search

import (
	"strings"
	"unicode"
)

type Entity struct {
	Name string
	Type string
}

var stopwords = map[string]bool{
	"the": true, "how": true, "does": true, "work": true,
	"what": true, "when": true, "is": true, "are": true,
	"a": true, "an": true, "in": true, "to": true,
	"for": true, "of": true, "and": true, "or": true,
}

func ExtractEntities(symbolName string, symbolKind string, chunkContent string) []Entity {
	seen := make(map[string]bool)
	var entities []Entity

	if symbolName != "" {
		lower := strings.ToLower(symbolName)
		if !seen[lower] {
			seen[lower] = true
			entities = append(entities, Entity{Name: lower, Type: symbolKind})
		}
	}

	for _, token := range tokenizeIdentifiers(chunkContent) {
		lower := strings.ToLower(token)
		if stopwords[lower] || len(lower) < 2 || seen[lower] {
			continue
		}
		seen[lower] = true
		entities = append(entities, Entity{Name: lower, Type: classifyToken(token)})
	}

	return entities
}

func ExtractQueryEntities(query string) []string {
	var entities []string
	seen := make(map[string]bool)

	for _, token := range strings.Fields(query) {
		cleaned := strings.TrimFunc(token, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
		})
		if cleaned == "" {
			continue
		}
		lower := strings.ToLower(cleaned)
		if stopwords[lower] || len(lower) < 2 || seen[lower] {
			continue
		}
		seen[lower] = true
		entities = append(entities, lower)
	}

	return entities
}

func tokenizeIdentifiers(content string) []string {
	var tokens []string
	var current strings.Builder

	for _, r := range content {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			current.WriteRune(r)
		} else {
			if current.Len() >= 2 {
				tok := current.String()
				if isIdentifierLike(tok) {
					tokens = append(tokens, tok)
					for _, part := range splitCamelCase(tok) {
						if len(part) >= 2 {
							tokens = append(tokens, part)
						}
					}
				}
			}
			current.Reset()
		}
	}
	if current.Len() >= 2 {
		tok := current.String()
		if isIdentifierLike(tok) {
			tokens = append(tokens, tok)
			for _, part := range splitCamelCase(tok) {
				if len(part) >= 2 {
					tokens = append(tokens, part)
				}
			}
		}
	}

	return tokens
}

func isIdentifierLike(s string) bool {
	if len(s) < 2 {
		return false
	}
	hasLetter := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			hasLetter = true
			break
		}
	}
	return hasLetter
}

func splitCamelCase(s string) []string {
	var parts []string
	var current strings.Builder

	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	if len(parts) <= 1 {
		return nil
	}
	return parts
}

func classifyToken(token string) string {
	if len(token) == 0 {
		return "identifier"
	}
	if unicode.IsUpper(rune(token[0])) {
		return "type"
	}
	return "function"
}
