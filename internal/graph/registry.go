package graph

import (
	"fmt"
	"path/filepath"
)

type Registry struct {
	extractors []Extractor
}

func NewRegistry(extractors ...Extractor) *Registry {
	return &Registry{extractors: extractors}
}

// ExtractEdges runs every extractor that supports the file's extension and
// concatenates their edges, de-duplicating identical edges. Multiple extractors
// may claim the same extension (e.g. the Go call-graph extractor and the Echo
// route extractor both handle ".go"); each contributes its own edge kinds.
func (r *Registry) ExtractEdges(filePath string, content []byte) ([]Edge, error) {
	ext := filepath.Ext(filePath)
	var all []Edge
	seen := make(map[string]struct{})
	for _, ex := range r.extractors {
		if !ex.Supports(ext) {
			continue
		}
		edges, err := ex.ExtractEdges(filePath, content)
		if err != nil {
			return nil, fmt.Errorf("extract edges %s: %w", filePath, err)
		}
		for _, e := range edges {
			key := string(e.Kind) + "\x00" + e.SourceNode + "\x00" + e.TargetNode
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			all = append(all, e)
		}
	}
	return all, nil
}
