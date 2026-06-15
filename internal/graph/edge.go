package graph

type EdgeKind string

const (
	EdgeContains   EdgeKind = "contains"
	EdgeImports    EdgeKind = "imports"
	EdgeCalls      EdgeKind = "calls"
	EdgeHTTP        EdgeKind = "http"        // "<METHOD> <path>" -> handler symbol name
	EdgeMiddleware  EdgeKind = "middleware"  // middleware symbol -> handler symbol name
	EdgeIntegration EdgeKind = "integration" // outbound HTTP calls, queue publishes, event emissions
)

type Edge struct {
	SourceNode string
	TargetNode string
	Kind       EdgeKind
	SourceFile string
	Line       int
	Language   string
	// Metadata carries extractor-supplied per-edge fields (e.g. {"method","path"}
	// for http edges). The watcher merges this with {line, language} on persist.
	// nil for extractors that supply none, preserving prior behavior.
	Metadata map[string]any
}

type Extractor interface {
	ExtractEdges(filePath string, content []byte) ([]Edge, error)
	Supports(ext string) bool
}

type FrameworkAwareExtractor interface {
	Extractor
	RequiresFrameworks() []string
}
