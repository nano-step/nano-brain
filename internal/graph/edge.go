package graph

type EdgeKind string

const (
	EdgeContains EdgeKind = "contains"
	EdgeImports  EdgeKind = "imports"
	EdgeCalls    EdgeKind = "calls"
)

type Edge struct {
	SourceNode string
	TargetNode string
	Kind       EdgeKind
	SourceFile string
	Line       int
	Language   string
}

type Extractor interface {
	ExtractEdges(filePath string, content []byte) ([]Edge, error)
	Supports(ext string) bool
}
