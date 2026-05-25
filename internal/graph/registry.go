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

func (r *Registry) ExtractEdges(filePath string, content []byte) ([]Edge, error) {
	ext := filepath.Ext(filePath)
	for _, ex := range r.extractors {
		if !ex.Supports(ext) {
			continue
		}
		edges, err := ex.ExtractEdges(filePath, content)
		if err != nil {
			return nil, fmt.Errorf("extract edges %s: %w", filePath, err)
		}
		return edges, nil
	}
	return nil, nil
}
