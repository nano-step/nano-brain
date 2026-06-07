package watcher

import (
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/nano-brain/nano-brain/internal/chunker"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"golang.org/x/net/context"
)

func (w *Watcher) extractAndInsertEntities(ctx context.Context, workspace string, chunks []chunker.Chunk, chunkIDs []uuid.UUID) {
	if len(chunks) != len(chunkIDs) {
		return
	}

	for i, ch := range chunks {
		if ch.SymbolName == "" {
			continue
		}

		entities := extractChunkEntities(ch.SymbolName, ch.SymbolKind, ch.Content)
		for _, e := range entities {
			_ = w.queries.InsertChunkEntity(ctx, sqlc.InsertChunkEntityParams{
				ChunkID:       chunkIDs[i],
				EntityName:    e.name,
				EntityType:    e.entityType,
				WorkspaceHash: workspace,
			})
		}
	}
}

type chunkEntity struct {
	name       string
	entityType string
}

func extractChunkEntities(symbolName string, symbolKind string, content string) []chunkEntity {
	seen := make(map[string]bool)
	var entities []chunkEntity

	lower := strings.ToLower(symbolName)
	seen[lower] = true
	entities = append(entities, chunkEntity{name: lower, entityType: symbolKind})

	for _, part := range splitCamelCaseWatcher(symbolName) {
		pl := strings.ToLower(part)
		if len(pl) >= 3 && !seen[pl] {
			seen[pl] = true
			entities = append(entities, chunkEntity{name: pl, entityType: symbolKind})
		}
	}

	for _, token := range tokenizeContent(content) {
		tl := strings.ToLower(token)
		if len(tl) < 3 || seen[tl] || isStopword(tl) {
			continue
		}
		seen[tl] = true
		typ := "function"
		if len(token) > 0 && unicode.IsUpper(rune(token[0])) {
			typ = "type"
		}
		entities = append(entities, chunkEntity{name: tl, entityType: typ})
	}

	return entities
}

func tokenizeContent(content string) []string {
	var tokens []string
	var current strings.Builder

	for _, r := range content {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			current.WriteRune(r)
		} else {
			if current.Len() >= 3 && hasLetter(current.String()) {
				tokens = append(tokens, current.String())
			}
			current.Reset()
		}
	}
	if current.Len() >= 3 && hasLetter(current.String()) {
		tokens = append(tokens, current.String())
	}
	return tokens
}

func hasLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func splitCamelCaseWatcher(s string) []string {
	var parts []string
	var current strings.Builder

	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) && current.Len() > 0 {
			parts = append(parts, current.String())
			current.Reset()
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

func isStopword(s string) bool {
	switch s {
	case "the", "how", "does", "work", "what", "when", "is", "are",
		"a", "an", "in", "to", "for", "of", "and", "or",
		"func", "return", "var", "const", "type", "import", "package",
		"if", "else", "switch", "case", "default", "break", "continue",
		"string", "int", "bool", "error", "nil", "true", "false":
		return true
	}
	return false
}
