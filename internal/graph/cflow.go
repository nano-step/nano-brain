package graph

import (
	"fmt"
	"path/filepath"
)

type CFGNode struct {
	ID    string `json:"id"`
	Type  string `json:"type"`            // "start" | "step" | "decision" | "terminal" | "merge"
	Label string `json:"label"`
	Line  int    `json:"line"`
	Kind  string `json:"kind,omitempty"`  // "error" | "return" (terminals only)
	Call  string `json:"call,omitempty"`  // target symbol (steps that are calls)
}

// CFGEdge represents a directed edge in a control-flow graph.
type CFGEdge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Branch string `json:"branch"` // "yes" | "no" | "case:<v>" | "default" | "loop" | "next"
}

// CFG is a control-flow graph for one function.
type CFG struct {
	Entry      string    `json:"entry,omitempty"`
	SourceFile string    `json:"source_file"`
	StartLine  int       `json:"start_line"`
	EndLine    int       `json:"end_line"`
	Nodes      []CFGNode `json:"nodes"`
	Edges      []CFGEdge `json:"edges"`
	Status     string    `json:"status"` // "complete" | "truncated" | "parse_error" | "unsupported"
}

// ControlFlowExtractor extracts control-flow graphs from source files.
type ControlFlowExtractor interface {
	SupportsCFG(ext string) bool
	ExtractCFGs(filePath string, content []byte) ([]CFG, error)
}

// CFGRegistry holds a set of ControlFlowExtractor instances.
type CFGRegistry struct {
	extractors []ControlFlowExtractor
}

// NewCFGRegistry creates a CFG registry with the given extractors.
func NewCFGRegistry(extractors ...ControlFlowExtractor) *CFGRegistry {
	return &CFGRegistry{extractors: extractors}
}

// ExtractCFGs runs every registered CFG extractor that supports the file
// extension and returns all extracted CFGs.
func (r *CFGRegistry) ExtractCFGs(filePath string, content []byte) ([]CFG, error) {
	if isMinified(filePath, content) {
		return nil, nil
	}
	ext := filepath.Ext(filePath)
	var all []CFG
	for _, ex := range r.extractors {
		if !ex.SupportsCFG(ext) {
			continue
		}
		cfgs, err := ex.ExtractCFGs(filePath, content)
		if err != nil {
			return nil, fmt.Errorf("extract cfg %s: %w", filePath, err)
		}
		all = append(all, cfgs...)
	}
	if all == nil {
		all = []CFG{}
	}
	return all, nil
}

// HasControlFlowExtractors returns true if at least one CFG extractor is registered.
func (r *CFGRegistry) HasControlFlowExtractors() bool {
	return len(r.extractors) > 0
}
