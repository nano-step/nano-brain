package graph

import "regexp"

type EdgeKind string

const (
	EdgeContains    EdgeKind = "contains"
	EdgeImports     EdgeKind = "imports"
	EdgeCalls       EdgeKind = "calls"
	EdgeHTTP        EdgeKind = "http"        // "<METHOD> <path>" -> handler symbol name
	EdgeMiddleware  EdgeKind = "middleware"  // middleware symbol -> handler symbol name
	EdgeIntegration EdgeKind = "integration" // outbound HTTP calls, queue publishes, event emissions
	EdgeReconcile   EdgeKind = "reconcile"   // transparent pass-through for flow builder BFS
)

const unresolvedCallTarget = "<unresolved>"

var canonicalJSCallTargetPattern = regexp.MustCompile(`^(?:(?:[A-Za-z0-9_$-]+/)*[A-Za-z0-9_$-]+(?:\.[A-Za-z0-9_$-]+)*\.(?:js|jsx|mjs|cjs|ts|tsx|mts|cts))::(?:[A-Za-z_$][A-Za-z0-9_$]*|[A-Za-z_$][A-Za-z0-9_$]*\.[A-Za-z_$][A-Za-z0-9_$]*|default|default\.[A-Za-z_$][A-Za-z0-9_$]*|default)$`)

// IsCanonicalJSCallTarget reports whether target is a source-scoped JS/TS
// calls identity. Legacy targets, including Ruby namespaces, stay outside this
// predicate so graph readers can preserve their existing compatibility rules.
func IsCanonicalJSCallTarget(target string) bool {
	return canonicalJSCallTargetPattern.MatchString(target)
}

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
