package graph

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
)

type Registry struct {
	extractors       []Extractor
	activeExtractors atomic.Pointer[[]Extractor]
	cfgExtractors    []ControlFlowExtractor
}

func NewRegistry(extractors ...Extractor) *Registry {
	return &Registry{extractors: extractors}
}

// RegisterControlFlowExtractor adds a control-flow (CFG) extractor. CFG
// extractors run independently of edge extractors and are invoked via
// ExtractCFGs.
func (r *Registry) RegisterControlFlowExtractor(ce ControlFlowExtractor) {
	r.cfgExtractors = append(r.cfgExtractors, ce)
}

// HasControlFlowExtractors reports whether any CFG extractor is registered.
func (r *Registry) HasControlFlowExtractors() bool {
	return len(r.cfgExtractors) > 0
}

// ExtractCFGs runs every registered control-flow extractor that supports the
// file's extension and concatenates their CFGs. Minified/generated files are
// skipped (same guard as ExtractEdges).
func (r *Registry) ExtractCFGs(filePath string, content []byte) ([]CFG, error) {
	if isMinified(filePath, content) {
		return nil, nil
	}
	ext := filepath.Ext(filePath)
	var all []CFG
	for _, ce := range r.cfgExtractors {
		if !ce.SupportsCFG(ext) {
			continue
		}
		cfgs, err := ce.ExtractCFGs(filePath, content)
		if err != nil {
			return nil, fmt.Errorf("extract cfgs %s: %w", filePath, err)
		}
		all = append(all, cfgs...)
	}
	return all, nil
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
//
// NOTE: ExtractEdges uses the global activeExtractors set, which can be
// incorrect when collections have different frameworks (last-write-wins bug).
// Prefer ExtractEdgesForFrameworks for per-collection correctness.
func (r *Registry) ExtractEdges(filePath string, content []byte) ([]Edge, error) {
	if isMinified(filePath, content) {
		return nil, nil
	}
	ext := filepath.Ext(filePath)
	var all []Edge
	seen := make(map[string]struct{})

	extractors := r.extractors
	if ae := r.activeExtractors.Load(); ae != nil && len(*ae) > 0 {
		extractors = *ae
	}

	return r.extractWith(filePath, content, ext, extractors, seen, all)
}

// ExtractEdgesForFrameworks runs extractors filtered by the given collection's
// detected frameworks instead of the global activeExtractors set. This avoids
// the last-write-wins bug where SetActiveFrameworks called for one collection
// filters out extractors needed by another.
func (r *Registry) ExtractEdgesForFrameworks(filePath string, content []byte, frameworks []string) ([]Edge, error) {
	if isMinified(filePath, content) {
		return nil, nil
	}
	ext := filepath.Ext(filePath)
	var all []Edge
	seen := make(map[string]struct{})

	var extractors []Extractor
	for _, ex := range r.extractors {
		if fa, ok := ex.(FrameworkAwareExtractor); ok {
			if req := fa.RequiresFrameworks(); len(req) > 0 && !hasIntersection(req, frameworks) {
				continue
			}
		}
		extractors = append(extractors, ex)
	}
	if len(extractors) == 0 {
		extractors = r.extractors
	}

	return r.extractWith(filePath, content, ext, extractors, seen, all)
}

// ExtractEdgesForFrameworksWithImports is ExtractEdgesForFrameworks plus
// import-specifier resolution (Fix B, #501): extractors implementing
// ImportResolvingExtractor (the JS/TS/Vue graph extractors) are called via
// ExtractEdgesWithImportContext(filePath, content, ic) instead of the plain
// Extractor.ExtractEdges, so imports edges get resolved workspace-relative
// target_node values. Every other extractor (the ~17 that don't implement
// ImportResolvingExtractor) is invoked exactly as before — the shared
// Extractor interface is untouched.
func (r *Registry) ExtractEdgesForFrameworksWithImports(filePath string, content []byte, frameworks []string, ic ImportContext) ([]Edge, error) {
	if isMinified(filePath, content) {
		return nil, nil
	}
	ext := filepath.Ext(filePath)
	var all []Edge
	seen := make(map[string]struct{})

	var extractors []Extractor
	for _, ex := range r.extractors {
		if fa, ok := ex.(FrameworkAwareExtractor); ok {
			if req := fa.RequiresFrameworks(); len(req) > 0 && !hasIntersection(req, frameworks) {
				continue
			}
		}
		extractors = append(extractors, ex)
	}
	if len(extractors) == 0 {
		extractors = r.extractors
	}

	for _, ex := range extractors {
		if !ex.Supports(ext) {
			continue
		}
		var edges []Edge
		var err error
		if iae, ok := ex.(ImportResolvingExtractor); ok {
			edges, err = iae.ExtractEdgesWithImportContext(filePath, content, ic)
		} else {
			edges, err = ex.ExtractEdges(filePath, content)
		}
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

// isMinified reports whether a file looks like minified/generated output that
// would only add noise to the graph (webpack/nuxt bundles, *.min.js, etc.).
// Minified files pack everything onto a few extremely long lines, so a very
// long maximum line length is a reliable signal.
func isMinified(filePath string, content []byte) bool {
	lower := strings.ToLower(filePath)
	for _, suf := range []string{".min.js", ".min.mjs", ".min.cjs", ".min.css", ".bundle.js", ".chunk.js", ".chunk.mjs", ".umd.js"} {
		if strings.HasSuffix(lower, suf) {
			return true
		}
	}
	if len(content) == 0 {
		return false
	}
	maxLine, cur, newlines := 0, 0, 0
	for _, b := range content {
		if b == '\n' {
			if cur > maxLine {
				maxLine = cur
			}
			cur = 0
			newlines++
			continue
		}
		cur++
	}
	if cur > maxLine {
		maxLine = cur
	}
	if maxLine > 2000 {
		return true
	}
	// Large file with very few line breaks (avg line length very high).
	if len(content) > 50000 && len(content)/(newlines+1) > 400 {
		return true
	}
	return false
}

func (r *Registry) extractWith(filePath string, content []byte, ext string, extractors []Extractor, seen map[string]struct{}, all []Edge) ([]Edge, error) {
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
