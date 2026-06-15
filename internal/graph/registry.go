package graph

import (
	"fmt"
	"path/filepath"
	"sync/atomic"
)

type Registry struct {
	extractors       []Extractor
	activeExtractors atomic.Pointer[[]Extractor]
}

func NewRegistry(extractors ...Extractor) *Registry {
	return &Registry{extractors: extractors}
}

// SetActiveFrameworks filters the extractor list to only those whose
// RequiredFrameworks intersect with the given set. Extractors that don't
// implement FrameworkAwareExtractor (or return empty RequiresFrameworks)
// always run. If filtering would eliminate all extractors, the full set
// is kept (fail-open). Thread-safe via atomic.Pointer.
func (r *Registry) SetActiveFrameworks(frameworks []string) {
	var filtered []Extractor
	for _, ex := range r.extractors {
		if fa, ok := ex.(FrameworkAwareExtractor); ok {
			if req := fa.RequiresFrameworks(); len(req) > 0 && !hasIntersection(req, frameworks) {
				continue
			}
		}
		filtered = append(filtered, ex)
	}
	if len(filtered) == 0 {
		filtered = r.extractors
	}
	r.activeExtractors.Store(&filtered)
}

func hasIntersection(a, b []string) bool {
	for _, va := range a {
		for _, vb := range b {
			if va == vb {
				return true
			}
		}
	}
	return false
}

// ExtractEdges runs every active extractor that supports the file's extension
// and concatenates their edges, de-duplicating identical edges. Multiple
// extractors may claim the same extension (e.g. the Go call-graph extractor
// and the Echo route extractor both handle ".go"); each contributes its own
// edge kinds.
func (r *Registry) ExtractEdges(filePath string, content []byte) ([]Edge, error) {
	ext := filepath.Ext(filePath)
	var all []Edge
	seen := make(map[string]struct{})

	extractors := r.extractors
	if ae := r.activeExtractors.Load(); ae != nil && len(*ae) > 0 {
		extractors = *ae
	}

	for _, ex := range extractors {
		if !ex.Supports(ext) {
			continue
		}
		edges, err := ex.ExtractEdges(filePath, content)
		if err != nil {
			return nil, fmt.Errorf("extract edges %s: %w", filePath, err)
		}
		for _, e := range edges {
			key := fmt.Sprintf("%s:%s:%s:%d", e.Kind, e.SourceNode, e.TargetNode, e.Line)
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			all = append(all, e)
		}
	}
	return all, nil
}
