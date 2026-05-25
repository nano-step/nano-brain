package symbol

import "path/filepath"

type Kind string

const (
	KindFunction  Kind = "function"
	KindMethod    Kind = "method"
	KindType      Kind = "type"
	KindInterface Kind = "interface"
	KindStruct    Kind = "struct"
	KindConst     Kind = "const"
	KindVar       Kind = "var"
)

type Symbol struct {
	Name      string
	Kind      Kind
	File      string
	Line      int
	Signature string
	Language  string
}

type Extractor interface {
	Extract(filePath string, content []byte) ([]Symbol, error)
	Supports(ext string) bool
}

type Registry struct {
	extractors []Extractor
}

func NewRegistry(extractors ...Extractor) *Registry {
	return &Registry{extractors: extractors}
}

func (r *Registry) Supports(ext string) bool {
	for _, e := range r.extractors {
		if e.Supports(ext) {
			return true
		}
	}
	return false
}

func (r *Registry) Extract(filePath string, content []byte) ([]Symbol, error) {
	ext := filepath.Ext(filePath)
	for _, e := range r.extractors {
		if e.Supports(ext) {
			return e.Extract(filePath, content)
		}
	}
	return nil, nil
}
